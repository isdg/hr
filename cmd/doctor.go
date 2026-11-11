package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/doctor"
	"github.com/isdg/hr/internal/rc"
)

var doctorJSON bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the vault for filesystem/config inconsistencies",
	Long: `Checks hr.toml and every feed directory for problems hr's normal
commands don't otherwise surface: malformed sidecars, orphaned files,
tombstone contradictions, id collisions, and config/directory drift.

Read-only: doctor never modifies the vault. Exits non-zero if any
error-level issue was found.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, cfg, err := openActiveVault()
		if err != nil {
			return err
		}
		report, err := doctor.Check(v, cfg)
		if err != nil {
			return err
		}
		issues, err := rcIssues()
		if err != nil {
			return err
		}
		report.Issues = append(report.Issues, issues...)
		if doctorJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(report.Issues); err != nil {
				return err
			}
		} else {
			printDoctorReport(report)
		}
		if report.HasErrors() {
			os.Exit(1)
		}
		return nil
	},
}

// rcIssues reports what in ~/.hrrc hr ignores, and an older hr's
// config.toml that could not move there because ~/.hrrc exists.
func rcIssues() ([]doctor.Issue, error) {
	r, err := rc.Load()
	if err != nil {
		return nil, err
	}
	var issues []doctor.Issue
	for _, k := range r.Unknown {
		msg := fmt.Sprintf("unknown key %q is ignored", k)
		if k == "vault" {
			msg = "vault is no longer read: set $HR_VAULT, " +
				"or cd into the vault"
		}
		issues = append(issues, doctor.Issue{
			Severity: doctor.Warn, Path: "~/.hrrc", Message: msg,
		})
	}
	legacy, err := rc.LegacyPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(legacy); err == nil {
		issues = append(issues, doctor.Issue{
			Severity: doctor.Warn, Path: legacy,
			Message: "no longer read: settings live in ~/.hrrc",
		})
	}
	return issues, nil
}

func printDoctorReport(r doctor.Report) {
	if len(r.Issues) == 0 {
		fmt.Println("vault looks healthy")
		return
	}
	for _, i := range r.Issues {
		path := i.Path
		if path == "" {
			path = "-"
		}
		fmt.Printf("%-5s %-40s %s\n", i.Severity, path, i.Message)
	}
	fmt.Printf("\n%d error(s), %d warning(s), %d info\n",
		r.Count(doctor.Error), r.Count(doctor.Warn), r.Count(doctor.Info))
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "JSON output")
	rootCmd.AddCommand(doctorCmd)
}

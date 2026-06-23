package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/corrupt"
)

var (
	corruptRange   string
	corruptNote    string
	corruptContext int
	corruptID      string
	corruptAll     bool
	corruptJSON    bool
	corruptForce   bool
)

var corruptCmd = &cobra.Command{
	Use:   "corrupt <article.md> --range L1:C1-L2:C2 [--note ...]",
	Short: "Mark a region of an article as corrupted",
	Long: `Mark a region of an article as corrupted so it can be restored later.

Positions are 1-based lines and 0-based byte columns; the end column is
exclusive. The binary reads the article and stores the exact selected
text (plus surrounding context) in the .meta.toml sidecar, so an LLM or
other tooling can later read every corrupted region and repair it.

  hr corrupt feeds/aristotle/-0350-...md --range 12:0-14:37 --note "OCR garble"

See 'hr corrupt list --all --json' for the unified report, and
'hr corrupt rm <article.md> --id <id>' to clear a mark after repair.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := parseRange(corruptRange)
		if err != nil {
			return err
		}
		c, err := corrupt.Mark(args[0], r, corruptNote, corruptContext)
		if err != nil {
			return err
		}
		fmt.Printf("marked %s (lines %d-%d)\n", c.ID, c.StartLine, c.EndLine)
		return nil
	},
}

var corruptListCmd = &cobra.Command{
	Use:   "list [article.md]",
	Short: "List corruption marks for an article or the whole vault",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		recs, err := gatherCorruptions(args)
		if err != nil {
			return err
		}
		if corruptJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(recs)
		}
		return printCorruptions(recs)
	},
}

var corruptRmCmd = &cobra.Command{
	Use:   "rm <article.md> --id <id>",
	Short: "Remove a corruption mark by id",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if corruptID == "" {
			return fmt.Errorf("--id is required")
		}
		removed, err := corrupt.Remove(args[0], corruptID)
		if err != nil {
			return err
		}
		if !removed {
			return fmt.Errorf("no corruption with id %q", corruptID)
		}
		fmt.Printf("removed %s\n", corruptID)
		return nil
	},
}

var corruptRestoreCmd = &cobra.Command{
	Use:   "restore <article.md> --id <id>",
	Short: "Replace a marked region with text from stdin, then clear the mark",
	Long: `Replace the region of a corruption mark with replacement text read
from stdin, then remove the mark.

By default it refuses if the article text at the recorded range no
longer matches the stored quote (drift); pass --force to override.

  echo "the corrected passage" | hr corrupt restore <article.md> --id a1b2c3d4`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if corruptID == "" {
			return fmt.Errorf("--id is required")
		}
		repl, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return err
		}
		err = corrupt.Restore(args[0], corruptID, string(repl), corruptForce)
		if errors.Is(err, corrupt.ErrDrift) {
			return fmt.Errorf("%w; re-check the mark or pass --force", err)
		}
		if err != nil {
			return err
		}
		fmt.Printf("restored %s\n", corruptID)
		return nil
	},
}

func gatherCorruptions(args []string) ([]corrupt.Record, error) {
	if len(args) == 1 && !corruptAll {
		rec, err := corrupt.List(args[0])
		if err != nil {
			return nil, err
		}
		return []corrupt.Record{rec}, nil
	}
	v, _, err := openActiveVault()
	if err != nil {
		return nil, err
	}
	return corrupt.ListAll(v)
}

func printCorruptions(recs []corrupt.Record) error {
	total := 0
	for _, rec := range recs {
		for _, c := range rec.Corruptions {
			total++
			loc := fmt.Sprintf("%d:%d-%d:%d",
				c.StartLine, c.StartCol, c.EndLine, c.EndCol)
			flag := " "
			if c.Stale {
				flag = "!"
			}
			fmt.Printf("%s %s  %s  %s  %q",
				flag, rec.Path, c.ID, loc, oneLine(c.Quote))
			if c.Stale {
				fmt.Print("  [STALE]")
			}
			if c.Note != "" {
				fmt.Printf("  (%s)", c.Note)
			}
			fmt.Println()
		}
	}
	if total == 0 {
		fmt.Println("(no corruptions)")
	}
	return nil
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ⏎ ")
	if len(s) > 60 {
		s = s[:60] + "…"
	}
	return s
}

// parseRange parses "L1:C1-L2:C2" into a corrupt.Range.
func parseRange(s string) (corrupt.Range, error) {
	var r corrupt.Range
	lo, hi, ok := strings.Cut(s, "-")
	if !ok {
		return r, fmt.Errorf("range must be L1:C1-L2:C2, got %q", s)
	}
	var err error
	if r.StartLine, r.StartCol, err = parsePos(lo); err != nil {
		return r, err
	}
	if r.EndLine, r.EndCol, err = parsePos(hi); err != nil {
		return r, err
	}
	return r, nil
}

func parsePos(s string) (line, col int, err error) {
	l, c, ok := strings.Cut(s, ":")
	if !ok {
		return 0, 0, fmt.Errorf("position must be LINE:COL, got %q", s)
	}
	if line, err = strconv.Atoi(l); err != nil {
		return 0, 0, fmt.Errorf("bad line %q: %w", l, err)
	}
	if col, err = strconv.Atoi(c); err != nil {
		return 0, 0, fmt.Errorf("bad column %q: %w", c, err)
	}
	return line, col, nil
}

func init() {
	corruptCmd.Flags().StringVar(&corruptRange, "range", "",
		"selection as L1:C1-L2:C2 (1-based lines, 0-based cols, end exclusive)")
	corruptCmd.Flags().StringVar(&corruptNote, "note", "",
		"optional reason / description")
	corruptCmd.Flags().IntVar(&corruptContext, "context-lines",
		corrupt.DefaultContextLines,
		"lines of surrounding context to capture on each side")
	_ = corruptCmd.MarkFlagRequired("range")

	corruptListCmd.Flags().BoolVar(&corruptAll, "all", false,
		"report every article in the vault")
	corruptListCmd.Flags().BoolVar(&corruptJSON, "json", false,
		"JSON output (unified report for tooling/LLM)")

	corruptRmCmd.Flags().StringVar(&corruptID, "id", "",
		"id of the corruption mark to remove")

	corruptRestoreCmd.Flags().StringVar(&corruptID, "id", "",
		"id of the corruption mark to restore")
	corruptRestoreCmd.Flags().BoolVar(&corruptForce, "force", false,
		"restore even if the text has drifted from the stored quote")

	corruptCmd.AddCommand(corruptListCmd)
	corruptCmd.AddCommand(corruptRmCmd)
	corruptCmd.AddCommand(corruptRestoreCmd)
	rootCmd.AddCommand(corruptCmd)
}

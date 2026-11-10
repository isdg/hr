package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/history"
	"github.com/isdg/hr/internal/listing"
	"github.com/isdg/hr/internal/vault"
)

var recentOpts listFlags

var recentCmd = &cobra.Command{
	Use:   "recent",
	Short: "List articles you opened, most recent first",
	Long: `List the articles you opened, most recently opened first.

Opens are recorded by ` + "`hr opened`" + `, which hr.vim runs whenever a
vault article is loaded into a buffer. The filters match ` + "`hr list`" + `,
except that --since bounds when an article was opened, not published.
The log is ~/.hr/<vault>/history.jsonl, outside the vault: it belongs
to this machine and cannot be regenerated.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, _, err := openActiveVault()
		if err != nil {
			return err
		}
		f, err := recentOpts.filter()
		if err != nil {
			return err
		}
		hist, err := history.Load(v.HistoryPath())
		if err != nil {
			return err
		}
		items, err := listing.Recent(v, f, hist)
		if err != nil {
			return err
		}
		return recentOpts.print(items, openedTime)
	},
}

func openedTime(it listing.Item) string {
	return it.Opened.Local().Format("2006-01-02 15:04")
}

var openedCmd = &cobra.Command{
	Use:   "opened <path>...",
	Short: "Record that articles were opened (feeds hr recent)",
	Long: `Record that articles were opened, for hr recent.

Each article's vault is the one containing it, found by walking up for
hr.toml, so an editor can call this from any working directory.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		now := time.Now()
		for _, p := range args {
			if err := recordOpen(p, now); err != nil {
				return err
			}
		}
		return nil
	},
}

func recordOpen(p string, at time.Time) error {
	abs, err := filepath.Abs(p)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	root, ok := vault.Discover(filepath.Dir(abs))
	if !ok {
		return fmt.Errorf("%s: not in an hr vault", p)
	}
	v, err := vault.Open(root)
	if err != nil {
		return err
	}
	feeds := v.FeedsDir() + string(filepath.Separator)
	if !strings.HasPrefix(abs, feeds) {
		return fmt.Errorf("%s: not an article under %s", p, v.FeedsDir())
	}
	return history.Record(v.HistoryPath(), abs, at)
}

func init() {
	recentOpts.register(recentCmd,
		"only items opened within (e.g. 7d, 24h)")
	rootCmd.AddCommand(recentCmd)
	rootCmd.AddCommand(openedCmd)
}

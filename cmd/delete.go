package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/tombstone"
)

var (
	deleteRestore bool
	deletePurge   bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete <article.md>",
	Short: "Delete an article (sync-safe: tombstoned so it isn't re-fetched)",
	Long: `Delete an article.

By default this is sync-safe: the .md, its sidecar, and raw HTML are
removed and a small <id>.deleted marker is left in the feed directory so
sync skips that item forever (the marker is committed with the vault, so
the deletion propagates across machines).

  hr delete <article.md>            delete + tombstone (sync skips it)
  hr delete <article.md> --restore  lift the tombstone (sync re-fetches)
  hr delete <article.md> --purge    hard delete, no tombstone (may re-sync)

--restore also accepts the <id>.deleted marker path.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if deleteRestore && deletePurge {
			return fmt.Errorf("--restore and --purge are mutually exclusive")
		}
		v, _, err := openActiveVault()
		if err != nil {
			return err
		}
		path := args[0]
		switch {
		case deleteRestore:
			ok, err := tombstone.Restore(path)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("no tombstone found for %s", filepath.Base(path))
			}
			fmt.Printf("restored %s (sync will re-fetch it)\n", filepath.Base(path))
		case deletePurge:
			if err := tombstone.Purge(v, path); err != nil {
				return err
			}
			fmt.Printf("purged %s (no tombstone)\n", filepath.Base(path))
		default:
			mp, err := tombstone.Delete(v, path)
			if err != nil {
				return err
			}
			fmt.Printf("deleted %s (tombstone %s)\n",
				filepath.Base(path), filepath.Base(mp))
		}
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVar(&deleteRestore, "restore", false,
		"lift an existing tombstone instead of deleting")
	deleteCmd.Flags().BoolVar(&deletePurge, "purge", false,
		"hard delete with no tombstone (not sync-safe)")
	rootCmd.AddCommand(deleteCmd)
}

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/isg9/hr/internal/config"
	"github.com/isg9/hr/internal/syncer"
	"github.com/isg9/hr/internal/vault"
)

var (
	syncFeedFilter string
	syncForce      bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Fetch new items for all (or filtered) feeds",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := vault.Resolve(vaultFlag)
		if err != nil {
			return err
		}
		v, err := vault.Open(root)
		if err != nil {
			return err
		}
		cfg, err := config.Load(v.ConfigPath())
		if err != nil {
			return err
		}
		ua := cfg.UserAgent
		if ua == "" {
			ua = "hr/0.1"
		}

		res, err := syncer.Run(cmd.Context(), syncer.Options{
			Vault:       v,
			Config:      cfg,
			FeedName:    syncFeedFilter,
			UserAgent:   ua,
			Force:       syncForce,
			OnFeedStart: logFeedStart,
			OnFeedDone:  logFeedDone,
		})
		if res != nil {
			printSyncSummary(res)
		}
		return err
	},
}

// logFeedStart/logFeedDone print live sync progress to stderr, leaving
// stdout for the final machine-readable summary.
func logFeedStart(i, total int, name string) {
	fmt.Fprintf(os.Stderr, "[%d/%d] syncing %s…\n", i, total, name)
}

func logFeedDone(i, total int, fr syncer.FeedResult) {
	switch {
	case fr.Err != nil:
		fmt.Fprintf(os.Stderr, "[%d/%d] %s: error: %v\n",
			i, total, fr.Name, fr.Err)
	case fr.NotModified:
		fmt.Fprintf(os.Stderr, "[%d/%d] %s: not modified\n",
			i, total, fr.Name)
	default:
		fmt.Fprintf(os.Stderr, "[%d/%d] %s: %d new, %d existing\n",
			i, total, fr.Name, fr.New, fr.Existing)
	}
}

func printSyncSummary(r *syncer.Result) {
	for _, fr := range r.Feeds {
		switch {
		case fr.Err != nil:
			fmt.Printf("%s: error: %v\n", fr.Name, fr.Err)
		case fr.NotModified:
			fmt.Printf("%s: not modified\n", fr.Name)
		default:
			fmt.Printf("%s: %d new, %d existing\n",
				fr.Name, fr.New, fr.Existing)
		}
	}
}

func init() {
	syncCmd.Flags().StringVar(&syncFeedFilter, "feed", "",
		"sync only this feed name")
	syncCmd.Flags().BoolVar(&syncForce, "force", false,
		"ignore cache and refetch even if not modified")
	rootCmd.AddCommand(syncCmd)
}

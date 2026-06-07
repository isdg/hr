package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/isg/hrb/internal/config"
	"github.com/isg/hrb/internal/syncer"
	"github.com/isg/hrb/internal/vault"
)

var syncFeedFilter string

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
			ua = "hrb/0.1"
		}

		res, err := syncer.Run(cmd.Context(), syncer.Options{
			Vault:     v,
			Config:    cfg,
			FeedName:  syncFeedFilter,
			UserAgent: ua,
		})
		if res != nil {
			printSyncSummary(res)
		}
		return err
	},
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
	rootCmd.AddCommand(syncCmd)
}

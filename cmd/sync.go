package cmd

import "github.com/spf13/cobra"

var syncFeedFilter string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Fetch new items for all (or filtered) feeds",
	RunE:  stub("sync"),
}

func init() {
	syncCmd.Flags().StringVar(&syncFeedFilter, "feed", "",
		"sync only this feed name")
	rootCmd.AddCommand(syncCmd)
}

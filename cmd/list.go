package cmd

import "github.com/spf13/cobra"

var (
	listUnread bool
	listFeed   string
	listTag    string
	listSince  string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List articles, one per line",
	RunE:  stub("list"),
}

var feedCmd = &cobra.Command{
	Use:   "feed",
	Short: "Default reader feed: unread, newest first (for nvim consumption)",
	RunE:  stub("feed"),
}

func init() {
	listCmd.Flags().BoolVar(&listUnread, "unread", false, "only unread items")
	listCmd.Flags().StringVar(&listFeed, "feed", "", "filter to a single feed")
	listCmd.Flags().StringVar(&listTag, "tag", "", "filter by tag")
	listCmd.Flags().StringVar(&listSince, "since", "", "only items newer than (e.g. 7d, 24h)")
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(feedCmd)
}

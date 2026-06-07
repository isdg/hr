package cmd

import "github.com/spf13/cobra"

var readCmd = &cobra.Command{
	Use:   "read <path>...",
	Short: "Mark articles as read",
	Args:  cobra.MinimumNArgs(1),
	RunE:  stub("read"),
}

var unreadCmd = &cobra.Command{
	Use:   "unread <path>...",
	Short: "Mark articles as unread",
	Args:  cobra.MinimumNArgs(1),
	RunE:  stub("unread"),
}

func init() {
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(unreadCmd)
}

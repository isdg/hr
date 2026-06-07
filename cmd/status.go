package cmd

import "github.com/spf13/cobra"

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show vault summary: unread counts, last sync, feed health",
	RunE:  stub("status"),
}

func init() { rootCmd.AddCommand(statusCmd) }

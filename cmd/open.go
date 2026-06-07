package cmd

import "github.com/spf13/cobra"

var openCmd = &cobra.Command{
	Use:   "open <path>",
	Short: "Open an article's original URL in $BROWSER",
	Args:  cobra.ExactArgs(1),
	RunE:  stub("open"),
}

func init() { rootCmd.AddCommand(openCmd) }

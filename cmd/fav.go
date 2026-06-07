package cmd

import "github.com/spf13/cobra"

var favCmd = &cobra.Command{
	Use:   "fav <path>...",
	Short: "Toggle favorite on articles",
	Args:  cobra.MinimumNArgs(1),
	RunE:  stub("fav"),
}

func init() { rootCmd.AddCommand(favCmd) }

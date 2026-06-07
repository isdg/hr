package cmd

import "github.com/spf13/cobra"

var initCmd = &cobra.Command{
	Use:   "init [dir]",
	Short: "Initialize a new hrb vault",
	Args:  cobra.MaximumNArgs(1),
	RunE:  stub("init"),
}

func init() { rootCmd.AddCommand(initCmd) }

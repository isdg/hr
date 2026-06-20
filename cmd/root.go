package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var vaultFlag string

var rootCmd = &cobra.Command{
	Use:   "hr",
	Short: "File-first RSS / blog reader",
	Long: `hr syncs feeds into a directory of plain markdown files you can read
in any editor. Articles are immutable; per-article state (read, favorite,
tags) lives in sidecar .meta.toml files. The whole vault is git-syncable.

Running 'hr' with no subcommand prints this help.`,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "hr:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&vaultFlag, "vault", "C", "",
		"vault directory (default: $HR_VAULT or ~/blogs)")
}

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var vaultFlag string

var rootCmd = &cobra.Command{
	Use:   "hrb",
	Short: "File-first RSS / blog reader",
	Long: `hrb syncs feeds into a directory of plain markdown files you can read
in any editor. Articles are immutable; per-article state (read, favorite,
tags) lives in sidecar .meta.toml files. The whole vault is git-syncable.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "hrb:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&vaultFlag, "vault", "C", "",
		"vault directory (default: $HRB_VAULT or ~/blogs)")
}

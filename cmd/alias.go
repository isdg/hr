package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/meta"
)

var aliasCmd = &cobra.Command{
	Use:   "alias <path> [name]",
	Short: "Set or clear an article's local display alias",
	Long: "Local display label that overrides the article's title in " +
		"`hr list` and the nvim panel. The alias is stored in the " +
		"article's sidecar .meta.toml. With no [name] argument, " +
		"clears the existing alias.",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
		alias := ""
		if len(args) == 2 {
			alias = args[1]
		}
		return meta.SetAlias(abs, alias)
	},
}

func init() { rootCmd.AddCommand(aliasCmd) }

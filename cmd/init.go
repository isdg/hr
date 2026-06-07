package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/isg/hrb/internal/vault"
)

var initCmd = &cobra.Command{
	Use:   "init [dir]",
	Short: "Initialize a new hrb vault",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := vaultFlag
		if len(args) == 1 {
			target = args[0]
		}
		root, err := vault.Resolve(target)
		if err != nil {
			return err
		}
		v, err := vault.Init(root)
		if err != nil {
			return err
		}
		fmt.Printf("initialized hrb vault at %s\n", v.Root)
		fmt.Printf("  config: %s\n", v.ConfigPath())
		fmt.Printf("  feeds:  %s/\n", v.FeedsDir())
		fmt.Println()
		fmt.Println("Edit the config to add feeds, then run `hrb sync`.")
		return nil
	},
}

func init() { rootCmd.AddCommand(initCmd) }

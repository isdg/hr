package cmd

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/vault"
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

var initCmd = &cobra.Command{
	Use:   "init <name> [dir]",
	Short: "Initialize a new hr vault",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !validName.MatchString(name) {
			return fmt.Errorf(
				"name must match [a-zA-Z0-9][a-zA-Z0-9_-]*: %q",
				name)
		}

		explicit := vaultFlag
		if len(args) == 2 {
			explicit = args[1]
		}
		root, err := vault.ResolveNew(explicit, name)
		if err != nil {
			return err
		}
		v, err := vault.Init(root, name)
		if err != nil {
			return err
		}
		fmt.Printf(
			"initialized hr vault %q at %s\n", name, v.Root)
		fmt.Printf("  config: %s\n", v.ConfigPath())
		fmt.Printf("  feeds:  %s/\n", v.FeedsDir())
		fmt.Println()
		fmt.Println(
			"Edit the config to add feeds, then run `hr sync`.")
		return nil
	},
}

func init() { rootCmd.AddCommand(initCmd) }

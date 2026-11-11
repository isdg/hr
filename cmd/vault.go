package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Print the vault hr would use here",
	Long: `Prints the root of the vault the other commands would use from the
current directory: -C, else the vault holding the current directory,
else $HR_VAULT, else the vault line of .hrrc. Editors and scripts use
it instead of resolving the vault themselves.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, _, err := openActiveVault()
		if err != nil {
			return err
		}
		fmt.Println(v.Root)
		return nil
	},
}

func init() { rootCmd.AddCommand(vaultCmd) }

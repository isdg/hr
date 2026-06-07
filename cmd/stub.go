package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func stub(name string) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("%s: not implemented yet", name)
	}
}

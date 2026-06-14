package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/isg/hrb/internal/meta"
)

var favCmd = &cobra.Command{
	Use:   "fav <path>...",
	Short: "Toggle favorite on articles",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, p := range args {
			abs, err := filepath.Abs(p)
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			now, err := meta.ToggleFavorite(abs)
			if err != nil {
				return err
			}
			state := "unfavorited"
			if now {
				state = "favorited"
			}
			fmt.Printf("%s: %s\n", p, state)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(favCmd) }

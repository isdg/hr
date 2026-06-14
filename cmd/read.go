package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/isg/hrb/internal/meta"
)

var readCmd = &cobra.Command{
	Use:   "read <path>...",
	Short: "Mark articles as read",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return applyToArticles(args, meta.MarkRead)
	},
}

var unreadCmd = &cobra.Command{
	Use:   "unread <path>...",
	Short: "Mark articles as unread",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return applyToArticles(args, meta.MarkUnread)
	},
}

func applyToArticles(paths []string, fn func(string) error) error {
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := fn(abs); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(unreadCmd)
}

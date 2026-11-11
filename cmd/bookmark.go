package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/bookmark"
)

var bookmarkClear bool

var bookmarkCmd = &cobra.Command{
	Use:   "bookmark <path> [LINE[:COL]]",
	Short: "Set, show or clear an article's reading place",
	Long: `Set, show or clear where you stopped reading an article.

With a position, record it: a 1-based LINE and an optional 0-based byte
COL. With none, print the current one as LINE:COL, or nothing when the
article has no bookmark. --clear removes it.

Each article has one reading place; the log keeps every change, in
~/.hr/<vault>/bookmarks.jsonl, outside the vault: it belongs to this
machine and cannot be regenerated. The vault is the one containing the
article, so an editor can call this from any working directory.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBookmark(cmd.OutOrStdout(), args, bookmarkClear)
	},
}

func runBookmark(out io.Writer, args []string, clear bool) error {
	v, abs, err := articleVault(args[0])
	if err != nil {
		return err
	}
	log := v.BookmarksPath()
	switch {
	case clear && len(args) == 2:
		return fmt.Errorf("--clear takes no position")
	case clear:
		return bookmark.Clear(log, abs, time.Now())
	case len(args) == 2:
		p, err := bookmark.ParsePosition(args[1])
		if err != nil {
			return err
		}
		return bookmark.Set(log, abs, p, time.Now())
	}
	p, ok, err := bookmark.Get(log, abs)
	if err == nil && ok {
		_, err = fmt.Fprintln(out, p)
	}
	return err
}

func init() {
	bookmarkCmd.Flags().BoolVar(&bookmarkClear, "clear", false,
		"remove the article's bookmark")
	rootCmd.AddCommand(bookmarkCmd)
}

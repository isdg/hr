package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/bookmark"
	"github.com/isdg/hr/internal/history"
	"github.com/isdg/hr/internal/listing"
	"github.com/isdg/hr/internal/vault"
)

var (
	bookmarkClear bool
	bookmarkLast  bool
)

var bookmarkCmd = &cobra.Command{
	Use:   "bookmark {<path> [LINE[:COL]] | --last}",
	Short: "Set, show or clear an article's reading place",
	Long: `Set, show or clear where you stopped reading an article.

With a position, record it: a 1-based LINE and an optional 0-based byte
COL. With none, print the current one as LINE:COL, or nothing when the
article has no bookmark. --clear removes it.

--last, with no path, prints the most recently set bookmark in the vault
as PATH<TAB>LINE:COL, skipping articles no longer there, or nothing when
there is none: where to resume reading.

Each article has one reading place; the log keeps every change, in
~/.hr/<vault>/bookmarks.jsonl, outside the vault: it belongs to this
machine and cannot be regenerated. The vault is the one containing the
article, so an editor can call this from any working directory.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if bookmarkLast && len(args) > 0 {
			return fmt.Errorf("--last takes no path")
		}
		if bookmarkLast {
			return nil
		}
		return cobra.RangeArgs(1, 2)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if bookmarkLast {
			if bookmarkClear {
				return fmt.Errorf("--last and --clear do not combine")
			}
			v, _, err := openActiveVault()
			if err != nil {
				return err
			}
			return lastBookmark(cmd.OutOrStdout(), v)
		}
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

// lastBookmark prints the newest bookmark whose article the vault still
// lists, so a deleted article falls back to the one before it.
func lastBookmark(out io.Writer, v *vault.Vault) error {
	entries, err := bookmark.Load(v.BookmarksPath())
	if err != nil {
		return err
	}
	newest := bookmark.Newest(entries)
	if len(newest) == 0 {
		return nil
	}
	items, err := listing.List(v, listing.Filter{Bookmarked: true})
	if err != nil {
		return err
	}
	paths := make(map[history.Key]string, len(items))
	for _, it := range items {
		paths[history.KeyFor(it.Feed, it.Path)] = it.Path
	}
	for _, e := range newest {
		if p, ok := paths[e.Key()]; ok {
			_, err := fmt.Fprintf(out, "%s\t%s\n", p, e.Position)
			return err
		}
	}
	return nil
}

func init() {
	bookmarkCmd.Flags().BoolVar(&bookmarkClear, "clear", false,
		"remove the article's bookmark")
	bookmarkCmd.Flags().BoolVar(&bookmarkLast, "last", false,
		"print the most recent bookmark as PATH<TAB>LINE:COL")
	rootCmd.AddCommand(bookmarkCmd)
}

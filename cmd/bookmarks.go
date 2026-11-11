package cmd

import (
	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/bookmark"
	"github.com/isdg/hr/internal/listing"
)

var bookmarksOpts listFlags

var bookmarksCmd = &cobra.Command{
	Use:   "bookmarks",
	Short: "List bookmarked articles, most recently bookmarked first",
	Long: `List the articles with a reading place (see hr bookmark), most
recently bookmarked first, dated by when the bookmark was set. The
filters match ` + "`hr list`" + `, except that --since bounds that time,
not publication.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, _, err := openActiveVault()
		if err != nil {
			return err
		}
		f, err := bookmarksOpts.filter()
		if err != nil {
			return err
		}
		entries, err := bookmark.Load(v.BookmarksPath())
		if err != nil {
			return err
		}
		items, err := listing.Bookmarks(v, f, entries)
		if err != nil {
			return err
		}
		return bookmarksOpts.print(items, bookmarkedTime)
	},
}

func bookmarkedTime(it listing.Item) string {
	return it.BookmarkedAt.Local().Format("2006-01-02 15:04")
}

func init() {
	bookmarksOpts.register(bookmarksCmd,
		"only items bookmarked within (e.g. 7d, 24h)")
	rootCmd.AddCommand(bookmarksCmd)
}

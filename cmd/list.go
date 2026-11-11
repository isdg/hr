package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/listing"
)

// listFlags is the filter and output vocabulary shared by `hr list` and
// `hr recent`, so hr.vim and the shell pickers can forward either.
type listFlags struct {
	unread    bool
	favorite  bool
	feeds     []string
	groups    []string
	tag       string
	since     string
	corrupted bool
	tsv       bool
	json      bool
}

var (
	listOpts   listFlags
	feedGroups []string
	feedFeeds  []string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List articles, newest first",
	RunE: func(cmd *cobra.Command, args []string) error {
		v, _, err := openActiveVault()
		if err != nil {
			return err
		}
		f, err := listOpts.filter()
		if err != nil {
			return err
		}
		items, err := listing.List(v, f)
		if err != nil {
			return err
		}
		return listOpts.print(items, publishedDate)
	},
}

var feedCmd = &cobra.Command{
	Use:   "feed",
	Short: "TSV of unread items, newest first (nvim-friendly)",
	RunE: func(cmd *cobra.Command, args []string) error {
		v, _, err := openActiveVault()
		if err != nil {
			return err
		}
		items, err := listing.List(v, listing.Filter{
			Unread: true,
			Groups: feedGroups,
			Feeds:  feedFeeds,
		})
		if err != nil {
			return err
		}
		return printTSV(items)
	},
}

func (lf *listFlags) register(c *cobra.Command, sinceHelp string) {
	fl := c.Flags()
	fl.BoolVar(&lf.unread, "unread", false, "only unread items")
	fl.BoolVar(&lf.favorite, "fav", false, "only favorite items")
	fl.StringSliceVar(&lf.feeds, "feed", nil,
		"filter to a feed (repeatable; exact name match)")
	fl.StringSliceVar(&lf.groups, "group", nil,
		"filter to a folder under feeds/ "+
			"(repeatable; subtree match; \"/\" = top level only)")
	fl.StringVar(&lf.tag, "tag", "", "filter by tag")
	fl.StringVar(&lf.since, "since", "", sinceHelp)
	fl.BoolVar(&lf.corrupted, "corrupted", false,
		"only items with corruption marks")
	fl.BoolVar(&lf.tsv, "tsv", false,
		"tab-separated output (machine-friendly)")
	fl.BoolVar(&lf.json, "json", false, "JSON output")
}

func (lf *listFlags) filter() (listing.Filter, error) {
	f := listing.Filter{
		Feeds:     lf.feeds,
		Groups:    lf.groups,
		Tag:       lf.tag,
		Unread:    lf.unread,
		Favorite:  lf.favorite,
		Corrupted: lf.corrupted,
	}
	if lf.since != "" {
		d, err := listing.ParseSince(lf.since)
		if err != nil {
			return f, fmt.Errorf("invalid --since: %w", err)
		}
		f.Since = d
	}
	return f, nil
}

// dateFunc picks the time the human-readable listing shows per item.
type dateFunc func(listing.Item) string

// print writes items in the chosen format.
func (lf *listFlags) print(items []listing.Item, date dateFunc) error {
	switch {
	case lf.json:
		return printJSON(items)
	case lf.tsv:
		return printTSV(items)
	default:
		return printPretty(items, date)
	}
}

func publishedDate(it listing.Item) string {
	return it.Published.Format("2006-01-02")
}

func printTSV(items []listing.Item) error {
	for _, it := range items {
		// Group and Opened are appended, not inserted: existing
		// consumers index fields 1-6 and must keep working.
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s",
			it.Path,
			it.Feed,
			it.Published.Format("2006-01-02"),
			boolBit(it.Read),
			boolBit(it.Favorite),
			tabSafe(it.Label()),
			it.Group,
		)
		if it.Opened != nil {
			fmt.Printf("\t%s", it.Opened.Format(time.RFC3339))
		}
		fmt.Println()
	}
	return nil
}

func boolBit(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func tabSafe(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func printJSON(items []listing.Item) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

func printPretty(items []listing.Item, date dateFunc) error {
	if len(items) == 0 {
		fmt.Println("(no articles)")
		return nil
	}
	feedW := 4
	for _, it := range items {
		if l := len(it.Feed); l > feedW {
			feedW = l
		}
	}
	if feedW > 20 {
		feedW = 20
	}
	for _, it := range items {
		feed := it.Feed
		if len(feed) > feedW {
			feed = feed[:feedW]
		}
		fmt.Printf("[%s%s%s] %s  %-*s  %s\n",
			stateChar(it.Read, "R"),
			stateChar(it.Favorite, "F"),
			stateChar(it.Corrupt > 0, "C"),
			date(it),
			feedW, feed,
			it.Label(),
		)
	}
	return nil
}

func stateChar(set bool, ch string) string {
	if set {
		return ch
	}
	return " "
}

func init() {
	listOpts.register(listCmd, "only items newer than (e.g. 7d, 24h)")
	feedCmd.Flags().StringSliceVar(&feedGroups, "group", nil,
		"filter to a folder under feeds/ (repeatable; subtree match)")
	feedCmd.Flags().StringSliceVar(&feedFeeds, "feed", nil,
		"filter to a feed (repeatable; exact name match)")
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(feedCmd)
}

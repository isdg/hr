package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/listing"
)

var (
	feedsJSON   bool
	feedsGroups []string
)

var feedsCmd = &cobra.Command{
	Use:   "feeds",
	Short: "List the feeds in hr.toml with their counts and descriptions",
	Long: `Shows every feed declared in hr.toml, by group then name: its
group, article and unread counts, and its description. --json adds
"about", the path of the feed's README.txt when it has one.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, cfg, err := openActiveVault()
		if err != nil {
			return err
		}
		feeds, err := listing.Feeds(v, cfg.Feeds, feedsGroups)
		if err != nil {
			return err
		}
		if feedsJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(feeds)
		}
		printFeeds(feeds)
		return nil
	},
}

func printFeeds(feeds []listing.Feed) {
	if len(feeds) == 0 {
		fmt.Println("(no feeds)")
		return
	}
	nameW, groupW := len("FEED"), len("GROUP")
	for _, f := range feeds {
		nameW = max(nameW, len(f.Name))
		groupW = max(groupW, len(groupDisplayName(f.Group)))
	}
	row := func(name, group, articles, unread, desc string) {
		line := fmt.Sprintf("%-*s  %-*s  %8s  %6s  %s",
			nameW, name, groupW, group, articles, unread, desc)
		fmt.Println(strings.TrimRight(line, " "))
	}
	row("FEED", "GROUP", "ARTICLES", "UNREAD", "DESCRIPTION")
	for _, f := range feeds {
		row(f.Name, groupDisplayName(f.Group), strconv.Itoa(f.Articles),
			strconv.Itoa(f.Unread), f.Description)
	}
}

func init() {
	feedsCmd.Flags().BoolVar(&feedsJSON, "json", false, "JSON output")
	feedsCmd.Flags().StringSliceVar(&feedsGroups, "group", nil,
		"filter to a folder under feeds/ "+
			"(repeatable; subtree match; \"/\" = top level only)")
	rootCmd.AddCommand(feedsCmd)
}

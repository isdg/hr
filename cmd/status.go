package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/cache"
	"github.com/isdg/hr/internal/corrupt"
	"github.com/isdg/hr/internal/listing"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show vault summary: article counts, corruptions, last sync",
	RunE: func(cmd *cobra.Command, args []string) error {
		v, cfg, err := openActiveVault()
		if err != nil {
			return err
		}

		items, err := listing.List(v, listing.Filter{})
		if err != nil {
			return err
		}
		var unread, favorite int
		for _, it := range items {
			if !it.Read {
				unread++
			}
			if it.Favorite {
				favorite++
			}
		}

		recs, err := corrupt.ListAll(v)
		if err != nil {
			return err
		}
		var marks, stale int
		for _, rec := range recs {
			marks += len(rec.Corruptions)
			for _, c := range rec.Corruptions {
				if c.Stale {
					stale++
				}
			}
		}

		fmt.Printf("  feeds:       %d\n", len(cfg.Feeds))
		fmt.Printf("  articles:    %d\n", len(items))
		fmt.Printf("  unread:      %d\n", unread)
		fmt.Printf("  favorites:   %d\n", favorite)
		corruptLine := fmt.Sprintf("%d marks in %d articles", marks, len(recs))
		if stale > 0 {
			corruptLine += fmt.Sprintf(" (%d stale)", stale)
		}
		fmt.Printf("  corruptions: %s\n", corruptLine)
		fmt.Printf("  last sync:   %s\n", lastSync(v.CachePath()))
		return nil
	},
}

// lastSync returns the most recent feed fetch time from the cache, or
// "never" if there is none.
func lastSync(cachePath string) string {
	c, err := cache.Load(cachePath)
	if err != nil {
		return "unknown"
	}
	var latest time.Time
	for _, e := range c.Entries {
		if e.FetchedAt.After(latest) {
			latest = e.FetchedAt
		}
	}
	if latest.IsZero() {
		return "never"
	}
	return latest.Local().Format("2006-01-02 15:04")
}

func init() { rootCmd.AddCommand(statusCmd) }

// Package listing scans a vault and returns articles with their
// metadata for human/machine display.
package listing

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/isg/hrb/internal/article"
	"github.com/isg/hrb/internal/meta"
	"github.com/isg/hrb/internal/vault"
)

type Item struct {
	Path      string    `json:"path"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Feed      string    `json:"feed"`
	GUID      string    `json:"guid,omitempty"`
	Published time.Time `json:"published"`
	Read      bool      `json:"read"`
	Favorite  bool      `json:"favorite"`
	Tags      []string  `json:"tags,omitempty"`
}

type Filter struct {
	Feed   string
	Tag    string
	Unread bool
	Since  time.Duration
}

func List(v *vault.Vault, f Filter) ([]Item, error) {
	feedsDir := v.FeedsDir()
	now := time.Now()
	items := make([]Item, 0)

	walk := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		it, err := loadItem(path)
		if err != nil {
			return nil
		}
		if !f.match(it, now) {
			return nil
		}
		items = append(items, it)
		return nil
	}
	if err := filepath.WalkDir(feedsDir, walk); err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Published.After(items[j].Published)
	})
	return items, nil
}

func loadItem(path string) (Item, error) {
	fm, err := article.ParseFile(path)
	if err != nil {
		return Item{}, err
	}
	m := meta.LoadOrDefault(path)
	pub, _ := time.Parse(time.RFC3339, fm.Published)
	return Item{
		Path:      path,
		Title:     fm.Title,
		URL:       fm.URL,
		Feed:      fm.Feed,
		GUID:      fm.GUID,
		Published: pub,
		Read:      m.Read,
		Favorite:  m.Favorite,
		Tags:      m.Tags,
	}, nil
}

func (f Filter) match(it Item, now time.Time) bool {
	if f.Feed != "" && it.Feed != f.Feed {
		return false
	}
	if f.Tag != "" && !slices.Contains(it.Tags, f.Tag) {
		return false
	}
	if f.Unread && it.Read {
		return false
	}
	if f.Since > 0 && it.Published.Before(now.Add(-f.Since)) {
		return false
	}
	return true
}

// ParseSince accepts standard Go durations (e.g. 24h) plus the "Nd"
// shorthand for N days.
func ParseSince(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil {
			return 0, fmt.Errorf("invalid days: %w", err)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

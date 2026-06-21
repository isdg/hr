// Package listing scans a vault and returns articles with their
// metadata for human/machine display.
package listing

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/isdg/hr/internal/article"
	"github.com/isdg/hr/internal/meta"
	"github.com/isdg/hr/internal/textfmt"
	"github.com/isdg/hr/internal/vault"
)

type Item struct {
	Path      string    `json:"path"`
	Title     string    `json:"title"`
	Alias     string    `json:"alias,omitempty"`
	URL       string    `json:"url"`
	Feed      string    `json:"feed"`
	GUID      string    `json:"guid,omitempty"`
	Published time.Time `json:"published"`
	Read      bool      `json:"read"`
	Favorite  bool      `json:"favorite"`
	Tags      []string  `json:"tags,omitempty"`
}

// Label returns the alias if set, otherwise the article title.
func (it Item) Label() string {
	if it.Alias != "" {
		return it.Alias
	}
	return it.Title
}

type Filter struct {
	Feed   string
	Tag    string
	Unread bool
	Since  time.Duration
}

func List(v *vault.Vault, f Filter) ([]Item, error) {
	all, err := loadAll(v)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	items := make([]Item, 0, len(all))
	for _, it := range all {
		if f.match(it, now) {
			items = append(items, it)
		}
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
	pub := parsePublished(fm.Published)
	return Item{
		Path:      path,
		Title:     textfmt.Line(fm.Title),
		Alias:     textfmt.Line(m.Alias),
		URL:       textfmt.Line(fm.URL),
		Feed:      textfmt.Line(fm.Feed),
		GUID:      textfmt.Line(fm.GUID),
		Published: pub,
		Read:      m.Read,
		Favorite:  m.Favorite,
		Tags:      m.Tags,
	}, nil
}

// parsePublished parses an RFC3339 timestamp, tolerating the signed
// negative years (e.g. "-0350-07-03T00:00:00Z") that hr writes for
// BC-dated articles but that Go's RFC3339 layout refuses to parse back.
// Returns the zero time if the value is unparseable.
func parsePublished(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	rest, ok := strings.CutPrefix(s, "-")
	if !ok {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, rest)
	if err != nil {
		return time.Time{}
	}
	return time.Date(-t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
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

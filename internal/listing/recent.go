package listing

import (
	"slices"
	"strings"
	"time"

	"github.com/isdg/hr/internal/history"
	"github.com/isdg/hr/internal/vault"
)

// Recent lists the opened articles matching f, most recently opened
// first. Here f.Since bounds the open time, not the publication date.
// Open times are local, so consumers can show them without converting.
func Recent(v *vault.Vault, f Filter, hist []history.Entry) ([]Item, error) {
	latest := history.Latest(hist)
	if len(latest) == 0 {
		return nil, nil
	}
	all, err := loadMarked(v)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	since := f.Since
	f.Since = 0
	var items []Item
	for _, it := range all {
		at, ok := latest[history.KeyFor(it.Feed, it.Path)]
		if !ok || !f.match(it, now) {
			continue
		}
		if since > 0 && at.Before(now.Add(-since)) {
			continue
		}
		local := at.Local()
		it.Opened = &local
		items = append(items, it)
	}

	slices.SortFunc(items, func(a, b Item) int {
		if c := b.Opened.Compare(*a.Opened); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	return items, nil
}

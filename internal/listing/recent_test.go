package listing

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isdg/hr/internal/history"
	"github.com/isdg/hr/internal/vault"
)

// opened returns a history entry for the article titled title, opened
// d ago, resolving its key the way `hr opened` does.
func opened(
	t *testing.T, v *vault.Vault, title string, d time.Duration,
) history.Entry {
	t.Helper()
	all, err := List(v, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range all {
		if it.Title == title {
			k := history.KeyFor(it.Feed, it.Path)
			at := time.Now().Add(-d)
			return history.Entry{At: at, Feed: k.Feed, ID: k.ID}
		}
	}
	t.Fatalf("no article titled %q", title)
	return history.Entry{}
}

func TestRecentOrdersByLastOpen(t *testing.T) {
	v := recentVault(t)
	hist := []history.Entry{
		opened(t, v, "days30", 3*time.Hour),
		opened(t, v, "bc", 2*time.Hour),
		opened(t, v, "hour", time.Hour),
		// Reopening days30 lifts it to the top.
		opened(t, v, "days30", time.Minute),
	}
	items, err := Recent(v, Filter{}, hist)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"days30", "hour", "bc"}
	if got := titles(items); !slices.Equal(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
	for _, it := range items {
		if it.Opened == nil {
			t.Errorf("%s has no Opened time", it.Title)
		} else if it.Opened.Location() != time.Local {
			t.Errorf("%s opened in %v, want local time",
				it.Title, it.Opened.Location())
		}
	}
	if age := time.Since(*items[0].Opened); age < 0 || age > time.Hour {
		t.Errorf("days30 opened %v ago, want its latest open (~1m)", age)
	}
}

func TestRecentEmptyHistory(t *testing.T) {
	items, err := Recent(recentVault(t), Filter{}, nil)
	if err != nil || len(items) != 0 {
		t.Errorf("Recent(no history) = %v, %v; want none", titles(items), err)
	}
}

// --since in recent bounds when an article was opened, not published:
// an old article opened today is recent reading.
func TestRecentSinceIsOpenTime(t *testing.T) {
	v := recentVault(t)
	hist := []history.Entry{
		opened(t, v, "bc", time.Hour),
		opened(t, v, "hour", 3*day),
		opened(t, v, "days3", 30*day),
	}
	items, err := Recent(v, Filter{Since: 7 * day}, hist)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bc", "hour"}
	if got := titles(items); !slices.Equal(got, want) {
		t.Errorf("recent since 7d = %v, want %v", got, want)
	}
}

// The rest of the filter applies as in hr list.
func TestRecentComposesWithFilter(t *testing.T) {
	v := recentVault(t)
	hist := []history.Entry{
		opened(t, v, "days3", time.Hour),
		opened(t, v, "hour", 2*time.Hour),
		opened(t, v, "bc", 3*time.Hour),
	}
	items, err := Recent(v, Filter{Groups: []string{"humans"}}, hist)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(items); !slices.Equal(got, []string{"hour"}) {
		t.Errorf("recent humans = %v, want [hour]", got)
	}
}

// History outlives articles: an entry whose file is gone is dropped,
// not an error, and the same id in another feed is a different article.
func TestRecentIgnoresUnknownEntries(t *testing.T) {
	v := recentVault(t)
	gone := opened(t, v, "days3", time.Minute)
	hour := opened(t, v, "hour", time.Hour)
	hist := []history.Entry{
		gone,
		hour,
		{At: time.Now(), Feed: "nosuchfeed", ID: hour.ID},
	}
	md, _ := filepath.Glob(filepath.Join(v.FeedsDir(), "sites", "lobsters",
		"days3-*.md"))
	if len(md) != 1 {
		t.Fatalf("days3 files: %v", md)
	}
	if err := os.Remove(md[0]); err != nil {
		t.Fatal(err)
	}
	items, err := Recent(v, Filter{}, hist)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(items); !slices.Equal(got, []string{"hour"}) {
		t.Errorf("recent = %v, want [hour]", got)
	}
}

// Opens tie when recorded in the same instant; the order must still be
// stable from run to run, so ties fall back to the path.
func TestRecentTiesAreDeterministic(t *testing.T) {
	v := recentVault(t)
	at := time.Now().Add(-time.Hour)
	var hist []history.Entry
	for _, title := range []string{"hour", "bc", "days3", "future"} {
		e := opened(t, v, title, 0)
		e.At = at
		hist = append(hist, e)
	}
	first, err := Recent(v, Filter{}, hist)
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		again, err := Recent(v, Filter{}, hist)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(titles(again), titles(first)) {
			t.Fatalf("tie order changed: %v then %v",
				titles(first), titles(again))
		}
	}
	if !slices.IsSortedFunc(first, func(a, b Item) int {
		return strings.Compare(a.Path, b.Path)
	}) {
		t.Errorf("ties not ordered by path: %v", titles(first))
	}
}

// Open times come from the history, never the listing index: after
// Recent, a plain List must not carry them.
func TestRecentDoesNotLeakIntoList(t *testing.T) {
	v := recentVault(t)
	if _, err := Recent(v, Filter{},
		[]history.Entry{opened(t, v, "hour", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	items, err := List(v, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Opened != nil {
			t.Errorf("List item %s carries Opened %v", it.Title, it.Opened)
		}
	}
}

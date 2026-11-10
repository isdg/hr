package listing

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/isdg/hr/internal/vault"
)

// mkDated writes an article under feeds/<rel> whose published field is
// pub, verbatim, and returns its path. Ids derive from the title, so
// titles must be unique.
func mkDated(t *testing.T, v *vault.Vault, rel, title, pub string) string {
	t.Helper()
	dir := filepath.Join(v.FeedsDir(), filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("---\ntitle: %q\nurl: \"http://x.test/%s\"\n"+
		"published: %q\nfeed: %q\n---\n\nbody\n",
		title, title, pub, filepath.Base(rel))
	id := fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(title)))
	md := filepath.Join(dir, title+"-"+id+".md")
	if err := os.WriteFile(md, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return md
}

func ago(d time.Duration) string {
	return time.Now().Add(-d).UTC().Format(time.RFC3339)
}

func titles(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}

const day = 24 * time.Hour

// recentVault spans the dates --since has to tell apart. Margins are
// hours or more, so the real clock in List cannot flake them.
func recentVault(t *testing.T) *vault.Vault {
	t.Helper()
	v := &vault.Vault{Root: t.TempDir()}
	mkDated(t, v, "humans/matklad", "future", ago(-2*day))
	mkDated(t, v, "humans/matklad", "hour", ago(time.Hour))
	mkDated(t, v, "sites/lobsters", "days3", ago(3*day))
	mkDated(t, v, "humans/matklad", "days30", ago(30*day))
	mkDated(t, v, "books/aristotle", "bc", "-0350-07-03T00:00:00Z")
	mkDated(t, v, "sites/lobsters", "undated", "not a date")
	return v
}

// The listing is newest first. Undated items carry the zero time
// (year 1), so they sort below every AD date but above BC ones.
func TestListNewestFirst(t *testing.T) {
	items, err := List(recentVault(t), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"future", "hour", "days3", "days30", "undated", "bc"}
	if got := titles(items); !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

// Offsets are honoured: a later wall clock in an earlier zone is still
// the earlier instant.
func TestListOrdersByInstantNotWallClock(t *testing.T) {
	v := &vault.Vault{Root: t.TempDir()}
	mkDated(t, v, "f", "utc", "2024-05-01T10:00:00Z")
	mkDated(t, v, "f", "ny", "2024-05-01T08:00:00-04:00")    // 12:00Z
	mkDated(t, v, "f", "tokyo", "2024-05-01T18:00:00+09:00") // 09:00Z
	items, err := List(v, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ny", "utc", "tokyo"}
	if got := titles(items); !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestFilterSince(t *testing.T) {
	v := recentVault(t)
	cases := []struct {
		since time.Duration
		want  []string
	}{
		{0, []string{"future", "hour", "days3", "days30", "undated", "bc"}},
		{2 * time.Hour, []string{"future", "hour"}},
		{7 * day, []string{"future", "hour", "days3"}},
		{365 * day, []string{"future", "hour", "days3", "days30"}},
	}
	for _, tc := range cases {
		items, err := List(v, Filter{Since: tc.since})
		if err != nil {
			t.Fatal(err)
		}
		if got := titles(items); !slices.Equal(got, tc.want) {
			t.Errorf("since %v = %v, want %v", tc.since, got, tc.want)
		}
	}
}

// The cutoff is inclusive: an item published exactly Since ago stays.
func TestFilterSinceBoundary(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	f := Filter{Since: 7 * day}
	cutoff := now.Add(-f.Since)
	for _, tc := range []struct {
		pub  time.Time
		want bool
	}{
		{cutoff, true},
		{cutoff.Add(time.Nanosecond), true},
		{cutoff.Add(-time.Nanosecond), false},
		{now, true},
		{now.Add(day), true},
		{time.Time{}, false},
	} {
		it := Item{Published: PubTime{tc.pub}}
		if got := f.match(it, now); got != tc.want {
			t.Errorf("match(%v) = %v, want %v", tc.pub, got, tc.want)
		}
	}
}

// --since narrows the other filters rather than replacing them.
func TestFilterSinceComposes(t *testing.T) {
	v := recentVault(t)
	items, err := List(v, Filter{
		Since:  7 * day,
		Groups: []string{"humans"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"future", "hour"}
	if got := titles(items); !slices.Equal(got, want) {
		t.Errorf("humans since 7d = %v, want %v", got, want)
	}

	items, err = List(v, Filter{Since: 7 * day, Feeds: []string{"lobsters"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(items); !slices.Equal(got, []string{"days3"}) {
		t.Errorf("lobsters since 7d = %v, want [days3]", got)
	}
}

// Dates round-trip through the listing index as RFC3339, so a second
// List served from the cache must filter and order exactly as the first.
func TestRecencySurvivesCachedIndex(t *testing.T) {
	v := recentVault(t)
	first, err := List(v, Filter{Since: 7 * day})
	if err != nil {
		t.Fatal(err)
	}
	second, err := List(v, Filter{Since: 7 * day})
	if err != nil {
		t.Fatal(err)
	}
	if a, b := titles(first), titles(second); !slices.Equal(a, b) {
		t.Errorf("cached listing = %v, fresh = %v", b, a)
	}
	all, err := List(v, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(all); got[len(got)-1] != "bc" {
		t.Errorf("BC date lost in the cache: order %v", got)
	}
}

func TestParseSince(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want time.Duration
	}{
		{"7d", 7 * day},
		{"1d", day},
		{"0d", 0},
		{"24h", day},
		{"90m", 90 * time.Minute},
		{"1h30m", 90 * time.Minute},
		{"106751d", 106751 * day}, // the largest whole-day Duration
	} {
		got, err := ParseSince(tc.in)
		if err != nil {
			t.Errorf("ParseSince(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseSince(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// A negative or overflowing window must be refused: Filter treats
// Since <= 0 as "no filter", so either would silently list everything,
// or (on wrap-around) a window nobody asked for.
func TestParseSinceRejects(t *testing.T) {
	for _, in := range []string{
		"", "d", "xd", "7", "7w", "1.5d", "1d12h",
		"-1d", "-24h", "106752d", "213504d", "99999999999999999999d",
	} {
		if d, err := ParseSince(in); err == nil {
			t.Errorf("ParseSince(%q) = %v, want an error", in, d)
		}
	}
}

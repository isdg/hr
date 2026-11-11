package listing

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/isdg/hr/internal/bookmark"
	"github.com/isdg/hr/internal/vault"
)

// mkArticle writes a minimal article at feeds/<rel>/<name>.md.
func mkArticle(t *testing.T, v *vault.Vault, rel, title string, read bool) {
	t.Helper()
	dir := filepath.Join(v.FeedsDir(), filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	feed := filepath.Base(rel)
	body := fmt.Sprintf(
		"---\ntitle: %q\nurl: \"http://x.test/%s\"\npublished: \"2020-01-01T00:00:00Z\"\nfeed: %q\n---\n\nbody\n",
		title, title, feed)
	md := filepath.Join(dir, "2020-01-01-"+title+"-aaaaaaaa.md")
	if err := os.WriteFile(md, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if read {
		sidecar := filepath.Join(dir, "2020-01-01-"+title+"-aaaaaaaa.meta.toml")
		if err := os.WriteFile(sidecar, []byte("read = true\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func testVault(t *testing.T) *vault.Vault {
	t.Helper()
	v := &vault.Vault{Root: t.TempDir(), State: t.TempDir()}
	mkArticle(t, v, "lobsters", "flat", false)
	mkArticle(t, v, "humans/matklad", "nested", false)
	mkArticle(t, v, "humans/archive/ewd", "deep", true)
	mkArticle(t, v, "books/plato", "book", false)
	return v
}

// Every article carries the folder its feed lives in.
func TestListPopulatesGroup(t *testing.T) {
	v := testVault(t)
	items, err := List(v, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}
	want := map[string]string{
		"flat":   "",
		"nested": "humans",
		"deep":   "humans/archive",
		"book":   "books",
	}
	for _, it := range items {
		if got := it.Group; got != want[it.Title] {
			t.Errorf("%s group = %q, want %q", it.Title, got, want[it.Title])
		}
	}
}

// --group matches by subtree, so a parent shelf includes its nested
// ones. Naming the root explicitly ("/") matches only top-level feeds.
func TestFilterByGroupIsSubtree(t *testing.T) {
	v := testVault(t)
	cases := []struct {
		groups []string
		want   []string
	}{
		{[]string{"humans"}, []string{"nested", "deep"}},
		{[]string{"humans/archive"}, []string{"deep"}},
		{[]string{"books"}, []string{"book"}},
		{[]string{"/"}, []string{"flat"}},
		{nil, []string{"flat", "nested", "deep", "book"}},
		{[]string{"nosuchgroup"}, nil},
		// A group is a path, not a string prefix: "human" must not
		// match "humans".
		{[]string{"human"}, nil},
		// Stacked groups union, and overlapping ones don't duplicate.
		{[]string{"books", "humans/archive"}, []string{"book", "deep"}},
		{[]string{"humans", "humans/archive"}, []string{"nested", "deep"}},
		{[]string{"books", "/"}, []string{"book", "flat"}},
		{[]string{"nosuchgroup", "books"}, []string{"book"}},
	}
	for _, tc := range cases {
		items, err := List(v, Filter{Groups: tc.groups})
		if err != nil {
			t.Fatalf("List(groups=%v): %v", tc.groups, err)
		}
		got := map[string]bool{}
		for _, it := range items {
			got[it.Title] = true
		}
		if len(got) != len(tc.want) {
			t.Errorf("groups %v matched %d items, want %d",
				tc.groups, len(got), len(tc.want))
			continue
		}
		for _, w := range tc.want {
			if !got[w] {
				t.Errorf("groups %v missed %q", tc.groups, w)
			}
		}
	}
}

// --group composes with the other filters rather than replacing them.
func TestFilterGroupCombinesWithUnread(t *testing.T) {
	v := testVault(t)
	items, err := List(v, Filter{Groups: []string{"humans"}, Unread: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "nested" {
		t.Errorf("got %+v, want only the unread humans item", items)
	}
}

func TestGroups(t *testing.T) {
	v := testVault(t)
	groups, err := Groups(v)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Group{}
	for _, g := range groups {
		got[g.Name] = g
	}
	for _, want := range []Group{
		{Name: "", Feeds: 1, Articles: 1, Unread: 1},
		{Name: "books", Feeds: 1, Articles: 1, Unread: 1},
		{Name: "humans", Feeds: 1, Articles: 1, Unread: 1},
		{Name: "humans/archive", Feeds: 1, Articles: 1, Unread: 0},
	} {
		if g := got[want.Name]; g != want {
			t.Errorf("group %q = %+v, want %+v", want.Name, g, want)
		}
	}
	if len(groups) != 4 {
		t.Errorf("got %d groups, want 4", len(groups))
	}
	// Counts are per-group, not cumulative: they must sum to the total.
	total := 0
	for _, g := range groups {
		total += g.Articles
	}
	if total != 4 {
		t.Errorf("article counts sum to %d, want 4 (subgroups double-counted?)", total)
	}
}

// The listing cache keys on mod times; a stale index built before Group
// existed must not survive a version bump.
func TestGroupSurvivesCachedIndex(t *testing.T) {
	v := testVault(t)
	if _, err := List(v, Filter{}); err != nil { // populate the cache
		t.Fatal(err)
	}
	items, err := List(v, Filter{Groups: []string{"humans/archive"}}) // read it back
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Group != "humans/archive" {
		t.Errorf("cached items lost their group: %+v", items)
	}
}

// --feed stacks the same way --group does, so several authors can be
// read in one command. Matching is exact: a feed is a name, not a path.
func TestFilterByFeedStacks(t *testing.T) {
	v := testVault(t)
	cases := []struct {
		feeds []string
		want  []string
	}{
		{[]string{"matklad"}, []string{"nested"}},
		{[]string{"matklad", "plato"}, []string{"nested", "book"}},
		{[]string{"matklad", "ewd", "lobsters"}, []string{"nested", "deep", "flat"}},
		{nil, []string{"flat", "nested", "deep", "book"}},
		{[]string{"nosuchfeed"}, nil},
		{[]string{"nosuchfeed", "plato"}, []string{"book"}},
		// Duplicates must not duplicate results.
		{[]string{"plato", "plato"}, []string{"book"}},
		// Exact match only: no prefix or substring behaviour.
		{[]string{"matk"}, nil},
		{[]string{"matklad2"}, nil},
	}
	for _, tc := range cases {
		items, err := List(v, Filter{Feeds: tc.feeds})
		if err != nil {
			t.Fatalf("List(feeds=%v): %v", tc.feeds, err)
		}
		if len(items) != len(tc.want) {
			t.Errorf("feeds %v matched %d items, want %d",
				tc.feeds, len(items), len(tc.want))
			continue
		}
		got := map[string]bool{}
		for _, it := range items {
			got[it.Title] = true
		}
		for _, w := range tc.want {
			if !got[w] {
				t.Errorf("feeds %v missed %q", tc.feeds, w)
			}
		}
	}
}

// --feed and --group intersect (both must hold), while values within
// each flag union. Mixing them narrows rather than widens.
func TestFilterFeedAndGroupIntersect(t *testing.T) {
	v := testVault(t)
	// ewd is in humans/archive; plato is in books. Asking for both
	// feeds but only the books shelf leaves just plato.
	items, err := List(v, Filter{
		Feeds:  []string{"ewd", "plato"},
		Groups: []string{"books"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "book" {
		t.Errorf("got %+v, want only the books-shelf item", items)
	}

	// A feed outside every named group yields nothing.
	items, err = List(v, Filter{
		Feeds:  []string{"matklad"},
		Groups: []string{"books"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("got %+v, want none", items)
	}
}

// --fav keeps favorites only and composes with the other filters.
func TestFilterFavorite(t *testing.T) {
	v := testVault(t)
	for rel, title := range map[string]string{
		"humans/matklad": "nested",
		"books/plato":    "book",
	} {
		name := "2020-01-01-" + title + "-aaaaaaaa.meta.toml"
		sidecar := filepath.Join(v.FeedsDir(), filepath.FromSlash(rel), name)
		err := os.WriteFile(sidecar, []byte("favorite = true\n"), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		f    Filter
		want []string
	}{
		{Filter{Favorite: true}, []string{"book", "nested"}},
		{Filter{Favorite: true, Groups: []string{"humans"}},
			[]string{"nested"}},
		{Filter{Favorite: true, Feeds: []string{"lobsters"}}, nil},
	}
	for _, tc := range cases {
		items, err := List(v, tc.f)
		if err != nil {
			t.Fatal(err)
		}
		got := titles(items)
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("List(%+v) = %v, want %v", tc.f, got, tc.want)
		}
	}
}

// Listings carry each article's latest reading place from the bookmark
// log, and --bookmarked keeps only those that have one.
func TestBookmarks(t *testing.T) {
	v := testVault(t)
	all, err := List(v, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	path := map[string]string{}
	for _, it := range all {
		path[it.Title] = it.Path
	}
	at := time.Now()
	for _, e := range []struct {
		title string
		pos   bookmark.Position
	}{
		{"nested", bookmark.Position{Line: 6}},
		{"deep", bookmark.Position{Line: 6, Col: 2}},
		{"book", bookmark.Position{Line: 6}},
	} {
		if err := bookmark.Set(v.BookmarksPath(), path[e.title], e.pos,
			at); err != nil {
			t.Fatal(err)
		}
	}
	err = bookmark.Clear(v.BookmarksPath(), path["book"], at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}

	items, err := List(v, Filter{Bookmarked: true})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bookmark.Position{}
	for _, it := range items {
		got[it.Title] = *it.Bookmark
	}
	want := map[string]bookmark.Position{
		"nested": {Line: 6},
		"deep":   {Line: 6, Col: 2},
	}
	if !maps.Equal(got, want) {
		t.Errorf("bookmarked = %v, want %v", got, want)
	}
	for _, it := range all {
		if it.Bookmark != nil {
			t.Errorf("%s marked before any bookmark was set", it.Title)
		}
	}
	items, err = List(v, Filter{Bookmarked: true, Unread: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(items); !slices.Equal(got, []string{"nested"}) {
		t.Errorf("bookmarked unread = %v, want [nested]", got)
	}
}

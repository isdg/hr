package listing

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/isdg/hr/internal/config"
	"github.com/isdg/hr/internal/vault"
)

func TestFeeds(t *testing.T) {
	v := testVault(t)
	about := filepath.Join(v.FeedsDir(), "humans", "matklad", vault.AboutFile)
	if err := os.WriteFile(about, []byte("Alex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	declared := []config.Feed{
		{Name: "matklad", Description: "Rust\n and IDEs"},
		{Name: "ewd"},
		{Name: "lobsters"},
		{Name: "fresh", Group: "humans/new"},
	}
	feeds, err := Feeds(v, declared, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []Feed{
		{Name: "lobsters", Group: "", Articles: 1, Unread: 1},
		{Name: "matklad", Group: "humans", Articles: 1, Unread: 1,
			Description: "Rust and IDEs", About: about},
		{Name: "ewd", Group: "humans/archive", Articles: 1},
		{Name: "fresh", Group: "humans/new"},
	}
	if !slices.Equal(feeds, want) {
		t.Errorf("got\n%+v\nwant\n%+v", feeds, want)
	}

	feeds, err = Feeds(v, declared, []string{"humans"})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range feeds {
		names = append(names, f.Name)
	}
	if want := []string{"matklad", "ewd", "fresh"}; !slices.Equal(names, want) {
		t.Errorf("--group humans: got %q, want %q", names, want)
	}
}

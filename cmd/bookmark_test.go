package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/isdg/hr/internal/bookmark"
	"github.com/isdg/hr/internal/vault"
)

// The bookmark lands in the log of the vault holding the article, found
// from the article itself, and outside a vault's feeds/ it is refused.
func TestBookmarkCommand(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "vault")
	write(t, filepath.Join(root, "hr.toml"), "name = \"t\"\n")
	md := filepath.Join(root, "feeds", "humans", "matklad",
		"2024-01-01-x-3f9a1c2e.md")
	write(t, md, "---\ntitle: \"x\"\nfeed: \"matklad\"\n---\n\nbody\n")
	write(t, filepath.Join(root, "notes.md"),
		"---\ntitle: \"n\"\nfeed: \"matklad\"\n---\n")
	t.Chdir(tmp)

	var out strings.Builder
	run := func(args ...string) error {
		out.Reset()
		clear := args[0] == "--clear"
		if clear {
			args = args[1:]
		}
		return runBookmark(&out, args, clear)
	}
	if err := run(md, "6:2"); err != nil {
		t.Fatal(err)
	}
	if err := run(md); err != nil || out.String() != "6:2\n" {
		t.Errorf("show = %q, %v; want 6:2", out.String(), err)
	}
	v, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	p, ok, err := bookmark.Get(v.BookmarksPath(), md)
	if err != nil || !ok || p != (bookmark.Position{Line: 6, Col: 2}) {
		t.Errorf("after set: %v, %v, %v; want 6:2", p, ok, err)
	}
	if err := run("--clear", md, "6"); err == nil {
		t.Error("--clear with a position succeeded, want an error")
	}
	if err := run("--clear", md); err != nil {
		t.Fatal(err)
	}
	if err := run(md); err != nil || out.Len() != 0 {
		t.Errorf("show after clear = %q, %v; want nothing", out.String(), err)
	}
	for _, args := range [][]string{
		{filepath.Join(root, "notes.md"), "1"},
		{md, "0"},
		{md, "99"},
	} {
		if err := run(args...); err == nil {
			t.Errorf("hr bookmark %v succeeded, want an error", args)
		}
	}
}

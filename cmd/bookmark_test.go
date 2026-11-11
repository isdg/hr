package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// --last names the newest live bookmark, and falls back to the one before
// when its article is gone from the vault.
func TestLastBookmark(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	write(t, filepath.Join(root, "hr.toml"), "name = \"last\"\n")
	dir := filepath.Join(root, "feeds", "humans", "matklad")
	older := filepath.Join(dir, "2024-01-01-a-11111111.md")
	newer := filepath.Join(dir, "2024-01-02-b-22222222.md")
	for _, p := range []string{older, newer} {
		write(t, p, "---\ntitle: \"x\"\nfeed: \"matklad\"\n---\n\nbody\n")
	}
	v, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := lastBookmark(&out, v); err != nil || out.Len() != 0 {
		t.Fatalf("no bookmarks: %q, %v; want nothing", out.String(), err)
	}

	at := time.Now()
	pos := bookmark.Position{Line: 6, Col: 1}
	if err := bookmark.Set(v.BookmarksPath(), older, pos, at); err != nil {
		t.Fatal(err)
	}
	pos.Col = 3
	err = bookmark.Set(v.BookmarksPath(), newer, pos, at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := lastBookmark(&out, v); err != nil ||
		out.String() != newer+"\t6:3\n" {
		t.Errorf("last = %q, %v; want %s at 6:3", out.String(), err, newer)
	}

	if err := os.Remove(newer); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := lastBookmark(&out, v); err != nil ||
		out.String() != older+"\t6:1\n" {
		t.Errorf("after delete = %q, %v; want %s at 6:1",
			out.String(), err, older)
	}
}

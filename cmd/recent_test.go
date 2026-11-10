package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/isdg/hr/internal/history"
	"github.com/isdg/hr/internal/vault"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// recordOpen finds the vault from the article itself, whatever the
// working directory, and sees through symlinks to it.
func TestRecordOpen(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "vault")
	write(t, filepath.Join(root, "hr.toml"), "name = \"t\"\n")
	md := filepath.Join(root, "feeds", "humans", "matklad",
		"2024-01-01-x-3f9a1c2e.md")
	write(t, md, "---\ntitle: \"x\"\nfeed: \"matklad\"\n---\n\nbody\n")
	write(t, filepath.Join(root, "notes.md"),
		"---\ntitle: \"n\"\nfeed: \"matklad\"\n---\n")
	write(t, filepath.Join(tmp, "elsewhere.md"),
		"---\ntitle: \"e\"\nfeed: \"matklad\"\n---\n")
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmp)

	at := time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)
	for _, p := range []string{
		md,
		filepath.Join("vault", "feeds", "humans", "matklad",
			"2024-01-01-x-3f9a1c2e.md"),
		filepath.Join(link, "feeds", "humans", "matklad",
			"2024-01-01-x-3f9a1c2e.md"),
	} {
		if err := recordOpen(p, at); err != nil {
			t.Errorf("recordOpen(%s): %v", p, err)
		}
	}
	for _, p := range []string{
		filepath.Join(root, "notes.md"),
		filepath.Join(tmp, "elsewhere.md"),
		filepath.Join(root, "feeds", "missing-aaaaaaaa.md"),
	} {
		if err := recordOpen(p, at); err == nil {
			t.Errorf("recordOpen(%s) succeeded, want an error", p)
		}
	}

	v := &vault.Vault{Root: root}
	got, err := history.Load(v.HistoryPath())
	if err != nil {
		t.Fatal(err)
	}
	want := history.Key{Feed: "matklad", ID: "3f9a1c2e"}
	if len(got) != 3 {
		t.Fatalf("logged %d opens, want 3: %+v", len(got), got)
	}
	for _, e := range got {
		if e.Key() != want {
			t.Errorf("logged %+v, want key %+v", e, want)
		}
	}
}

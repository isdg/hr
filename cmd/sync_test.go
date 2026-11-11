package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/isdg/hr/internal/vault"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestChangedSidecars(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"AUTHOR", "COMMITTER"} {
		t.Setenv("GIT_"+k+"_NAME", "hr test")
		t.Setenv("GIT_"+k+"_EMAIL", "hr@example.com")
	}
	v := &vault.Vault{Root: t.TempDir()}
	feed := filepath.Join(v.FeedsDir(), "x")
	if err := os.MkdirAll(feed, 0o755); err != nil {
		t.Fatal(err)
	}
	put := func(name string) string {
		p := filepath.Join(feed, name)
		if err := os.WriteFile(p, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, name := range []string{
		"edited.md", "edited.meta.toml", "gone.md", "gone.meta.toml",
		"read.md",
	} {
		put(name)
	}
	gitIn(t, v.Root, "init", "--quiet")
	gitIn(t, v.Root, "add", ".")
	gitIn(t, v.Root, "commit", "--quiet", "-m", "vault")

	edited := filepath.Join(feed, "edited.meta.toml")
	if err := os.WriteFile(edited, []byte("read = true"), 0o644); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(feed, "gone.meta.toml")
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	read := put("read.meta.toml")
	put("pending.md")
	put("pending.meta.toml")
	written := put("written.md")
	withWritten := put("written.meta.toml")
	put("notes.txt")

	got, err := changedSidecars(t.Context(), v, []string{written})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{edited, gone, read, withWritten}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("changedSidecars =\n%q\nwant\n%q", got, want)
	}
}

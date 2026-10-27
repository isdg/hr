package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// newRepo makes an isolated repository with one commit, so tests neither
// read nor depend on the developer's git config.
func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"AUTHOR", "COMMITTER"} {
		t.Setenv("GIT_"+k+"_NAME", "hr test")
		t.Setenv("GIT_"+k+"_EMAIL", "hr@example.com")
	}
	dir := t.TempDir()
	git(t, dir, "init", "--quiet")
	write(t, dir, "hr.toml")
	git(t, dir, "add", "hr.toml")
	git(t, dir, "commit", "--quiet", "-m", "init")
	return dir
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, rel string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(rel), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func committed(t *testing.T, dir string) []string {
	t.Helper()
	out := git(t, dir, "show", "--name-only", "--format=", "HEAD")
	return strings.Split(out, "\n")
}

func TestCommitOnlyGivenPaths(t *testing.T) {
	dir := newRepo(t)
	a := write(t, dir, "feeds/x/a.md")
	b := write(t, dir, "feeds/x/a.meta.toml")
	write(t, dir, "staged.txt")
	git(t, dir, "add", "staged.txt")
	write(t, dir, "untracked.txt")

	msg := "sync: 1 new article\n\nx  +1"
	if err := Commit(t.Context(), dir, msg, []string{a, b}); err != nil {
		t.Fatal(err)
	}

	want := []string{"feeds/x/a.md", "feeds/x/a.meta.toml"}
	if got := committed(t, dir); !slices.Equal(got, want) {
		t.Errorf("committed %q, want %q", got, want)
	}
	if got := git(t, dir, "log", "-1", "--format=%B"); got != msg {
		t.Errorf("message %q, want %q", got, msg)
	}
	status := git(t, dir, "status", "--porcelain")
	if !strings.Contains(status, "A  staged.txt") ||
		!strings.Contains(status, "?? untracked.txt") {
		t.Errorf("unrelated changes disturbed:\n%s", status)
	}
}

func TestCommitNoPaths(t *testing.T) {
	dir := newRepo(t)
	head := git(t, dir, "rev-parse", "HEAD")
	if err := Commit(t.Context(), dir, "x", nil); err != nil {
		t.Fatal(err)
	}
	if got := git(t, dir, "rev-parse", "HEAD"); got != head {
		t.Error("no paths still made a commit")
	}
}

func TestCommitLiteralPaths(t *testing.T) {
	dir := newRepo(t)
	p := write(t, dir, "a[1].md")
	write(t, dir, "a1.md")
	if err := Commit(t.Context(), dir, "x", []string{p}); err != nil {
		t.Fatal(err)
	}
	if got := committed(t, dir); !slices.Equal(got, []string{"a[1].md"}) {
		t.Errorf("committed %q, want only a[1].md", got)
	}
}

func TestCommitVaultInSubdir(t *testing.T) {
	repo := newRepo(t)
	vault := filepath.Join(repo, "vault")
	p := write(t, vault, "feeds/x/a.md")
	if err := Commit(t.Context(), vault, "x", []string{p}); err != nil {
		t.Fatal(err)
	}
	want := []string{"vault/feeds/x/a.md"}
	if got := committed(t, repo); !slices.Equal(got, want) {
		t.Errorf("committed %q, want %q", got, want)
	}
}

func TestCommitNotARepo(t *testing.T) {
	newRepo(t) // isolated git config only
	dir := t.TempDir()
	p := write(t, dir, "a.md")
	err := Commit(t.Context(), dir, "x", []string{p})
	const want = "not a git repository"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want a not-a-repository error", err)
	}
}

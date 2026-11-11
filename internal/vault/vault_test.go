package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isdg/hr/internal/config"
	"github.com/isdg/hr/internal/rc"
)

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hr.toml"), []byte("name = \"t\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "feeds", "demo")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, ok := Discover(sub)
	if !ok || got != root {
		t.Fatalf("Discover(sub) = %q, %v; want %q, true", got, ok, root)
	}

	got, ok = Discover(root)
	if !ok || got != root {
		t.Fatalf("Discover(root) = %q, %v; want %q, true", got, ok, root)
	}

	elsewhere := t.TempDir()
	if _, ok := Discover(elsewhere); ok {
		t.Fatal("Discover(elsewhere) should find nothing")
	}
}

// realpath resolves macOS's /var -> /private/var, which os.Getwd() may
// or may not report after a Chdir into a TempDir.
func realpath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// Precedence: -C, then the vault holding the current directory, then
// $HR_VAULT, which is a default and never pulls hr out of a vault.
func TestResolvePrecedence(t *testing.T) {
	here, env, flag := mkVault(t, "here"), mkVault(t, "env"),
		mkVault(t, "flag")
	t.Setenv("HR_VAULT", env)
	t.Chdir(here)

	if got, err := Resolve(flag); err != nil || got != flag {
		t.Errorf("Resolve(flag) = %q, %v; want %q", got, err, flag)
	}
	got, err := Resolve("")
	if err != nil || realpath(t, got) != realpath(t, here) {
		t.Errorf("inside a vault: Resolve = %q, %v; want %q",
			got, err, here)
	}
	t.Chdir(t.TempDir())
	if got, err := Resolve(""); err != nil || got != env {
		t.Errorf("outside: Resolve = %q, %v; want $HR_VAULT %q",
			got, err, env)
	}
}

func TestResolveExpandsTilde(t *testing.T) {
	t.Setenv("HR_VAULT", "~/feed")
	t.Chdir(t.TempDir())
	want := filepath.Join(os.Getenv("HOME"), "feed")
	if got, err := Resolve(""); err != nil || got != want {
		t.Errorf("Resolve = %q, %v; want %q", got, err, want)
	}
}

// With no vault anywhere, the error says how to name one, and points at
// a vault line in the user's rc, which hr no longer reads.
func TestResolveNoVault(t *testing.T) {
	t.Setenv("HR_VAULT", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	_, err := Resolve("")
	if err == nil || !strings.Contains(err.Error(), "$HR_VAULT") {
		t.Fatalf("Resolve error = %v, want one naming $HR_VAULT", err)
	}
	if strings.Contains(err.Error(), "no longer read") {
		t.Errorf("error %q blames an rc that has no vault line", err)
	}

	writeRC(t, "vault = \"/v\"\n")
	_, err = Resolve("")
	if err == nil || !strings.Contains(err.Error(), "no longer read") {
		t.Errorf("Resolve error = %v, want one naming the rc", err)
	}
}

func writeRC(t *testing.T, body string) {
	t.Helper()
	p, err := rc.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveNewIgnoresHRVault(t *testing.T) {
	t.Setenv("HR_VAULT", mkVault(t, "feed"))
	want := filepath.Join(os.Getenv("HOME"), "blogs", "blog")
	if got, err := ResolveNew("", "blog"); err != nil || got != want {
		t.Errorf("ResolveNew = %q, %v; want %q", got, err, want)
	}
}

// Every setting in the template must be top-level: below a [[feeds]]
// header TOML hands it to that feed, and hr silently ignores it.
func TestDefaultConfigSettingsAreTopLevel(t *testing.T) {
	p := filepath.Join(t.TempDir(), "hr.toml")
	if err := os.WriteFile(p, buildConfig("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(p, config.Settings{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Unknown) > 0 {
		t.Errorf("template has unrecognised keys %q", cfg.Unknown)
	}
	if cfg.Ordering != "desc" || cfg.UserAgent != "hr/0.1" {
		t.Errorf("settings not applied: %+v", cfg)
	}
}

func mkVault(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "v")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte("name = '" + name + "'\n") // literal: no escapes
	err := os.WriteFile(filepath.Join(root, "hr.toml"), cfg, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// A vault's state lives under the hr home, named after the vault, so
// nothing machine-local sits in the vault's repository.
func TestOpenStateUnderHome(t *testing.T) {
	t.Setenv("HR_HOME", t.TempDir())
	v, err := Open(mkVault(t, "blog"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(os.Getenv("HR_HOME"), "blog")
	if v.State != want {
		t.Errorf("State = %q, want %q", v.State, want)
	}
	for _, p := range []string{v.HistoryPath(), v.CachePath(),
		v.IndexPath(), v.RawDir(), v.LogDir()} {
		if !strings.HasPrefix(p, want+string(filepath.Separator)) {
			t.Errorf("%s is outside the state dir", p)
		}
	}
}

func TestOpenRejectsUnusableNames(t *testing.T) {
	t.Setenv("HR_HOME", t.TempDir())
	for _, name := range []string{"", ".", "..", "a/b", `a\b`,
		"config.toml"} {
		if _, err := Open(mkVault(t, name)); err == nil {
			t.Errorf("Open(name %q) succeeded, want an error", name)
		}
	}
}

// The old in-repo .hr/ moves to the state dir once; if the state dir
// already exists, the old one stays put for doctor to report.
func TestOpenAdoptsLegacyState(t *testing.T) {
	t.Setenv("HR_HOME", t.TempDir())
	root := mkVault(t, "blog")
	old := filepath.Join(root, ".hr", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(v.HistoryPath()); err != nil {
		t.Errorf("history not moved: %v", err)
	}
	if _, err := os.Stat(v.LegacyStateDir()); !os.IsNotExist(err) {
		t.Errorf("legacy .hr still there: %v", err)
	}

	if err := os.MkdirAll(v.LegacyStateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(v.LegacyStateDir()); err != nil {
		t.Errorf("second legacy .hr was moved over existing state: %v",
			err)
	}
}

func TestInitRefusesTakenName(t *testing.T) {
	t.Setenv("HR_HOME", t.TempDir())
	v, err := Init(filepath.Join(t.TempDir(), "a"), "blog")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(v.State); err != nil {
		t.Errorf("Init made no state dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(v.Root, ".gitignore")); err == nil {
		t.Error("Init still writes a .gitignore for an in-repo .hr/")
	}
	if _, err := Init(filepath.Join(t.TempDir(), "b"), "blog"); err == nil {
		t.Error("second vault named blog shares the first's state")
	}
}

func TestInitLeavesHrrcAlone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("HR_HOME", t.TempDir())
	if _, err := Init(filepath.Join(t.TempDir(), "v"), "blog"); err != nil {
		t.Fatal(err)
	}
	p, err := rc.Path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(p); !os.IsNotExist(err) {
		t.Errorf("Init wrote %s: %v", p, err)
	}
}

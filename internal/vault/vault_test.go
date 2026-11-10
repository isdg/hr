package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isdg/hr/internal/config"
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

func TestResolvePrefersDiscoveryOverRC(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hr.toml"), []byte("name = \"t\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir()) // empty ~/.hrrc
	t.Setenv("HR_VAULT", "")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	// macOS TempDir roots are under a /var symlink to /private/var;
	// os.Getwd() resolves it after Chdir, so compare resolved paths.
	wantReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantReal {
		t.Fatalf("Resolve() = %q, want %q", got, wantReal)
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

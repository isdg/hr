package rc

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// home makes a scratch HOME whose ~/.hrrc holds body ("" = no file),
// with HR_HOME unset, and returns the ~/.hrrc path.
func home(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("HR_HOME", "")
	p := filepath.Join(dir, ".hrrc")
	if body != "" {
		writeFile(t, p, body)
	}
	return p
}

// legacy is where an older hr kept the settings in the scratch HOME.
func legacy() string {
	return filepath.Join(os.Getenv("HOME"), ".hr", "config.toml")
}

func writeFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLoadSettings(t *testing.T) {
	home(t, `# who we are to servers
user_agent = "hrb/0.1"
autoread = true
colour = "green"
`)
	r, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if r.UserAgent != "hrb/0.1" || !r.AutoRead {
		t.Errorf("Load = %+v", r)
	}
	if !slices.Equal(r.Unknown, []string{"colour"}) {
		t.Errorf("Unknown = %q, want [colour]", r.Unknown)
	}
}

func TestLoadWithoutFile(t *testing.T) {
	home(t, "")
	r, err := Load()
	if err != nil || r.UserAgent != "" || len(r.Unknown) != 0 {
		t.Errorf("Load = %+v, %v; want zero settings", r, err)
	}
}

// The vault is the environment's to name, so a vault line is just an
// unknown key now; doctor and the no-vault error point at it.
func TestVaultLineIsUnknown(t *testing.T) {
	home(t, "vault = \"~/feed\"\n")
	r, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.Unknown, []string{"vault"}) {
		t.Errorf("Unknown = %q, want [vault]", r.Unknown)
	}
}

func TestLoadNeverWrites(t *testing.T) {
	p := home(t, "# mine\nvault = \"/v\"\n")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "# mine\nvault = \"/v\"\n" {
		t.Errorf("~/.hrrc = %q, changed by Load", got)
	}
}

func TestHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("HR_HOME", "")
	if got, err := Home(); err != nil || got != filepath.Join(dir, ".hr") {
		t.Errorf("Home() = %q, %v; want ~/.hr", got, err)
	}
	t.Setenv("HR_HOME", filepath.Join(dir, "elsewhere"))
	if got, _ := Home(); got != filepath.Join(dir, "elsewhere") {
		t.Errorf("Home() = %q, want $HR_HOME", got)
	}
}

// An older hr's ~/.hr/config.toml moves to ~/.hrrc on first use,
// settings, comments and all.
func TestLegacyConfigIsAdopted(t *testing.T) {
	p := home(t, "")
	body := "# mine\nuser_agent = \"hrb/0.1\"\n"
	writeFile(t, legacy(), body)
	r, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if r.UserAgent != "hrb/0.1" {
		t.Errorf("UserAgent = %q, want hrb/0.1", r.UserAgent)
	}
	if got := read(t, p); got != body {
		t.Errorf("~/.hrrc = %q, want the old config.toml", got)
	}
	if _, err := os.Stat(legacy()); !os.IsNotExist(err) {
		t.Errorf("config.toml still there: %v", err)
	}
}

// An existing ~/.hrrc wins, even a dangling link into dotfiles, and the
// stray config.toml is left for doctor; with $HR_HOME set, its
// config.toml never moves.
func TestLegacyConfigNeverOverwrites(t *testing.T) {
	p := home(t, "user_agent = \"new\"\n")
	writeFile(t, legacy(), "user_agent = \"old\"\n")
	if r, err := Load(); err != nil || r.UserAgent != "new" {
		t.Errorf("Load = %+v, %v; want new", r, err)
	}
	if read(t, legacy()) != "user_agent = \"old\"\n" {
		t.Error("config.toml changed")
	}

	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), p); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy()); err != nil {
		t.Errorf("config.toml moved over a dangling ~/.hrrc: %v", err)
	}

	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HR_HOME", filepath.Join(os.Getenv("HOME"), ".hr"))
	if r, err := Load(); err != nil || r.UserAgent != "" {
		t.Errorf("Load under HR_HOME = %+v, %v; want empty", r, err)
	}
	if _, err := os.Stat(legacy()); err != nil {
		t.Errorf("config.toml moved under HR_HOME: %v", err)
	}
}

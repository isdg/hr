package rc

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// home makes a scratch HOME whose ~/.hr/.hrrc holds body ("" = no
// file), with HR_HOME unset, and returns the .hrrc path.
func home(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("HR_HOME", "")
	p := filepath.Join(dir, ".hr", ".hrrc")
	if body != "" {
		writeFile(t, p, body)
	}
	return p
}

// userHrrc and configTOML are where older hr kept the settings.
func userHrrc() string {
	return filepath.Join(os.Getenv("HOME"), ".hrrc")
}

func configTOML() string {
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

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func TestLoadSettings(t *testing.T) {
	home(t, `vault = "~/feed"
# who we are to servers
user_agent = "hrb/0.1"
autoread = true
colour = "green"
`)
	r, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if r.Vault != "~/feed" || r.UserAgent != "hrb/0.1" || !r.AutoRead {
		t.Errorf("Load = %+v", r)
	}
	if !slices.Equal(r.Unknown, []string{"colour"}) {
		t.Errorf("Unknown = %q, want [colour]", r.Unknown)
	}
}

func TestLoadWithoutFile(t *testing.T) {
	home(t, "")
	r, err := Load()
	if err != nil || r.Vault != "" || len(r.Unknown) != 0 {
		t.Errorf("Load = %+v, %v; want zero settings", r, err)
	}
}

func TestLoadNeverWrites(t *testing.T) {
	p := home(t, "# mine\nvault = \"/v\"\n")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "# mine\nvault = \"/v\"\n" {
		t.Errorf(".hrrc = %q, changed by Load", got)
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

// $HR_HOME moves the settings with the state, so one variable isolates
// a scratch hr completely.
func TestHRHomeHoldsTheRC(t *testing.T) {
	home(t, "vault = \"/real\"\n")
	scratch := t.TempDir()
	t.Setenv("HR_HOME", scratch)
	if p, err := Path(); err != nil || p != filepath.Join(scratch, ".hrrc") {
		t.Errorf("Path() = %q, %v; want $HR_HOME/.hrrc", p, err)
	}
	if r, err := Load(); err != nil || r.Vault != "" {
		t.Errorf("Load = %+v, %v; read the real .hrrc", r, err)
	}
}

// ~/.hrrc, a regular file or a link into dotfiles, moves into ~/.hr.
func TestUserHrrcIsAdopted(t *testing.T) {
	p := home(t, "")
	body := "# mine\nvault = \"~/feed\"\n"
	writeFile(t, userHrrc(), body)
	r, err := Load()
	if err != nil || r.Vault != "~/feed" {
		t.Fatalf("Load = %+v, %v; want ~/feed", r, err)
	}
	if got := read(t, p); got != body || exists(userHrrc()) {
		t.Errorf(".hrrc = %q, ~/.hrrc left: %v", got, exists(userHrrc()))
	}

	p = home(t, "")
	target := filepath.Join(t.TempDir(), "hrrc")
	writeFile(t, target, body)
	if err := os.Symlink(target, userHrrc()); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.Readlink(p); err != nil || got != target {
		t.Errorf(".hrrc links to %q, %v; want %q", got, err, target)
	}
}

func TestConfigTOMLIsAdopted(t *testing.T) {
	p := home(t, "")
	writeFile(t, configTOML(), "vault = \"~/feed\"\n")
	if r, err := Load(); err != nil || r.Vault != "~/feed" {
		t.Fatalf("Load = %+v, %v; want ~/feed", r, err)
	}
	if !exists(p) || exists(configTOML()) {
		t.Error("config.toml was not moved to .hrrc")
	}
}

// With both legacy files, the newer ~/.hrrc wins and config.toml is
// left for doctor to report.
func TestNewerLegacyWins(t *testing.T) {
	home(t, "")
	writeFile(t, userHrrc(), "vault = \"/new\"\n")
	writeFile(t, configTOML(), "vault = \"/old\"\n")
	if r, err := Load(); err != nil || r.Vault != "/new" {
		t.Errorf("Load = %+v, %v; want /new", r, err)
	}
	if !exists(configTOML()) {
		t.Error("config.toml vanished")
	}
}

// An existing .hrrc, even a dangling link, is never replaced.
func TestLegacyNeverOverwrites(t *testing.T) {
	p := home(t, "vault = \"/kept\"\n")
	writeFile(t, userHrrc(), "vault = \"/old\"\n")
	if r, err := Load(); err != nil || r.Vault != "/kept" {
		t.Errorf("Load = %+v, %v; want /kept", r, err)
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
	if !exists(userHrrc()) {
		t.Error("~/.hrrc moved over a dangling .hrrc")
	}
}

// A scratch $HR_HOME adopts its own config.toml but never takes the
// user's ~/.hrrc.
func TestHRHomeNeverTakesUserHrrc(t *testing.T) {
	home(t, "")
	writeFile(t, userHrrc(), "vault = \"/user\"\n")
	scratch := t.TempDir()
	t.Setenv("HR_HOME", scratch)
	writeFile(t, filepath.Join(scratch, "config.toml"), "vault = \"/s\"\n")
	if r, err := Load(); err != nil || r.Vault != "/s" {
		t.Errorf("Load = %+v, %v; want /s", r, err)
	}
	if !exists(userHrrc()) {
		t.Error("~/.hrrc moved into $HR_HOME")
	}
}

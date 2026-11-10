package rc

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// home makes a scratch HOME whose ~/.hr/config.toml holds body ("" = no
// file), with HR_HOME unset, and returns the config path.
func home(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("HR_HOME", "")
	p := filepath.Join(dir, ".hr", "config.toml")
	if body != "" {
		writeFile(t, p, body)
	}
	return p
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

func TestSetVaultKeepsTheRest(t *testing.T) {
	p := home(t, `# hr user settings
vault = "/old"

# who we are to servers
user_agent = "hrb/0.1"
`)
	if err := SetVault("/new"); err != nil {
		t.Fatal(err)
	}
	want := `# hr user settings
vault = "/new"

# who we are to servers
user_agent = "hrb/0.1"
`
	if got := read(t, p); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	if err := SetVault(""); err != nil {
		t.Fatal(err)
	}
	want = `# hr user settings

# who we are to servers
user_agent = "hrb/0.1"
`
	if got := read(t, p); got != want {
		t.Errorf("after clear got\n%s\nwant\n%s", got, want)
	}
}

func TestSetVaultAdds(t *testing.T) {
	p := home(t, "")
	if err := SetVault("/v"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "vault = \"/v\"\n" {
		t.Errorf("new file = %q", got)
	}

	// Without a vault line, it goes in before any table, where TOML
	// still reads it as top-level.
	p = home(t, "autoread = true\n[extra]\nvault = \"/t\"\n")
	if err := SetVault("/v"); err != nil {
		t.Fatal(err)
	}
	want := "autoread = true\nvault = \"/v\"\n[extra]\nvault = \"/t\"\n"
	if got := read(t, p); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if r, err := Load(); err != nil || r.Vault != "/v" {
		t.Errorf("Load = %+v, %v", r, err)
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

// A ~/.hrrc from before ~/.hr moves into place on first use, settings,
// comments and all.
func TestLegacyHrrcIsAdopted(t *testing.T) {
	p := home(t, "")
	legacy := filepath.Join(os.Getenv("HOME"), ".hrrc")
	body := "# mine\nvault = \"~/feed\"\n"
	writeFile(t, legacy, body)
	r, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if r.Vault != "~/feed" {
		t.Errorf("Vault = %q, want ~/feed", r.Vault)
	}
	if got := read(t, p); got != body {
		t.Errorf("config.toml = %q, want the old ~/.hrrc", got)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("~/.hrrc still there: %v", err)
	}
}

// An existing config.toml wins and the stray ~/.hrrc is left alone;
// with $HR_HOME set, ~/.hrrc is never touched.
func TestLegacyHrrcNeverOverwrites(t *testing.T) {
	p := home(t, "vault = \"/new\"\n")
	legacy := filepath.Join(os.Getenv("HOME"), ".hrrc")
	writeFile(t, legacy, "vault = \"/old\"\n")
	if r, err := Load(); err != nil || r.Vault != "/new" {
		t.Errorf("Load = %+v, %v; want /new", r, err)
	}
	if read(t, legacy) != "vault = \"/old\"\n" {
		t.Error("~/.hrrc changed")
	}

	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HR_HOME", filepath.Join(t.TempDir(), "hr"))
	if r, err := Load(); err != nil || r.Vault != "" {
		t.Errorf("Load under HR_HOME = %+v, %v; want empty", r, err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Errorf("~/.hrrc moved under HR_HOME: %v", err)
	}
}

func TestSetVaultCreatesHome(t *testing.T) {
	p := home(t, "")
	if err := SetVault("/v"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "vault = \"/v\"\n" {
		t.Errorf("config.toml = %q", got)
	}
}

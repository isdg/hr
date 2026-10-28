package rc

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// home points ~/.hrrc at a temp file holding body ("" = no file).
func home(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	p := filepath.Join(dir, ".hrrc")
	if body != "" {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return p
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

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// doctor flags keys hr ignores and an older settings file left behind
// because .hrrc already exists.
func TestRCIssues(t *testing.T) {
	user := t.TempDir()
	t.Setenv("HOME", user)
	t.Setenv("HR_HOME", "")
	write(t, filepath.Join(user, ".hr", ".hrrc"),
		"vault = \"~/feed\"\ncolour = \"green\"\n")
	write(t, filepath.Join(user, ".hrrc"), "vault = \"/old\"\n")

	issues, err := rcIssues()
	if err != nil {
		t.Fatal(err)
	}
	var msgs []string
	for _, i := range issues {
		msgs = append(msgs, i.Path+": "+i.Message)
	}
	got := strings.Join(msgs, "\n")
	for _, want := range []string{`unknown key "colour"`,
		filepath.Join(user, ".hrrc") + ": no longer read"} {
		if !strings.Contains(got, want) {
			t.Errorf("issues:\n%s\nwant one with %q", got, want)
		}
	}
	if strings.Contains(got, `"vault"`) {
		t.Errorf("vault flagged as unknown:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(user, ".hrrc")); err != nil {
		t.Errorf("doctor moved or removed ~/.hrrc: %v", err)
	}
}

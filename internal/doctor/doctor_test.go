package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/isdg/hr/internal/config"
	"github.com/isdg/hr/internal/vault"
)

func TestCheckConfigUnknownKeys(t *testing.T) {
	cfg := &config.Config{
		Unknown: []string{"feeds.user_agent", "feeds.colour", "autocomit"},
	}
	var got []string
	checkConfig(cfg, func(sev Severity, _, format string, args ...any) {
		got = append(got, string(sev)+" "+fmt.Sprintf(format, args...))
	})
	want := []string{
		`warn "user_agent" is set inside a [[feeds]] entry, ` +
			"so it is ignored; move it above the first [[feeds]]",
		`warn unknown key "feeds.colour" is ignored`,
		`warn unknown key "autocomit" is ignored`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestCheckLegacyState(t *testing.T) {
	v := &vault.Vault{Root: t.TempDir(), State: t.TempDir()}
	var warned bool
	add := func(Severity, string, string, ...any) { warned = true }
	checkLegacyState(v, add)
	if warned {
		t.Error("warned with no legacy .hr")
	}
	if err := os.Mkdir(filepath.Join(v.Root, ".hr"), 0o755); err != nil {
		t.Fatal(err)
	}
	checkLegacyState(v, add)
	if !warned {
		t.Error("no warning for a leftover .hr in the vault")
	}
}

func TestCheckConfigArchiveNeedsManual(t *testing.T) {
	cfg := &config.Config{Feeds: []config.Feed{
		{Name: "done", Tags: []string{"manual", "archive"}},
		{Name: "polled", URL: "https://x/feed", Tags: []string{"archive"}},
	}}
	var got []string
	checkConfig(cfg, func(sev Severity, _, format string, args ...any) {
		got = append(got, string(sev)+" "+fmt.Sprintf(format, args...))
	})
	want := []string{
		`warn feed "polled" is tagged "archive" but not "manual", ` +
			"and only a hand-curated feed can be archived",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

package config

import (
	"slices"
	"testing"
)

func TestLoadUnknown(t *testing.T) {
	p := writeConfig(t, `name = "blog"
autocomit = true

[[feeds]]
url  = "https://example.com/feed"
name = "a"
ordering = "desc"
`)
	cfg, err := Load(p, Settings{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"autocomit", "feeds.ordering"}
	if !slices.Equal(cfg.Unknown, want) {
		t.Errorf("Unknown = %q, want %q", cfg.Unknown, want)
	}
	if cfg.Ordering != "" {
		t.Errorf("Ordering = %q: a key below [[feeds]] leaked", cfg.Ordering)
	}
}

func TestIsSetting(t *testing.T) {
	for key, want := range map[string]bool{
		"ordering":   true,
		"autocommit": true,
		"user_agent": true,
		"name":       false,
		"feeds":      false,
		"url":        false,
		"Unknown":    false,
	} {
		if got := IsSetting(key); got != want {
			t.Errorf("IsSetting(%q) = %v, want %v", key, got, want)
		}
	}
}

// A vault's hr.toml overrides the user's settings key by key: a key it
// sets wins even when false, and a key it leaves out keeps base's value.
func TestLoadOverBase(t *testing.T) {
	p := writeConfig(t, `name = "blog"
autoread = false
ordering = "asc"
`)
	base := Settings{
		AutoRead: true, Ordering: "desc", UserAgent: "hrb/0.1",
	}
	cfg, err := Load(p, base)
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{Ordering: "asc", UserAgent: "hrb/0.1"}
	if cfg.Settings != want {
		t.Errorf("Settings = %+v, want %+v", cfg.Settings, want)
	}
	if len(cfg.Unknown) > 0 {
		t.Errorf("Unknown = %q", cfg.Unknown)
	}
}

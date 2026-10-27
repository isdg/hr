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
	cfg, err := Load(p)
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
		"feeds":      false,
		"url":        false,
		"Unknown":    false,
		"-":          false,
	} {
		if got := IsSetting(key); got != want {
			t.Errorf("IsSetting(%q) = %v, want %v", key, got, want)
		}
	}
}

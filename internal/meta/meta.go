// Package meta reads, writes, and merges sidecar .meta.toml files.
package meta

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/isdg/hr/internal/textfmt"
)

type Meta struct {
	Read     bool       `toml:"read"`
	ReadAt   *time.Time `toml:"read_at,omitempty"`
	Favorite bool       `toml:"favorite"`
	Tags     []string   `toml:"tags,omitempty"`
	Alias    string     `toml:"alias,omitempty"`
}

func Path(articlePath string) string {
	return strings.TrimSuffix(articlePath, ".md") + ".meta.toml"
}

func Load(articlePath string) (*Meta, error) {
	path := Path(articlePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Meta
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &m, nil
}

func Save(articlePath string, m *Meta) error {
	path := Path(articlePath)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(m); err != nil {
		return fmt.Errorf("encode meta: %w", err)
	}
	return nil
}

// LoadOrDefault returns the sidecar at articlePath, or a zero-value
// Meta if it's missing or malformed.
func LoadOrDefault(articlePath string) *Meta {
	if m, err := Load(articlePath); err == nil {
		return m
	}
	return &Meta{}
}

// loadForUpdate returns the existing sidecar, or a default Meta if the
// sidecar is missing. Parse errors are surfaced (so we don't silently
// overwrite valid state with defaults).
func loadForUpdate(articlePath string) (*Meta, error) {
	m, err := Load(articlePath)
	if err == nil {
		return m, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return &Meta{}, nil
	}
	return nil, err
}

func ensureArticle(articlePath string) error {
	info, err := os.Stat(articlePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("article not found: %s", articlePath)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("not a file: %s", articlePath)
	}
	if !strings.HasSuffix(articlePath, ".md") {
		return fmt.Errorf("not a .md article: %s", articlePath)
	}
	return nil
}

func MarkRead(articlePath string) error {
	if err := ensureArticle(articlePath); err != nil {
		return err
	}
	m, err := loadForUpdate(articlePath)
	if err != nil {
		return err
	}
	m.Read = true
	now := time.Now().UTC()
	m.ReadAt = &now
	return Save(articlePath, m)
}

func MarkUnread(articlePath string) error {
	if err := ensureArticle(articlePath); err != nil {
		return err
	}
	m, err := loadForUpdate(articlePath)
	if err != nil {
		return err
	}
	m.Read = false
	m.ReadAt = nil
	return Save(articlePath, m)
}

// Fmt re-sanitizes the alias on a sidecar and rewrites if changed.
// Missing sidecars are no-ops. Returns (changed, error).
func Fmt(articlePath string) (bool, error) {
	m, err := Load(articlePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	orig := m.Alias
	m.Alias = textfmt.Line(m.Alias)
	if m.Alias == orig {
		return false, nil
	}
	return true, Save(articlePath, m)
}

// SetAlias sets the display alias on an article (or clears it if alias
// is empty/whitespace).
func SetAlias(articlePath, alias string) error {
	if err := ensureArticle(articlePath); err != nil {
		return err
	}
	m, err := loadForUpdate(articlePath)
	if err != nil {
		return err
	}
	m.Alias = textfmt.Line(alias)
	return Save(articlePath, m)
}

// ToggleFavorite flips the favorite bit and returns the new value.
func ToggleFavorite(articlePath string) (bool, error) {
	if err := ensureArticle(articlePath); err != nil {
		return false, err
	}
	m, err := loadForUpdate(articlePath)
	if err != nil {
		return false, err
	}
	m.Favorite = !m.Favorite
	return m.Favorite, Save(articlePath, m)
}

func WriteIfAbsent(articlePath string) (bool, error) {
	_, err := os.Stat(Path(articlePath))
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, Save(articlePath, &Meta{})
}

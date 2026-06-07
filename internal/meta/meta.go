// Package meta reads, writes, and merges sidecar .meta.toml files.
package meta

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Meta struct {
	Read     bool       `toml:"read"`
	ReadAt   *time.Time `toml:"read_at,omitempty"`
	Favorite bool       `toml:"favorite"`
	Tags     []string   `toml:"tags,omitempty"`
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

// Package rc locates hr's home, ~/.hr, and reads ~/.hrrc: the user's own
// defaults for every vault's settings. hr never writes ~/.hrrc, so it can
// be a link into a dotfiles repo.
package rc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/isdg/hr/internal/config"
)

type RC struct {
	config.Settings

	// Unknown lists the keys Load did not recognise, which are ignored.
	Unknown []string `toml:"-"`
}

// Home is hr's machine-local directory, outside every vault: $HR_HOME,
// else ~/.hr. It holds one state directory per vault.
func Home() (string, error) {
	if h := os.Getenv("HR_HOME"); h != "" {
		return filepath.Abs(h)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".hr"), nil
}

// Path returns ~/.hrrc, first moving an older hr's config.toml there.
// An explicit $HR_HOME never gives its config.toml up.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	path := filepath.Join(home, ".hrrc")
	if os.Getenv("HR_HOME") == "" {
		if err := adoptLegacy(path); err != nil {
			return "", err
		}
	}
	return path, nil
}

// LegacyPath is where hr kept these settings, and the default vault,
// before ~/.hrrc.
func LegacyPath() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config.toml"), nil
}

func adoptLegacy(path string) error {
	legacy, err := LegacyPath()
	if err != nil {
		return nil
	}
	if _, err := os.Stat(legacy); err != nil {
		return nil
	}
	if _, err := os.Lstat(path); err == nil {
		return nil
	}
	if err := os.Rename(legacy, path); err != nil {
		return fmt.Errorf("move %s to %s: %w", legacy, path, err)
	}
	return nil
}

func Load() (*RC, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &RC{}, nil
	}
	if err != nil {
		return nil, err
	}
	var r RC
	md, err := toml.Decode(string(data), &r)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, k := range md.Undecoded() {
		r.Unknown = append(r.Unknown, k.String())
	}
	return &r, nil
}

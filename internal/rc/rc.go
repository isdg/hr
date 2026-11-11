// Package rc locates hr's home, ~/.hr, and reads its .hrrc: the user's
// default vault and their defaults for every vault's settings. hr never
// writes .hrrc, so it can be a link into a dotfiles repo.
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
	Vault string `toml:"vault"`
	config.Settings

	// Unknown lists the keys Load did not recognise, which are ignored.
	Unknown []string `toml:"-"`
}

// Home is hr's machine-local directory, outside every vault: $HR_HOME,
// else ~/.hr. It holds .hrrc and one state directory per vault.
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

// Path returns <home>/.hrrc, first moving an older hr's settings file
// there when it has none.
func Path() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, ".hrrc")
	if err := adoptLegacy(path); err != nil {
		return "", err
	}
	return path, nil
}

// LegacyPaths are where older hr kept these settings, newest first:
// ~/.hrrc, then <home>/config.toml. ~/.hrrc is never taken into an
// explicit $HR_HOME, so a scratch home cannot claim the user's file.
func LegacyPaths() ([]string, error) {
	home, err := Home()
	if err != nil {
		return nil, err
	}
	paths := []string{filepath.Join(home, "config.toml")}
	if os.Getenv("HR_HOME") == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("locate home directory: %w", err)
		}
		paths = append([]string{filepath.Join(user, ".hrrc")}, paths...)
	}
	return paths, nil
}

func adoptLegacy(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return nil
	}
	legacy, err := LegacyPaths()
	if err != nil {
		return err
	}
	for _, old := range legacy {
		if _, err := os.Lstat(old); err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.Rename(old, path); err != nil {
			return fmt.Errorf("move %s to %s: %w", old, path, err)
		}
		return nil
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

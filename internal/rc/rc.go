// Package rc reads and writes ~/.hrbrc, the user's local pointer to
// the active hrb vault.
package rc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type RC struct {
	Vault string `toml:"vault"`
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".hrbrc"), nil
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
	if err := toml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &r, nil
}

func Save(r *RC) error {
	path, err := Path()
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(r); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

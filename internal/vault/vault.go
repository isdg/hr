// Package vault resolves vault paths, opens existing vaults, and
// initializes new ones on disk.
package vault

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed default.toml
var defaultConfig []byte

type Vault struct {
	Root string
}

// Resolve returns the absolute vault root from (in priority order):
// the passed-in path, $HRB_VAULT, or the default ~/blogs. The returned
// path is not guaranteed to exist.
func Resolve(path string) (string, error) {
	raw := path
	if raw == "" {
		raw = os.Getenv("HRB_VAULT")
	}
	if raw == "" {
		raw = "~/blogs"
	}
	expanded, err := expandTilde(raw)
	if err != nil {
		return "", err
	}
	return filepath.Abs(expanded)
}

func expandTilde(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, p[2:]), nil
}

func Open(root string) (*Vault, error) {
	v := &Vault{Root: root}
	if _, err := os.Stat(v.ConfigPath()); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"no hrb vault at %s (run `hrb init` first)", root)
		}
		return nil, err
	}
	return v, nil
}

func Init(root string) (*Vault, error) {
	v := &Vault{Root: root}

	if _, err := os.Stat(v.ConfigPath()); err == nil {
		return nil, fmt.Errorf("vault already initialized at %s", root)
	}

	dirs := []string{v.Root, v.FeedsDir(), v.MetaDir(), v.LogDir()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", d, err)
		}
	}

	err := os.WriteFile(v.ConfigPath(), defaultConfig, 0o644)
	if err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	gitignore := filepath.Join(v.Root, ".gitignore")
	err = os.WriteFile(gitignore, []byte(".hrb/\n"), 0o644)
	if err != nil {
		return nil, fmt.Errorf("write .gitignore: %w", err)
	}

	return v, nil
}

func (v *Vault) ConfigPath() string {
	return filepath.Join(v.Root, "hrb.toml")
}

func (v *Vault) FeedsDir() string {
	return filepath.Join(v.Root, "feeds")
}

func (v *Vault) MetaDir() string {
	return filepath.Join(v.Root, ".hrb")
}

func (v *Vault) CachePath() string {
	return filepath.Join(v.MetaDir(), "cache.json")
}

func (v *Vault) LogDir() string {
	return filepath.Join(v.MetaDir(), "log")
}

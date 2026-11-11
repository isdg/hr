package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/isdg/hr/internal/config"
	"github.com/isdg/hr/internal/rc"
	"github.com/isdg/hr/internal/vault"
)

func openActiveVault() (*vault.Vault, *config.Config, error) {
	root, err := vault.Resolve(vaultFlag)
	if err != nil {
		return nil, nil, err
	}
	v, err := vault.Open(root)
	if err != nil {
		return nil, nil, err
	}
	cfg, _, err := loadConfig(v)
	if err != nil {
		return nil, nil, err
	}
	return v, cfg, nil
}

// loadConfig reads the vault's hr.toml over the settings in ~/.hr/config.toml.
func loadConfig(v *vault.Vault) (*config.Config, *rc.RC, error) {
	r, err := rc.Load()
	if err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load(v.ConfigPath(), r.Settings)
	if err != nil {
		return nil, nil, err
	}
	return cfg, r, nil
}

// articleVault resolves the article at p and the vault holding it, found
// by walking up for hr.toml rather than from the flags, so an editor can
// name an article from any working directory.
func articleVault(p string) (*vault.Vault, string, error) {
	abs, err := filepath.Abs(p)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", p, err)
	}
	root, ok := vault.Discover(filepath.Dir(abs))
	if !ok {
		return nil, "", fmt.Errorf("%s: not in an hr vault", p)
	}
	v, err := vault.Open(root)
	if err != nil {
		return nil, "", err
	}
	feeds := v.FeedsDir() + string(filepath.Separator)
	if !strings.HasPrefix(abs, feeds) {
		return nil, "", fmt.Errorf("%s: not an article under %s",
			p, v.FeedsDir())
	}
	return v, abs, nil
}

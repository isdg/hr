package cmd

import (
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

// loadConfig reads the vault's hr.toml over the settings in ~/.hrrc.
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

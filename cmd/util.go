package cmd

import (
	"github.com/isdg/hr/internal/config"
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
	cfg, err := config.Load(v.ConfigPath())
	if err != nil {
		return nil, nil, err
	}
	return v, cfg, nil
}

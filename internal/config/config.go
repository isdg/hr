// Package config loads and saves the vault's hrb.toml.
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Name      string `toml:"name"`
	Feeds     []Feed `toml:"feeds"`
	AutoRead  bool   `toml:"autoread"`
	ShowRead  bool   `toml:"showread"`
	Ordering  string `toml:"ordering"`
	UserAgent string `toml:"user_agent"`
}

type Feed struct {
	URL  string   `toml:"url"`
	Name string   `toml:"name"`
	Tags []string `toml:"tags"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &c, nil
}

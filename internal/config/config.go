// Package config loads and saves the vault's hr.toml.
package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Name  string `toml:"name"`
	Feeds []Feed `toml:"feeds"`
	Settings

	// Unknown lists the dotted keys Load did not recognise, which are
	// otherwise ignored, e.g. "feeds.ordering".
	Unknown []string `toml:"-"`
}

// Settings are the keys a vault's hr.toml shares with the user's
// ~/.hr/config.toml. A key the vault sets wins; otherwise the user's
// value holds.
type Settings struct {
	AutoRead  bool   `toml:"autoread"`
	ShowRead  bool   `toml:"showread"`
	Ordering  string `toml:"ordering"`
	UserAgent string `toml:"user_agent"`

	// AutoCommit makes `hr sync` commit the files it writes, and the
	// sidecars changed since the last commit, to the git repository
	// holding the vault.
	AutoCommit bool `toml:"autocommit"`
}

// TagManual marks a hand-curated feed: one with nothing to poll, whose
// articles are written by hand (or by a build script) instead of fetched.
// `hr sync` reports such feeds as needing a manual update rather than
// trying to fetch them.
const TagManual = "manual"

type Feed struct {
	// URL is the feed to poll. Required unless the feed is tagged
	// TagManual, in which case there is nothing to poll.
	URL  string   `toml:"url"`
	Name string   `toml:"name"`
	Tags []string `toml:"tags"`

	// Group is the folder under feeds/ this feed belongs to, e.g.
	// "humans" or "sites/aggregators". It only decides where a feed
	// with no directory yet is created: once the directory exists, its
	// location on disk is authoritative and moving it (with `hr mv`,
	// `mv`, or `git mv`) is what regroups the feed.
	Group string `toml:"group"`
}

// IsManual reports whether the feed is hand-curated, i.e. carries the
// TagManual tag.
func (f Feed) IsManual() bool {
	for _, t := range f.Tags {
		if strings.EqualFold(strings.TrimSpace(t), TagManual) {
			return true
		}
	}
	return false
}

// Load reads the hr.toml at path over base, the user's settings: only
// the keys the file sets replace base's values.
func Load(path string, base Settings) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	c := Config{Settings: base}
	md, err := toml.Decode(string(data), &c)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	for _, k := range md.Undecoded() {
		c.Unknown = append(c.Unknown, k.String())
	}
	return &c, nil
}

// IsSetting reports whether key names one of the Settings, such as
// "ordering". Below a [[feeds]] header TOML gives such a key to that
// feed, which is the usual way one goes unrecognised.
func IsSetting(key string) bool {
	for _, f := range reflect.VisibleFields(reflect.TypeFor[Settings]()) {
		if name, _, _ := strings.Cut(f.Tag.Get("toml"), ","); name == key {
			return true
		}
	}
	return false
}

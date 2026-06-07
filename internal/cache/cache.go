// Package cache persists per-feed sync state (ETag, Last-Modified,
// last fetch time) to .hrb/cache.json. It is gitignored.
package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
	FetchedAt    time.Time `json:"fetched_at"`
}

type Cache struct {
	path    string
	Entries map[string]Entry `json:"entries"`
}

func Load(path string) (*Cache, error) {
	c := &Cache{path: path, Entries: map[string]Entry{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, err
	}
	if c.Entries == nil {
		c.Entries = map[string]Entry{}
	}
	return c, nil
}

func (c *Cache) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o644)
}

func (c *Cache) Get(feed string) Entry { return c.Entries[feed] }

func (c *Cache) Set(feed string, e Entry) { c.Entries[feed] = e }

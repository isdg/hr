// Package tombstone implements sync-safe article deletion. Deleting an
// article removes its files and leaves a small <id>.deleted marker in the
// feed directory; sync consults these markers and skips the matching feed
// items, so a deleted article is never re-fetched. Markers are committed
// with the vault, so deletions propagate across machines.
package tombstone

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/isdg/hr/internal/article"
	"github.com/isdg/hr/internal/meta"
	"github.com/isdg/hr/internal/vault"
)

const markerExt = ".deleted"

// Marker is the content of an <id>.deleted tombstone file.
type Marker struct {
	ID        string    `toml:"id"`
	Feed      string    `toml:"feed"`
	Title     string    `toml:"title,omitempty"`
	DeletedAt time.Time `toml:"deleted_at"`
}

func markerPath(feedDir, id string) string {
	return filepath.Join(feedDir, id+markerExt)
}

// Delete removes the article's .md, sidecar, and raw HTML, then writes a
// tombstone marker so sync skips its id. Returns the marker path.
func Delete(v *vault.Vault, path string) (string, error) {
	id, feedDir, err := locate(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	feed := filepath.Base(feedDir)
	title := ""
	if fm, _, err := article.ReadFile(path); err == nil {
		title = fm.Title
	}

	mp := markerPath(feedDir, id)
	if err := writeMarker(mp, Marker{
		ID: id, Feed: feed, Title: title, DeletedAt: time.Now().UTC(),
	}); err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	_ = os.Remove(meta.Path(path))
	removeRaw(v, feed, filepath.Base(path))
	return mp, nil
}

// Restore lifts the tombstone for an article so sync will re-fetch it.
// path may be the original .md path or the .deleted marker path. Returns
// whether a marker was removed.
func Restore(path string) (bool, error) {
	id, feedDir, err := locate(path)
	if err != nil {
		return false, err
	}
	err = os.Remove(markerPath(feedDir, id))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// Purge hard-deletes the article (files + any tombstone) without leaving
// a marker. It is NOT sync-safe: a feed still listing the item will
// re-fetch it. Intended for non-synced vaults.
func Purge(v *vault.Vault, path string) error {
	id, feedDir, err := locate(path)
	if err != nil {
		return err
	}
	feed := filepath.Base(feedDir)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = os.Remove(meta.Path(path))
	_ = os.Remove(markerPath(feedDir, id))
	removeRaw(v, feed, filepath.Base(path))
	return nil
}

// DeletedIDs returns the set of tombstoned article ids in feedDir.
func DeletedIDs(feedDir string) (map[string]bool, error) {
	hits, err := filepath.Glob(filepath.Join(feedDir, "*"+markerExt))
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(hits))
	for _, h := range hits {
		set[strings.TrimSuffix(filepath.Base(h), markerExt)] = true
	}
	return set, nil
}

// locate derives the stable id and feed directory from an article path
// (.md) or a marker path (.deleted).
func locate(path string) (id, feedDir string, err error) {
	feedDir = filepath.Dir(path)
	base := filepath.Base(path)
	if mid, ok := strings.CutSuffix(base, markerExt); ok {
		return mid, feedDir, nil
	}
	if !strings.HasSuffix(base, ".md") {
		return "", "", fmt.Errorf("not a .md article or %s marker: %s", markerExt, path)
	}
	id, ok := article.IDFromName(base)
	if !ok {
		return "", "", fmt.Errorf("cannot derive id from filename: %s", path)
	}
	return id, feedDir, nil
}

func writeMarker(path string, m Marker) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(m)
}

func removeRaw(v *vault.Vault, feed, mdBase string) {
	if v == nil || feed == "" {
		return
	}
	_ = os.Remove(v.RawPath(feed, mdBase))
}

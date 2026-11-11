// Package history keeps the append-only log of articles opened for
// reading, the source of `hr recent`.
package history

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/isdg/hr/internal/article"
	"github.com/isdg/hr/internal/jsonl"
)

// Entry is one recorded open. Articles are keyed by feed and id rather
// than path, so moving a feed between groups keeps its history.
type Entry struct {
	At   time.Time `json:"at"`
	Feed string    `json:"feed"`
	ID   string    `json:"id"`
}

type Key struct{ Feed, ID string }

func (e Entry) Key() Key { return Key{Feed: e.Feed, ID: e.ID} }

// KeyFor derives the key of the article at path in feed: its stable id
// when the name is canonical, else the file name, unique in its feed.
func KeyFor(feed, path string) Key {
	base := filepath.Base(path)
	id, ok := article.IDFromName(base)
	if !ok {
		id = strings.TrimSuffix(base, ".md")
	}
	return Key{Feed: feed, ID: id}
}

// Append adds e as one line of the log at path.
func Append(path string, e Entry) error { return jsonl.Append(path, e) }

// Load returns every well-formed entry in file order. A missing log is
// empty history; malformed lines are skipped rather than fatal.
func Load(path string) ([]Entry, error) {
	return jsonl.Load(path, func(e Entry) bool {
		return e.Feed != "" && e.ID != "" && !e.At.IsZero()
	})
}

// Latest maps each article to its most recent open.
func Latest(entries []Entry) map[Key]time.Time {
	out := make(map[Key]time.Time, len(entries))
	for _, e := range entries {
		if at, ok := out[e.Key()]; !ok || e.At.After(at) {
			out[e.Key()] = e.At
		}
	}
	return out
}

// KeyOf derives the key of the article at articlePath from the feed
// its front matter names, as listings key it.
func KeyOf(articlePath string) (Key, error) {
	if !strings.HasSuffix(articlePath, ".md") {
		return Key{}, fmt.Errorf("not a .md article: %s", articlePath)
	}
	fm, err := article.ParseFile(articlePath)
	if err != nil {
		return Key{}, err
	}
	if fm.Feed == "" {
		return Key{}, fmt.Errorf("%s: front matter names no feed",
			articlePath)
	}
	return KeyFor(fm.Feed, articlePath), nil
}

// Record logs an open of the article at articlePath.
func Record(logPath, articlePath string, at time.Time) error {
	k, err := KeyOf(articlePath)
	if err != nil {
		return err
	}
	return Append(logPath, Entry{At: at.UTC(), Feed: k.Feed, ID: k.ID})
}

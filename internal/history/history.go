// Package history keeps the append-only log of articles opened for
// reading, the source of `hr recent`.
package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/isdg/hr/internal/article"
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

// Append adds e as one line. O_APPEND with a single write keeps lines
// whole when two editors record opens at once.
func Append(path string, e Entry) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Load returns every well-formed entry in file order. A missing log is
// empty history; malformed lines are skipped rather than fatal.
func Load(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Feed == "" ||
			e.ID == "" || e.At.IsZero() {
			continue
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
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

// Record logs an open of the article at articlePath, keyed by the feed
// its front matter names, as listings key it.
func Record(logPath, articlePath string, at time.Time) error {
	if !strings.HasSuffix(articlePath, ".md") {
		return fmt.Errorf("not a .md article: %s", articlePath)
	}
	fm, err := article.ParseFile(articlePath)
	if err != nil {
		return err
	}
	if fm.Feed == "" {
		return fmt.Errorf("%s: front matter names no feed", articlePath)
	}
	k := KeyFor(fm.Feed, articlePath)
	return Append(logPath, Entry{At: at.UTC(), Feed: k.Feed, ID: k.ID})
}

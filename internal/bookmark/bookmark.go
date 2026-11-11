// Package bookmark keeps the append-only log of reading places: where in
// an article you stopped, so an editor can take you back there.
package bookmark

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/isdg/hr/internal/history"
	"github.com/isdg/hr/internal/jsonl"
)

// Position is a place in an article: a 1-based line and a 0-based byte
// column, the convention corruption marks use.
type Position struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

// ParsePosition reads "LINE" or "LINE:COL".
func ParsePosition(s string) (Position, error) {
	ls, cs, hasCol := strings.Cut(s, ":")
	line, err := strconv.Atoi(ls)
	if err != nil || line < 1 {
		return Position{}, fmt.Errorf("bad line in %q: want >= 1", s)
	}
	p := Position{Line: line}
	if hasCol {
		col, err := strconv.Atoi(cs)
		if err != nil || col < 0 {
			return Position{}, fmt.Errorf("bad column in %q: want >= 0", s)
		}
		p.Col = col
	}
	return p, nil
}

// Entry is one change to an article's bookmark, keyed like history so a
// feed can move between groups. A zero Line clears the bookmark.
type Entry struct {
	At   time.Time `json:"at"`
	Feed string    `json:"feed"`
	ID   string    `json:"id"`
	Position
}

func (e Entry) Key() history.Key {
	return history.Key{Feed: e.Feed, ID: e.ID}
}

// Load returns every well-formed entry in file order. A missing log has
// no bookmarks; malformed lines are skipped rather than fatal.
func Load(path string) ([]Entry, error) {
	return jsonl.Load(path, func(e Entry) bool {
		return e.Feed != "" && e.ID != "" && !e.At.IsZero() &&
			e.Line >= 0 && e.Col >= 0
	})
}

// Current maps each bookmarked article to its latest position. The
// newest entry wins, the later line on a tie, and a clear removes it.
func Current(entries []Entry) map[history.Key]Position {
	out := make(map[history.Key]Position)
	for _, e := range Newest(entries) {
		out[e.Key()] = e.Position
	}
	return out
}

// Newest returns each bookmarked article's winning entry (see Current),
// most recently set first, the later line first on a tie.
func Newest(entries []Entry) []Entry {
	win := make(map[history.Key]int, len(entries))
	for i, e := range entries {
		if j, ok := win[e.Key()]; !ok || !e.At.Before(entries[j].At) {
			win[e.Key()] = i
		}
	}
	idx := make([]int, 0, len(win))
	for _, i := range win {
		if entries[i].Line > 0 {
			idx = append(idx, i)
		}
	}
	slices.SortFunc(idx, func(a, b int) int {
		if c := entries[b].At.Compare(entries[a].At); c != 0 {
			return c
		}
		return b - a
	})
	out := make([]Entry, len(idx))
	for n, i := range idx {
		out[n] = entries[i]
	}
	return out
}

// Set logs p as the reading place in the article at articlePath. The
// position must exist in the file: a column may sit at the line's end.
func Set(logPath, articlePath string, p Position, at time.Time) error {
	k, err := history.KeyOf(articlePath)
	if err != nil {
		return err
	}
	if err := check(articlePath, p); err != nil {
		return err
	}
	return jsonl.Append(logPath,
		Entry{At: at.UTC(), Feed: k.Feed, ID: k.ID, Position: p})
}

// Clear logs that the article at articlePath has no reading place.
func Clear(logPath, articlePath string, at time.Time) error {
	k, err := history.KeyOf(articlePath)
	if err != nil {
		return err
	}
	return jsonl.Append(logPath, Entry{At: at.UTC(), Feed: k.Feed, ID: k.ID})
}

// Get returns the article's current reading place, if it has one.
func Get(logPath, articlePath string) (Position, bool, error) {
	k, err := history.KeyOf(articlePath)
	if err != nil {
		return Position{}, false, err
	}
	entries, err := Load(logPath)
	if err != nil {
		return Position{}, false, err
	}
	p, ok := Current(entries)[k]
	return p, ok, nil
}

func check(path string, p Position) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	if p.Line > len(lines) {
		return fmt.Errorf("%s: line %d past the end (%d lines)",
			path, p.Line, len(lines))
	}
	if n := len(lines[p.Line-1]); p.Col > n {
		return fmt.Errorf("%s: column %d past the end of line %d (%d bytes)",
			path, p.Col, p.Line, n)
	}
	return nil
}

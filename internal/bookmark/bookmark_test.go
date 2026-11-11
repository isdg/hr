package bookmark

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/isdg/hr/internal/history"
)

var t0 = time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)

func article(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "2024-01-01-x-3f9a1c2e.md")
	body := "---\ntitle: \"x\"\nfeed: \"matklad\"\n---\n\nбукв\nend\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParsePosition(t *testing.T) {
	for in, want := range map[string]Position{
		"7": {Line: 7}, "7:0": {Line: 7}, "12:5": {Line: 12, Col: 5},
	} {
		if got, err := ParsePosition(in); err != nil || got != want {
			t.Errorf("ParsePosition(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "0", "-1", "x", "3:", "3:-1", "3:x"} {
		if _, err := ParsePosition(in); err == nil {
			t.Errorf("ParsePosition(%q) succeeded, want an error", in)
		}
	}
}

func TestSetGetClear(t *testing.T) {
	md := article(t)
	log := filepath.Join(t.TempDir(), "bookmarks.jsonl")
	if _, ok, err := Get(log, md); ok || err != nil {
		t.Fatalf("Get(empty log) = %v, %v; want none", ok, err)
	}
	if err := Set(log, md, Position{Line: 6, Col: 2}, t0); err != nil {
		t.Fatal(err)
	}
	// Line 6 is "букв", 8 bytes: its end is a valid place, past it is not.
	end := Position{Line: 6, Col: 8}
	if err := Set(log, md, end, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if p, ok, err := Get(log, md); !ok || err != nil || p != (Position{6, 8}) {
		t.Errorf("Get = %v, %v, %v; want 6:8", p, ok, err)
	}
	if err := Clear(log, md, t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := Get(log, md); ok {
		t.Error("bookmark survived Clear")
	}
	// The log keeps every change, clears included.
	if all, _ := Load(log); len(all) != 3 {
		t.Errorf("log holds %d entries, want 3", len(all))
	}
}

func TestSetRejectsMissingPlace(t *testing.T) {
	md := article(t)
	log := filepath.Join(t.TempDir(), "bookmarks.jsonl")
	for _, p := range []Position{{Line: 8}, {Line: 6, Col: 9}} {
		if err := Set(log, md, p, t0); err == nil {
			t.Errorf("Set(%v) succeeded, want an error", p)
		}
	}
	notes := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(notes, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Set(log, notes, Position{Line: 1}, t0); err == nil {
		t.Error("Set on a non-article succeeded, want an error")
	}
	if all, _ := Load(log); len(all) != 0 {
		t.Errorf("rejected sets logged %d entries", len(all))
	}
}

// The newest entry wins whatever the file order, a tie goes to the later
// line, and a clear drops the article.
func TestCurrent(t *testing.T) {
	a := history.Key{Feed: "f", ID: "a"}
	b := history.Key{Feed: "f", ID: "b"}
	c := history.Key{Feed: "f", ID: "c"}
	e := func(k history.Key, at time.Time, line int) Entry {
		return Entry{At: at, Feed: k.Feed, ID: k.ID,
			Position: Position{Line: line}}
	}
	got := Current([]Entry{
		e(a, t0.Add(time.Hour), 5),
		e(a, t0, 1), // older, appended later by a skewed clock
		e(b, t0, 2),
		e(b, t0, 3),
		e(c, t0, 4),
		e(c, t0.Add(time.Minute), 0),
	})
	want := map[history.Key]Position{a: {Line: 5}, b: {Line: 3}}
	if len(got) != len(want) {
		t.Fatalf("Current = %v, want %v", got, want)
	}
	for k, p := range want {
		if got[k] != p {
			t.Errorf("Current[%v] = %v, want %v", k, got[k], p)
		}
	}
}

// Newest orders the live bookmarks by when they were set, newest first,
// leaving out cleared ones and superseded positions.
func TestNewest(t *testing.T) {
	e := func(id string, at time.Time, line int) Entry {
		return Entry{At: at, Feed: "f", ID: id,
			Position: Position{Line: line}}
	}
	got := Newest([]Entry{
		e("a", t0, 1),
		e("b", t0.Add(time.Minute), 2),
		e("c", t0.Add(2*time.Minute), 3),
		e("a", t0.Add(3*time.Minute), 4),
		e("c", t0.Add(4*time.Minute), 0),
		e("d", t0.Add(time.Minute), 5), // ties b, later in the log
	})
	var ids []string
	for _, g := range got {
		ids = append(ids, fmt.Sprintf("%s:%d", g.ID, g.Line))
	}
	want := []string{"a:4", "d:5", "b:2"}
	if !slices.Equal(ids, want) {
		t.Errorf("Newest = %v, want %v", ids, want)
	}
}

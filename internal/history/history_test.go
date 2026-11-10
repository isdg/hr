package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)

func TestAppendLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".hr", "history.jsonl")
	want := []Entry{
		{At: t0, Feed: "matklad", ID: "3f9a1c2e"},
		{At: t0.Add(time.Minute), Feed: "ritchie", ID: "1972-01-01-notes"},
	}
	for _, e := range want {
		if err := Append(path, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if !got[i].At.Equal(want[i].At) || got[i].Key() != want[i].Key() {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// No history yet is the normal first-run state, not an error.
func TestLoadMissing(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "history.jsonl"))
	if err != nil || len(got) != 0 {
		t.Errorf("Load(missing) = %v, %v; want no entries, no error", got, err)
	}
}

// A torn last line (crash mid-append) or a hand-mangled one must not
// cost the rest of the history.
func TestLoadSkipsBadLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	data := `{"at":"2026-11-10T14:00:00Z","feed":"a","id":"11111111"}
not json
{"at":"2026-11-10T14:01:00Z","feed":"","id":"22222222"}
{"at":"2026-11-10T14:02:00Z","feed":"b","id":""}

{"at":"2026-11-10T14:03:00Z","feed":"c","id":"33333333"}
{"at":"2026-11-10T14:04:00Z","feed":"d","id":"4444`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Feed != "a" || got[1].Feed != "c" {
		t.Errorf("got %+v, want only the a and c entries", got)
	}
}

// Latest keeps the newest open per article, whatever the line order.
func TestLatest(t *testing.T) {
	a := Key{Feed: "f", ID: "aaaaaaaa"}
	b := Key{Feed: "f", ID: "bbbbbbbb"}
	other := Key{Feed: "g", ID: "aaaaaaaa"} // same id, other feed
	got := Latest([]Entry{
		{At: t0, Feed: a.Feed, ID: a.ID},
		{At: t0.Add(2 * time.Hour), Feed: a.Feed, ID: a.ID},
		{At: t0.Add(time.Hour), Feed: a.Feed, ID: a.ID},
		{At: t0.Add(time.Minute), Feed: b.Feed, ID: b.ID},
		{At: t0.Add(3 * time.Hour), Feed: other.Feed, ID: other.ID},
	})
	want := map[Key]time.Time{
		a:     t0.Add(2 * time.Hour),
		b:     t0.Add(time.Minute),
		other: t0.Add(3 * time.Hour),
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, w := range want {
		if !got[k].Equal(w) {
			t.Errorf("Latest[%v] = %v, want %v", k, got[k], w)
		}
	}
}

func TestKeyFor(t *testing.T) {
	for _, tc := range []struct{ path, id string }{
		{"/v/feeds/humans/matklad/2024-01-01-x-3f9a1c2e.md", "3f9a1c2e"},
		// Hand-built feeds may lack the id suffix; the file name is
		// unique within its feed, so it stands in.
		{"/v/feeds/humans/archive/ritchie/1972-01-01-notes.md",
			"1972-01-01-notes"},
	} {
		if got := KeyFor("f", tc.path); got != (Key{"f", tc.id}) {
			t.Errorf("KeyFor(%q) = %+v, want id %q", tc.path, got, tc.id)
		}
	}
}

func TestRecord(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, ".hr", "history.jsonl")
	md := filepath.Join(dir, "2024-01-01-x-3f9a1c2e.md")
	body := "---\ntitle: \"x\"\nfeed: \"matklad\"\n---\n\nbody\n"
	if err := os.WriteFile(md, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 10, 15, 0, 0, 0, time.FixedZone("", 3600))
	if err := Record(log, md, at); err != nil {
		t.Fatal(err)
	}
	got, err := Load(log)
	if err != nil {
		t.Fatal(err)
	}
	want := Entry{At: at, Feed: "matklad", ID: "3f9a1c2e"}
	if len(got) != 1 || got[0].Key() != want.Key() || !got[0].At.Equal(at) {
		t.Fatalf("logged %+v, want %+v", got, want)
	}
	if got[0].At.Location() != time.UTC {
		t.Errorf("logged in %v, want UTC", got[0].At.Location())
	}

	for _, bad := range []string{
		filepath.Join(dir, "missing-3f9a1c2e.md"),
		filepath.Join(dir, "notes.txt"),
	} {
		if err := Record(log, bad, at); err == nil {
			t.Errorf("Record(%s) succeeded, want an error", bad)
		}
	}
	nofeed := filepath.Join(dir, "nofeed-aaaaaaaa.md")
	if err := os.WriteFile(nofeed, []byte("---\ntitle: \"y\"\n---\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	if err := Record(log, nofeed, at); err == nil {
		t.Error("Record(no feed) succeeded, want an error")
	}
	if got, _ := Load(log); len(got) != 1 {
		t.Errorf("failed records wrote to the log: %+v", got)
	}
}

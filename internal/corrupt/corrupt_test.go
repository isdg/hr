package corrupt

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/isdg/hr/internal/meta"
)

// "# Poetics — Part I" — the em dash (—, U+2014) is 3 bytes at offset 9.
const emLine = "# Poetics — Part I"

func TestExtractSingleLine(t *testing.T) {
	got := extract([]string{emLine}, Range{1, 0, 1, 9})
	if got != "# Poetics" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractSnapsRuneBoundary(t *testing.T) {
	// Byte 12 lands inside the 3-byte em dash; must snap back to 9 and
	// never produce invalid UTF-8.
	got := extract([]string{emLine}, Range{1, 0, 1, 12})
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8 produced: %q", got)
	}
	if got != "# Poetics " {
		t.Fatalf("got %q, want snapped to %q", got, "# Poetics ")
	}
}

func TestExtractMultiLine(t *testing.T) {
	got := extract([]string{emLine, "Part I"}, Range{1, 4, 2, 4})
	want := "etics — Part I\nPart"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		r  Range
		ok bool
	}{
		{Range{1, 0, 1, 5}, true},
		{Range{0, 0, 1, 5}, false},  // line < 1
		{Range{1, 0, 9, 5}, false},  // past EOF (2 lines)
		{Range{2, 0, 1, 5}, false},  // end before start
		{Range{1, 5, 1, 2}, false},  // end col before start col
		{Range{1, -1, 1, 5}, false}, // negative col
	}
	for _, c := range cases {
		err := c.r.validate(2)
		if (err == nil) != c.ok {
			t.Errorf("validate(%+v): err=%v, want ok=%v", c.r, err, c.ok)
		}
	}
}

func TestMarkExpectMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	// line 5 "The quick brwn fox." — "brwn" is bytes 10-14.
	if err := os.WriteFile(path,
		[]byte("---\nx\n---\n\nThe quick brwn fox.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Range{5, 10, 5, 14}
	if _, err := Mark(path, r, MarkOptions{Expect: "quick", ContextLines: -1}); err == nil {
		t.Fatal("expected selection-mismatch error")
	}
	if m := meta.LoadOrDefault(path); len(m.Corruptions) != 0 {
		t.Fatalf("mark persisted despite mismatch: %d", len(m.Corruptions))
	}
	if _, err := Mark(path, r, MarkOptions{Expect: "brwn", ContextLines: -1}); err != nil {
		t.Fatalf("matching expect should succeed: %v", err)
	}
}

func TestMarkRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte("---\nx\n---\n\n"+emLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Mark(path, Range{5, 0, 5, 12}, MarkOptions{Note: "garble", ContextLines: -1})
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(c.Quote) {
		t.Fatalf("invalid UTF-8 quote: %q", c.Quote)
	}
	// Re-mark identical region: upsert, not duplicate.
	if _, err := Mark(path, Range{5, 0, 5, 12}, MarkOptions{Note: "again", ContextLines: -1}); err != nil {
		t.Fatal(err)
	}
	m := meta.LoadOrDefault(path)
	if len(m.Corruptions) != 1 {
		t.Fatalf("want 1 corruption after re-mark, got %d", len(m.Corruptions))
	}
	if m.Corruptions[0].ID != c.ID {
		t.Fatalf("id changed: %q vs %q", m.Corruptions[0].ID, c.ID)
	}
}

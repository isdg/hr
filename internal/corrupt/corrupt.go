// Package corrupt marks regions of an article as corrupted and reports
// them across a vault, so an LLM (or other tooling) can later restore
// the original text. Marks live in each article's .meta.toml sidecar.
package corrupt

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/isdg/hr/internal/article"
	"github.com/isdg/hr/internal/meta"
	"github.com/isdg/hr/internal/textfmt"
	"github.com/isdg/hr/internal/vault"
)

// ErrDrift means the article text at a mark's recorded location no
// longer matches its captured quote, so the location can't be trusted
// for an automatic edit.
var ErrDrift = errors.New(
	"article text no longer matches the mark's quote (drift)")

// DefaultContextLines is how many lines of surrounding text are captured
// on each side of a marked region when the caller doesn't specify.
const DefaultContextLines = 2

// Range is a selection within an article: 1-based lines, 0-based byte
// columns, with EndCol exclusive.
type Range struct {
	StartLine, StartCol int
	EndLine, EndCol     int
}

// Record is an article together with its corruption marks, used for
// vault-wide reporting.
type Record struct {
	Path        string            `json:"path"`
	Feed        string            `json:"feed"`
	Title       string            `json:"title"`
	Corruptions []meta.Corruption `json:"corruptions"`
}

// MarkOptions configures Mark.
type MarkOptions struct {
	Note         string
	ContextLines int    // surrounding lines per side; <0 means default
	Expect       string // if non-empty, the extracted text must match it
}

// Mark extracts the text at r from the article and records a corruption
// on its sidecar, returning the stored entry. Re-marking the identical
// region updates the existing entry rather than duplicating it.
//
// If opts.Expect is set (e.g. the selection piped from the editor) and
// the extracted text doesn't match it, Mark fails without writing —
// catching a wrong range before it's persisted.
func Mark(articlePath string, r Range, opts MarkOptions) (meta.Corruption, error) {
	ctxLines := opts.ContextLines
	if ctxLines < 0 {
		ctxLines = DefaultContextLines
	}
	data, err := os.ReadFile(articlePath)
	if err != nil {
		return meta.Corruption{}, err
	}
	lines := strings.Split(string(data), "\n")
	if err := r.validate(len(lines)); err != nil {
		return meta.Corruption{}, err
	}

	quote := extract(lines, r)
	if want := strings.TrimRight(opts.Expect, "\n"); want != "" &&
		want != strings.TrimRight(quote, "\n") {
		return meta.Corruption{}, fmt.Errorf(
			"selection mismatch: range extracts %q, stdin has %q "+
				"(range likely wrong; pass --force to mark anyway)",
			quote, want)
	}
	c := meta.Corruption{
		ID:        id(r, quote),
		StartLine: r.StartLine,
		StartCol:  r.StartCol,
		EndLine:   r.EndLine,
		EndCol:    r.EndCol,
		Quote:     quote,
		Context:   context(lines, r, ctxLines),
		Note:      textfmt.Line(opts.Note),
		CreatedAt: time.Now().UTC(),
	}
	if err := meta.AddCorruption(articlePath, c); err != nil {
		return meta.Corruption{}, err
	}
	return c, nil
}

// Remove deletes a corruption mark by id.
func Remove(articlePath, id string) (bool, error) {
	return meta.RemoveCorruption(articlePath, id)
}

// Restore replaces the region of the mark id with replacement, then
// clears the mark. It refuses with ErrDrift if the current text at the
// recorded range no longer matches the stored quote, unless force.
func Restore(articlePath, id, replacement string, force bool) error {
	m := meta.LoadOrDefault(articlePath)
	c, ok := find(m.Corruptions, id)
	if !ok {
		return fmt.Errorf("no corruption with id %q", id)
	}
	data, err := os.ReadFile(articlePath)
	if err != nil {
		return err
	}
	text := string(data)
	lines := strings.Split(text, "\n")
	r := rangeOf(c)
	if err := r.validate(len(lines)); err != nil {
		return fmt.Errorf("mark range invalid: %w", err)
	}
	start, end := span(lines, r)
	if !force && text[start:end] != c.Quote {
		return ErrDrift
	}
	out := text[:start] + replacement + text[end:]
	if err := os.WriteFile(articlePath, []byte(out), 0o644); err != nil {
		return err
	}
	_, err = meta.RemoveCorruption(articlePath, id)
	return err
}

func find(cs []meta.Corruption, id string) (meta.Corruption, bool) {
	for _, c := range cs {
		if c.ID == id {
			return c, true
		}
	}
	return meta.Corruption{}, false
}

func rangeOf(c meta.Corruption) Range {
	return Range{c.StartLine, c.StartCol, c.EndLine, c.EndCol}
}

// span returns the absolute byte offsets [start,end) in the file for r.
func span(lines []string, r Range) (int, int) {
	starts := make([]int, len(lines))
	off := 0
	for i, l := range lines {
		starts[i] = off
		off += len(l) + 1 // +1 for the joining "\n"
	}
	start := starts[r.StartLine-1] + colByte(lines[r.StartLine-1], r.StartCol)
	end := starts[r.EndLine-1] + colByte(lines[r.EndLine-1], r.EndCol)
	return start, end
}

// List returns the corruption record for a single article (empty
// Corruptions if none). Each corruption's Stale flag is computed against
// the current article text.
func List(articlePath string) (Record, error) {
	m := meta.LoadOrDefault(articlePath)
	rec := Record{Path: articlePath}
	if fm, err := article.ParseFile(articlePath); err == nil {
		rec.Feed = textfmt.Line(fm.Feed)
		rec.Title = textfmt.Line(fm.Title)
	}
	if len(m.Corruptions) == 0 {
		return rec, nil
	}
	cs := make([]meta.Corruption, len(m.Corruptions))
	copy(cs, m.Corruptions)
	if data, err := os.ReadFile(articlePath); err == nil {
		text := string(data)
		lines := strings.Split(text, "\n")
		for i := range cs {
			cs[i].Stale = isStale(text, lines, cs[i])
		}
	}
	rec.Corruptions = cs
	return rec, nil
}

// isStale reports whether the text at c's recorded range no longer
// matches its quote (e.g. after the article was reformatted).
func isStale(text string, lines []string, c meta.Corruption) bool {
	r := rangeOf(c)
	if r.validate(len(lines)) != nil {
		return true
	}
	start, end := span(lines, r)
	return text[start:end] != c.Quote
}

// ListAll walks the vault and returns one Record per article that has at
// least one corruption mark.
func ListAll(v *vault.Vault) ([]Record, error) {
	var recs []Record
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		m := meta.LoadOrDefault(path)
		if len(m.Corruptions) == 0 {
			return nil
		}
		rec, _ := List(path)
		recs = append(recs, rec)
		return nil
	}
	if err := filepath.WalkDir(v.FeedsDir(), walk); err != nil {
		return nil, err
	}
	return recs, nil
}

func (r Range) validate(n int) error {
	if r.StartLine < 1 || r.EndLine < 1 {
		return fmt.Errorf("line numbers are 1-based")
	}
	if r.StartLine > n || r.EndLine > n {
		return fmt.Errorf("range past end of file (%d lines)", n)
	}
	if r.EndLine < r.StartLine ||
		(r.EndLine == r.StartLine && r.EndCol < r.StartCol) {
		return fmt.Errorf("end of range precedes start")
	}
	if r.StartCol < 0 || r.EndCol < 0 {
		return fmt.Errorf("columns are 0-based and non-negative")
	}
	return nil
}

func extract(lines []string, r Range) string {
	var quote string
	if r.StartLine == r.EndLine {
		l := lines[r.StartLine-1]
		quote = l[colByte(l, r.StartCol):colByte(l, r.EndCol)]
	} else {
		first := lines[r.StartLine-1]
		last := lines[r.EndLine-1]
		parts := []string{first[colByte(first, r.StartCol):]}
		for li := r.StartLine + 1; li <= r.EndLine-1; li++ {
			parts = append(parts, lines[li-1])
		}
		parts = append(parts, last[:colByte(last, r.EndCol)])
		quote = strings.Join(parts, "\n")
	}
	// Backstop: never persist invalid UTF-8 (it breaks TOML round-trip).
	return strings.ToValidUTF8(quote, "")
}

func context(lines []string, r Range, ctxLines int) string {
	start := max(1, r.StartLine-ctxLines)
	end := min(len(lines), r.EndLine+ctxLines)
	return strings.Join(lines[start-1:end], "\n")
}

// colByte clamps a byte column into [0,len(line)] and snaps it back to
// the nearest rune boundary, so a selection never splits a multibyte
// character (which would yield invalid UTF-8).
func colByte(line string, col int) int {
	if col <= 0 {
		return 0
	}
	if col >= len(line) {
		return len(line)
	}
	for col > 0 && !utf8.RuneStart(line[col]) {
		col--
	}
	return col
}

func id(r Range, quote string) string {
	h := sha1.Sum(fmt.Appendf(nil, "%d:%d-%d:%d|%s",
		r.StartLine, r.StartCol, r.EndLine, r.EndCol, quote))
	return hex.EncodeToString(h[:])[:8]
}

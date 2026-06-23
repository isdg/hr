// Package corrupt marks regions of an article as corrupted and reports
// them across a vault, so an LLM (or other tooling) can later restore
// the original text. Marks live in each article's .meta.toml sidecar.
package corrupt

import (
	"crypto/sha1"
	"encoding/hex"
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

// contextLines is how many lines of surrounding text are captured on
// each side of a marked region.
const contextLines = 2

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

// Mark extracts the text at r from the article and records a corruption
// on its sidecar, returning the stored entry. Re-marking the identical
// region updates the existing entry rather than duplicating it.
func Mark(articlePath string, r Range, note string) (meta.Corruption, error) {
	data, err := os.ReadFile(articlePath)
	if err != nil {
		return meta.Corruption{}, err
	}
	lines := strings.Split(string(data), "\n")
	if err := r.validate(len(lines)); err != nil {
		return meta.Corruption{}, err
	}

	quote := extract(lines, r)
	c := meta.Corruption{
		ID:        id(r, quote),
		StartLine: r.StartLine,
		StartCol:  r.StartCol,
		EndLine:   r.EndLine,
		EndCol:    r.EndCol,
		Quote:     quote,
		Context:   context(lines, r),
		Note:      textfmt.Line(note),
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

// List returns the corruption record for a single article (empty
// Corruptions if none).
func List(articlePath string) (Record, error) {
	m := meta.LoadOrDefault(articlePath)
	rec := Record{Path: articlePath, Corruptions: m.Corruptions}
	if fm, err := article.ParseFile(articlePath); err == nil {
		rec.Feed = textfmt.Line(fm.Feed)
		rec.Title = textfmt.Line(fm.Title)
	}
	return rec, nil
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

func context(lines []string, r Range) string {
	start := max(1, r.StartLine-contextLines)
	end := min(len(lines), r.EndLine+contextLines)
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

// Package jsonl reads and appends the one-JSON-value-per-line logs hr
// keeps in a vault's state directory.
package jsonl

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Append adds v as one line. O_APPEND with a single write keeps lines
// whole when two editors append at once.
func Append(path string, v any) error {
	line, err := json.Marshal(v)
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

// Load returns every line that decodes into a T that valid accepts, in
// file order. A missing log is empty; bad lines are skipped, not fatal,
// so a torn last line (crash mid-append) costs only itself.
func Load[T any](path string, valid func(T) bool) ([]T, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var v T
		if json.Unmarshal(sc.Bytes(), &v) != nil || !valid(v) {
			continue
		}
		out = append(out, v)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

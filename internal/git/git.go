// Package git records vault changes by shelling out to the git CLI, so
// the user's own config, hooks and signing apply exactly as by hand.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Commit records exactly paths in a new commit of the repository holding
// dir. Anything else staged or modified is left as it was. No paths is a
// no-op.
func Commit(ctx context.Context, dir, msg string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	spec, err := pathspec(dir, paths)
	if err != nil {
		return err
	}
	add := []string{"add", "--pathspec-from-file=-", "--pathspec-file-nul"}
	if err := run(ctx, dir, spec, add...); err != nil {
		return err
	}
	// A pathspec makes commit take only those paths (--only), not the
	// whole index.
	commit := []string{"commit", "--quiet", "-m", msg,
		"--pathspec-from-file=-", "--pathspec-file-nul"}
	return run(ctx, dir, spec, commit...)
}

// pathspec NUL-joins paths relative to dir, for --pathspec-file-nul.
func pathspec(dir string, paths []string) ([]byte, error) {
	var b bytes.Buffer
	for _, p := range paths {
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return nil, err
		}
		b.WriteString(rel)
		b.WriteByte(0)
	}
	return b.Bytes(), nil
}

func run(ctx context.Context, dir string, stdin []byte, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)
	// Article filenames are data, never globs: "[" in a slug must not
	// match some other file.
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s",
			args[0], err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

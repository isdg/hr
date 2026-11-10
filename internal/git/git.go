// Package git records vault changes by shelling out to the git CLI, so
// the user's own config, hooks and signing apply exactly as by hand.
package git

import (
	"bytes"
	"context"
	"errors"
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
	if _, err := run(ctx, dir, spec, add...); err != nil {
		return err
	}
	// A pathspec makes commit take only those paths (--only), not the
	// whole index.
	commit := []string{"commit", "--quiet", "-m", msg,
		"--pathspec-from-file=-", "--pathspec-file-nul"}
	_, err = run(ctx, dir, spec, commit...)
	return err
}

// Modified returns the tracked files under dir whose content differs from
// HEAD or that were deleted, staged or not, as absolute paths. Before the
// first commit nothing is tracked, so nothing is modified.
func Modified(ctx context.Context, dir string) ([]string, error) {
	if ok, err := hasHead(ctx, dir); !ok || err != nil {
		return nil, err
	}
	out, err := run(ctx, dir, nil, "diff", "--name-only", "-z",
		"--diff-filter=MD", "--relative", "HEAD", "--", ".")
	if err != nil {
		return nil, err
	}
	var paths []string
	for rel := range strings.SplitSeq(string(out), "\x00") {
		if rel != "" {
			paths = append(paths, filepath.Join(dir, rel))
		}
	}
	return paths, nil
}

func hasHead(ctx context.Context, dir string) (bool, error) {
	_, err := run(ctx, dir, nil,
		"rev-parse", "--quiet", "--verify", "HEAD^{commit}")
	// --verify --quiet exits 1, silently, only for a missing ref.
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return err == nil, err
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

func run(
	ctx context.Context, dir string, stdin []byte, args ...string,
) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)
	// Article filenames are data, never globs: "[" in a slug must not
	// match some other file.
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s",
			args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// Package vault resolves vault paths, opens existing vaults, and
// initializes new ones on disk.
package vault

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/isdg/hr/internal/rc"
)

//go:embed default.toml
var defaultConfig []byte

type Vault struct {
	Root string
	// State is this machine's state for the vault (caches, raw HTML,
	// logs, open history), kept out of the vault so its repository only
	// carries content. Open and Init set it to ~/.hr/<name>.
	State string
}

// Resolve returns the absolute root of the active vault for operating
// commands. Priority: explicit (flag) > discovered from the current
// directory (walking up for hr.toml, like `.git`) > $HR_VAULT.
func Resolve(explicit string) (string, error) {
	raw := explicit
	if raw == "" {
		if cwd, err := os.Getwd(); err == nil {
			if root, ok := Discover(cwd); ok {
				raw = root
			}
		}
	}
	if raw == "" {
		raw = os.Getenv("HR_VAULT")
	}
	if raw == "" {
		return "", noVault()
	}
	return absExpand(raw)
}

func noVault() error {
	msg := "no vault: pass -C <dir>, cd into a vault, or set $HR_VAULT"
	path, err := rc.Path()
	if err != nil {
		return errors.New(msg)
	}
	if r, err := rc.Load(); err == nil && r.Vault != "" {
		msg += fmt.Sprintf(" (the vault line of %s is no longer read)", path)
	}
	return errors.New(msg)
}

// Discover walks up from dir looking for an hr.toml, the way `git`
// discovers a repo root from `.git`. It lets a cloned vault work as
// soon as you `cd` into it (or a subdirectory of it), with no flag or
// $HR_VAULT needed. Returns the vault root and whether one was found.
func Discover(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "hr.toml")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// ResolveNew returns the absolute path for a vault about to be
// created: explicit, else ~/blogs/<name>. $HR_VAULT names the vault in
// use, never a new one.
func ResolveNew(explicit, name string) (string, error) {
	raw := explicit
	if raw == "" {
		raw = "~/blogs/" + name
	}
	return absExpand(raw)
}

func absExpand(p string) (string, error) {
	expanded, err := expandTilde(p)
	if err != nil {
		return "", err
	}
	return filepath.Abs(expanded)
}

func expandTilde(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, p[2:]), nil
}

func Open(root string) (*Vault, error) {
	v := &Vault{Root: root}
	if _, err := os.Stat(v.ConfigPath()); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"no hr vault at %s (run `hr init` first)", root)
		}
		return nil, err
	}
	var cfg struct {
		Name string `toml:"name"`
	}
	if _, err := toml.DecodeFile(v.ConfigPath(), &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", v.ConfigPath(), err)
	}
	state, err := StateDir(cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", v.ConfigPath(), err)
	}
	v.State = state
	if err := adoptLegacyState(v); err != nil {
		return nil, err
	}
	return v, nil
}

// StateDir returns where a vault named name keeps its state on this
// machine. The name must be unique among the machine's vaults.
func StateDir(name string) (string, error) {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) || name == "config.toml" {
		return "", fmt.Errorf("vault name %q cannot name a directory "+
			"under the hr home", name)
	}
	home, err := rc.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, name), nil
}

// LegacyStateDir is where hr kept a vault's state before ~/.hr.
func (v *Vault) LegacyStateDir() string {
	return filepath.Join(v.Root, ".hr")
}

// adoptLegacyState moves a vault's old in-repo .hr/ to its state dir,
// once. When both exist the old one is left for doctor to report.
func adoptLegacyState(v *Vault) error {
	legacy := v.LegacyStateDir()
	if _, err := os.Stat(legacy); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if _, err := os.Stat(v.State); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(v.State), 0o755); err != nil {
		return err
	}
	if err := os.Rename(legacy, v.State); err != nil {
		return fmt.Errorf("move %s to %s: %w", legacy, v.State, err)
	}
	return nil
}

func Init(root, name string) (*Vault, error) {
	state, err := StateDir(name)
	if err != nil {
		return nil, err
	}
	v := &Vault{Root: root, State: state}
	if _, err := os.Stat(state); err == nil {
		return nil, fmt.Errorf("a vault named %q already keeps its "+
			"state in %s; pick another name", name, state)
	}

	if _, err := os.Stat(v.ConfigPath()); err == nil {
		return nil, fmt.Errorf(
			"vault already initialized at %s", root)
	}

	dirs := []string{v.Root, v.FeedsDir(), v.MetaDir(), v.LogDir()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", d, err)
		}
	}

	err = os.WriteFile(v.ConfigPath(), buildConfig(name), 0o644)
	if err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}
	return v, nil
}

func buildConfig(name string) []byte {
	header := fmt.Sprintf("name = %q\n\n", name)
	return append([]byte(header), defaultConfig...)
}

func (v *Vault) ConfigPath() string {
	return filepath.Join(v.Root, "hr.toml")
}

func (v *Vault) FeedsDir() string {
	return filepath.Join(v.Root, "feeds")
}

// Rel renders p relative to the vault root with forward slashes, for
// use in messages. Paths outside the vault are returned unchanged.
func (v *Vault) Rel(p string) string {
	rel, err := filepath.Rel(v.Root, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return filepath.ToSlash(rel)
}

func (v *Vault) MetaDir() string {
	if v.State == "" {
		panic("vault: State unset; get the Vault from Open or Init")
	}
	return v.State
}

// HistoryPath is the log of opened articles behind `hr recent`. Unlike
// the rest of MetaDir it is not regenerable: deleting it loses history.
func (v *Vault) HistoryPath() string {
	return filepath.Join(v.MetaDir(), "history.jsonl")
}

// BookmarksPath is the log of reading places behind `hr bookmark`. Like
// history it is not regenerable.
func (v *Vault) BookmarksPath() string {
	return filepath.Join(v.MetaDir(), "bookmarks.jsonl")
}

func (v *Vault) CachePath() string {
	return filepath.Join(v.MetaDir(), "cache.json")
}

// IndexPath is the listing cache: a single file mirroring every
// article's parsed frontmatter + meta so `hr list`/`feed` can skip
// re-parsing unchanged files. Rebuilt lazily; safe to delete.
func (v *Vault) IndexPath() string {
	return filepath.Join(v.MetaDir(), "index.json")
}

func (v *Vault) LogDir() string {
	return filepath.Join(v.MetaDir(), "log")
}

func (v *Vault) RawDir() string {
	return filepath.Join(v.MetaDir(), "raw")
}

// RawPath returns the archive location for an article's original HTML,
// mirroring the feeds/ layout: <vault>/.hr/raw/<feed>/<base>.html where
// <base> is the article filename minus its .md suffix.
func (v *Vault) RawPath(feedName, articleFilename string) string {
	base := strings.TrimSuffix(articleFilename, ".md")
	return filepath.Join(v.RawDir(), feedName, base+".html")
}

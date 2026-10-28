// Package rc reads and writes ~/.hrrc: the user's pointer to the active
// hr vault and their own defaults for every vault's settings.
package rc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/isdg/hr/internal/config"
)

type RC struct {
	Vault string `toml:"vault"`
	config.Settings

	// Unknown lists the keys Load did not recognise, which are ignored.
	Unknown []string `toml:"-"`
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".hrrc"), nil
}

func Load() (*RC, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &RC{}, nil
	}
	if err != nil {
		return nil, err
	}
	var r RC
	md, err := toml.Decode(string(data), &r)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, k := range md.Undecoded() {
		r.Unknown = append(r.Unknown, k.String())
	}
	return &r, nil
}

var (
	vaultLine  = regexp.MustCompile(`^\s*vault\s*=`)
	tableStart = regexp.MustCompile(`^\s*\[`)
)

// SetVault points ~/.hrrc at vault, or unsets it when vault is "". It
// edits the text rather than re-encoding the file, so the user's
// settings and comments survive.
func SetVault(vault string) error {
	path, err := Path()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var lines []string
	if text := strings.TrimSuffix(string(data), "\n"); text != "" {
		lines = strings.Split(text, "\n")
	}

	// A top-level key must precede the first table header.
	top := slices.IndexFunc(lines, tableStart.MatchString)
	if top < 0 {
		top = len(lines)
	}
	line := "vault = " + strconv.Quote(vault)
	switch i := slices.IndexFunc(lines[:top], vaultLine.MatchString); {
	case i >= 0 && vault == "":
		lines = slices.Delete(lines, i, i+1)
	case i >= 0:
		lines[i] = line
	case vault != "":
		lines = slices.Insert(lines, top, line)
	}

	out := strings.Join(lines, "\n")
	if out != "" {
		out += "\n"
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

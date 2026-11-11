package listing

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/isdg/hr/internal/config"
	"github.com/isdg/hr/internal/textfmt"
	"github.com/isdg/hr/internal/vault"
)

// Feed summarizes one feed declared in hr.toml.
type Feed struct {
	Name        string `json:"name"`
	Group       string `json:"group"` // "" is the feeds root
	Articles    int    `json:"articles"`
	Unread      int    `json:"unread"`
	Description string `json:"description,omitempty"`

	// About is the path of the feed's vault.AboutFile, when it has one.
	About string `json:"about,omitempty"`
}

// Feeds summarizes the declared feeds that fall under groups (see
// inGroups), sorted by group, then name. A feed with no directory yet
// sits in its configured group, with no articles.
func Feeds(
	v *vault.Vault, declared []config.Feed, groups []string,
) ([]Feed, error) {
	loc, err := v.Locate()
	if err != nil {
		return nil, err
	}
	items, err := loadAll(v)
	if err != nil {
		return nil, err
	}
	type count struct{ articles, unread int }
	byDir := map[string]count{}
	for _, it := range items {
		c := byDir[filepath.Dir(it.Path)]
		c.articles++
		if !it.Read {
			c.unread++
		}
		byDir[filepath.Dir(it.Path)] = c
	}

	out := make([]Feed, 0, len(declared))
	for _, f := range declared {
		dir, err := loc.Dir(f.Name, f.Group)
		if err != nil {
			return nil, err
		}
		fd := Feed{
			Name:        f.Name,
			Group:       v.GroupOf(dir),
			Articles:    byDir[dir].articles,
			Unread:      byDir[dir].unread,
			Description: textfmt.Line(f.Description),
		}
		if !inGroups(fd.Group, groups) {
			continue
		}
		about := filepath.Join(dir, vault.AboutFile)
		if _, err := os.Stat(about); err == nil {
			fd.About = about
		}
		out = append(out, fd)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Package article writes frontmatter+body markdown files for fetched
// items.
package article

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Article struct {
	Title     string
	URL       string
	Published time.Time
	FeedName  string
	GUID      string
	Body      string
}

func (a *Article) Filename() string {
	date := a.Published.Format("2006-01-02")
	return fmt.Sprintf("%s-%s-%s.md", date, slugify(a.Title), a.ID())
}

// ID returns a stable 8-char fingerprint for the article, derived from
// the GUID (or URL, or title+timestamp) so the same item produces the
// same filename across syncs.
func (a *Article) ID() string {
	src := a.GUID
	if src == "" {
		src = a.URL
	}
	if src == "" {
		src = a.Title + a.Published.Format(time.RFC3339)
	}
	h := sha1.Sum([]byte(src))
	return hex.EncodeToString(h[:])[:8]
}

var (
	nonAlphanum = regexp.MustCompile(`[^a-z0-9]+`)
	multiDash   = regexp.MustCompile(`-+`)
)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanum.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		s = "untitled"
	}
	return s
}

// Write writes the article to its feed subdirectory if it doesn't
// already exist. Returns (created, path, error).
func Write(feedsDir string, a *Article) (bool, string, error) {
	feedDir := filepath.Join(feedsDir, a.FeedName)
	if err := os.MkdirAll(feedDir, 0o755); err != nil {
		return false, "", err
	}
	path := filepath.Join(feedDir, a.Filename())
	if _, err := os.Stat(path); err == nil {
		return false, path, nil
	}

	content, err := render(a)
	if err != nil {
		return false, "", err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return false, "", err
	}
	return true, path, nil
}

type frontmatter struct {
	Title     string `yaml:"title"`
	URL       string `yaml:"url"`
	Published string `yaml:"published"`
	Feed      string `yaml:"feed"`
	GUID      string `yaml:"guid,omitempty"`
}

func render(a *Article) ([]byte, error) {
	fm := frontmatter{
		Title:     a.Title,
		URL:       a.URL,
		Published: a.Published.Format(time.RFC3339),
		Feed:      a.FeedName,
		GUID:      a.GUID,
	}
	fmData, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}

	body := cleanBody(a.Body, a.Title)

	var b strings.Builder
	b.WriteString("---\n")
	b.Write(fmData)
	b.WriteString("---\n\n")
	b.WriteString(body)
	b.WriteString("\n")
	return []byte(b.String()), nil
}

// cleanBody strips leading h1 lines from body that don't reference the
// article title (usually feed-level navigation/breadcrumbs), then
// ensures the body opens with `# <title>`.
func cleanBody(body, title string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return "# " + title
	}
	for {
		nl := strings.IndexByte(body, '\n')
		first := body
		if nl >= 0 {
			first = body[:nl]
		}
		if !strings.HasPrefix(first, "# ") {
			break
		}
		if containsTitle(first, title) {
			return body
		}
		if nl < 0 {
			body = ""
			break
		}
		body = strings.TrimLeft(body[nl+1:], "\n")
	}
	if body == "" {
		return "# " + title
	}
	return "# " + title + "\n\n" + body
}

func containsTitle(line, title string) bool {
	if title == "" {
		return false
	}
	return strings.Contains(
		strings.ToLower(line), strings.ToLower(title))
}

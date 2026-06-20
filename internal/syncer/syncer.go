// Package syncer orchestrates per-feed fetch + write across feeds.
package syncer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/mmcdole/gofeed"

	"github.com/isg9/hr/internal/article"
	"github.com/isg9/hr/internal/cache"
	"github.com/isg9/hr/internal/config"
	"github.com/isg9/hr/internal/errlog"
	"github.com/isg9/hr/internal/feed"
	"github.com/isg9/hr/internal/meta"
	"github.com/isg9/hr/internal/vault"
)

type Options struct {
	Vault     *vault.Vault
	Config    *config.Config
	FeedName  string // empty = all feeds
	UserAgent string
	Force     bool // ignore cache; refetch even if not modified

	// OnFeedStart/OnFeedDone, if set, are called around each feed so
	// callers can show live progress. i is 1-based; total is the feed
	// count. Both run on Run's goroutine (no concurrency to guard).
	OnFeedStart func(i, total int, name string)
	OnFeedDone  func(i, total int, fr FeedResult)
}

type FeedResult struct {
	Name        string
	URL         string
	New         int
	Existing    int
	NotModified bool
	Err         error
}

type Result struct {
	Feeds []FeedResult
}

func Run(ctx context.Context, opts Options) (*Result, error) {
	elog := errlog.New(filepath.Join(opts.Vault.MetaDir(), "err.txt"))

	c, err := cache.Load(opts.Vault.CachePath())
	if err != nil {
		elog.Write("cache.load", err)
		return nil, fmt.Errorf("load cache: %w", err)
	}

	feeds, err := selectFeeds(opts.Config.Feeds, opts.FeedName)
	if err != nil {
		return nil, err
	}

	conv := md.NewConverter("", true, nil)
	result := &Result{Feeds: make([]FeedResult, 0, len(feeds))}
	total := len(feeds)
	for i, f := range feeds {
		if opts.OnFeedStart != nil {
			opts.OnFeedStart(i+1, total, f.Name)
		}
		fr := syncFeed(ctx, opts, f, c, conv, elog)
		if opts.OnFeedDone != nil {
			opts.OnFeedDone(i+1, total, fr)
		}
		result.Feeds = append(result.Feeds, fr)
	}

	if err := c.Save(); err != nil {
		elog.Write("cache.save", err)
		return result, fmt.Errorf("save cache: %w", err)
	}
	return result, nil
}

func selectFeeds(
	all []config.Feed, name string,
) ([]config.Feed, error) {
	if name == "" {
		return all, nil
	}
	for _, f := range all {
		if f.Name == name {
			return []config.Feed{f}, nil
		}
	}
	return nil, fmt.Errorf("feed %q not found in config", name)
}

func syncFeed(
	ctx context.Context,
	opts Options,
	f config.Feed,
	c *cache.Cache,
	conv *md.Converter,
	elog *errlog.Log,
) FeedResult {
	fr := FeedResult{Name: f.Name, URL: f.URL}
	tag := "feed:" + f.Name

	entry := c.Get(f.Name)
	fopts := feed.Options{UserAgent: opts.UserAgent}
	if !opts.Force {
		fopts.ETag = entry.ETag
		fopts.LastModified = entry.LastModified
	}
	res, err := feed.Fetch(ctx, f.URL, fopts)
	if err != nil {
		elog.Write(tag+":fetch", err)
		fr.Err = err
		return fr
	}
	if res.NotModified {
		fr.NotModified = true
		return fr
	}

	for _, item := range res.Feed.Items {
		a := itemToArticle(item, f.Name, conv)
		written, path, err := article.Write(opts.Vault.FeedsDir(), a)
		if err != nil {
			elog.Write(tag+":write", err)
			fr.Err = err
			return fr
		}
		if written {
			fr.New++
		} else {
			fr.Existing++
		}
		if _, err := meta.WriteIfAbsent(path); err != nil {
			elog.Write(tag+":meta", err)
			fr.Err = err
			return fr
		}
		if err := writeRawHTML(opts.Vault, item, a); err != nil {
			elog.Write(tag+":raw", err)
		}
	}

	c.Set(f.Name, cache.Entry{
		ETag:         res.ETag,
		LastModified: res.LastModified,
		FetchedAt:    time.Now().UTC(),
	})
	return fr
}

// writeRawHTML stashes the original feed body HTML at
// <vault>/.hr/raw/<id>.html the first time we see an item. Idempotent
// (skips if the file already exists). Cheap insurance against future
// HTML→markdown conversion bugs.
func writeRawHTML(
	v *vault.Vault, item *gofeed.Item, a *article.Article,
) error {
	html := item.Content
	if html == "" {
		html = item.Description
	}
	if html == "" {
		return nil
	}
	rawPath := v.RawPath(a.FeedName, a.Filename())
	if _, err := os.Stat(rawPath); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(rawPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(rawPath, []byte(html), 0o644)
}

func itemToArticle(
	item *gofeed.Item, feedName string, conv *md.Converter,
) *article.Article {
	title := item.Title
	if title == "" {
		title = "(untitled)"
	}
	return &article.Article{
		Title:     title,
		URL:       item.Link,
		Published: derivePublished(item),
		FeedName:  feedName,
		GUID:      item.GUID,
		Body:      extractBody(item, conv),
	}
}

func extractBody(item *gofeed.Item, conv *md.Converter) string {
	html := item.Content
	if html == "" {
		html = item.Description
	}
	if html == "" {
		return ""
	}
	out, err := conv.ConvertString(html)
	if err != nil {
		return html
	}
	return out
}

func derivePublished(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil {
		return *item.UpdatedParsed
	}
	return time.Now().UTC()
}

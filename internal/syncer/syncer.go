// Package syncer orchestrates per-feed fetch + write across feeds.
package syncer

import (
	"context"
	"fmt"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/mmcdole/gofeed"

	"github.com/isg/hrb/internal/article"
	"github.com/isg/hrb/internal/cache"
	"github.com/isg/hrb/internal/config"
	"github.com/isg/hrb/internal/feed"
	"github.com/isg/hrb/internal/meta"
	"github.com/isg/hrb/internal/vault"
)

type Options struct {
	Vault     *vault.Vault
	Config    *config.Config
	FeedName  string // empty = all feeds
	UserAgent string
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
	c, err := cache.Load(opts.Vault.CachePath())
	if err != nil {
		return nil, fmt.Errorf("load cache: %w", err)
	}

	feeds, err := selectFeeds(opts.Config.Feeds, opts.FeedName)
	if err != nil {
		return nil, err
	}

	conv := md.NewConverter("", true, nil)
	result := &Result{Feeds: make([]FeedResult, 0, len(feeds))}
	for _, f := range feeds {
		fr := syncFeed(ctx, opts, f, c, conv)
		result.Feeds = append(result.Feeds, fr)
	}

	if err := c.Save(); err != nil {
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
) FeedResult {
	fr := FeedResult{Name: f.Name, URL: f.URL}

	entry := c.Get(f.Name)
	res, err := feed.Fetch(ctx, f.URL, feed.Options{
		UserAgent:    opts.UserAgent,
		ETag:         entry.ETag,
		LastModified: entry.LastModified,
	})
	if err != nil {
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
			fr.Err = err
			return fr
		}
		if written {
			fr.New++
		} else {
			fr.Existing++
		}
		if _, err := meta.WriteIfAbsent(path); err != nil {
			fr.Err = err
			return fr
		}
	}

	c.Set(f.Name, cache.Entry{
		ETag:         res.ETag,
		LastModified: res.LastModified,
		FetchedAt:    time.Now().UTC(),
	})
	return fr
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

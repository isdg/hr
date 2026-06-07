// Package feed wraps gofeed with conditional-GET caching.
package feed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

type Result struct {
	NotModified  bool
	ETag         string
	LastModified string
	Feed         *gofeed.Feed
}

type Options struct {
	UserAgent    string
	ETag         string
	LastModified string
}

func Fetch(
	ctx context.Context, url string, opts Options,
) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}
	if opts.ETag != "" {
		req.Header.Set("If-None-Match", opts.ETag)
	}
	if opts.LastModified != "" {
		req.Header.Set("If-Modified-Since", opts.LastModified)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return &Result{NotModified: true}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	parsed, err := gofeed.NewParser().ParseString(string(body))
	if err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}

	return &Result{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		Feed:         parsed,
	}, nil
}

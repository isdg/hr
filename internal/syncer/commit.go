package syncer

import (
	"fmt"
	"sort"
	"strings"
)

// Written returns every file the run created, across all feeds.
func (r *Result) Written() []string {
	var paths []string
	for _, fr := range r.Feeds {
		paths = append(paths, fr.Written...)
	}
	return paths
}

// CommitMessage describes a vault commit of the run's writes plus updated
// pre-existing sidecars (read state and the like). The subject names the
// first of new articles, missing sidecars, updates that applies.
func (r *Result) CommitMessage(updated int) string {
	var feeds []FeedResult
	var total, nameW int
	for _, fr := range r.Feeds {
		if fr.New > 0 {
			feeds = append(feeds, fr)
			total += fr.New
			nameW = max(nameW, len(fr.Name))
		}
	}
	sort.SliceStable(feeds, func(i, j int) bool {
		if feeds[i].New != feeds[j].New {
			return feeds[i].New > feeds[j].New
		}
		return feeds[i].Name < feeds[j].Name
	})

	state := "update " + count(updated, "sidecar")

	var subject string
	var body []string
	switch written := len(r.Written()); {
	case total > 0:
		subject = "sync: " + count(total, "new article")
		var b strings.Builder
		for _, fr := range feeds {
			fmt.Fprintf(&b, "%-*s  +%d\n", nameW, fr.Name, fr.New)
		}
		body = append(body, strings.TrimSuffix(b.String(), "\n"))
	case written > 0:
		// Only sidecars for articles already on disk were written.
		subject = "sync: add " + count(written, "missing sidecar")
	default:
		return "sync: " + state
	}
	if updated > 0 {
		body = append(body, state)
	}
	return strings.Join(append([]string{subject}, body...), "\n\n")
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

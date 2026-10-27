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

// CommitMessage describes the run's writes for a vault commit: the new
// article count as the subject, then each feed that gained any, most
// first.
func (r *Result) CommitMessage() string {
	var feeds []FeedResult
	var total, nameW int
	for _, fr := range r.Feeds {
		if fr.New > 0 {
			feeds = append(feeds, fr)
			total += fr.New
			nameW = max(nameW, len(fr.Name))
		}
	}
	if total == 0 {
		// Only sidecars for articles already on disk were written.
		return fmt.Sprintf("sync: add %s",
			count(len(r.Written()), "missing sidecar"))
	}
	sort.SliceStable(feeds, func(i, j int) bool {
		if feeds[i].New != feeds[j].New {
			return feeds[i].New > feeds[j].New
		}
		return feeds[i].Name < feeds[j].Name
	})

	var b strings.Builder
	fmt.Fprintf(&b, "sync: %s\n\n", count(total, "new article"))
	for _, fr := range feeds {
		fmt.Fprintf(&b, "%-*s  +%d\n", nameW, fr.Name, fr.New)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

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

// CommitMessage describes a vault commit of the run's new articles plus
// the updated sidecars (read state and the like) that ride along.
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
	if total == 0 {
		return "sync: " + state
	}
	// A title tells hourly syncs apart in a log; the count alone repeats.
	subject := "sync: " + count(total, "new article")
	if titles := feeds[0].Titles; len(titles) > 0 {
		title := strings.Join(strings.Fields(titles[0]), " ")
		subject = fit(fmt.Sprintf("sync: %d new: %s", total, title))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", subject)
	for _, fr := range feeds {
		fmt.Fprintf(&b, "%-*s  +%d\n", nameW, fr.Name, fr.New)
	}
	if updated > 0 {
		fmt.Fprintf(&b, "\n%s\n", state)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// maxSubject is git's customary subject limit, which commit-msg hooks
// commonly enforce, in characters.
const maxSubject = 50

// fit cuts s to maxSubject characters, marking the cut with an ellipsis.
func fit(s string) string {
	r := []rune(s)
	if len(r) <= maxSubject {
		return s
	}
	return strings.TrimRight(string(r[:maxSubject-1]), " ") + "…"
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

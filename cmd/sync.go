package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/config"
	"github.com/isdg/hr/internal/git"
	"github.com/isdg/hr/internal/syncer"
	"github.com/isdg/hr/internal/vault"
)

var (
	syncFeedFilter string
	syncForce      bool
	syncJobs       int
	syncCommit     bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Fetch new items for all (or filtered) feeds",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := vault.Resolve(vaultFlag)
		if err != nil {
			return err
		}
		v, err := vault.Open(root)
		if err != nil {
			return err
		}
		cfg, r, err := loadConfig(v)
		if err != nil {
			return err
		}
		if n := len(cfg.Unknown) + len(r.Unknown); n > 0 {
			fmt.Fprintf(os.Stderr, "warning: %s in hr.toml or "+
				"~/.hr/config.toml, "+
				"ignored (see `hr doctor`)\n", plural(n, "unknown key"))
		}
		ua := cfg.UserAgent
		if ua == "" {
			ua = "hr/0.1"
		}

		// Live progress → stderr; the résumé → stdout. Color on each is
		// gated on that stream being a terminal.
		live := newStyler(os.Stderr)
		nameW, total := feedLayout(cfg, syncFeedFilter)
		counterW := len(strconv.Itoa(total))

		start := time.Now()
		res, err := syncer.Run(cmd.Context(), syncer.Options{
			Vault:       v,
			Config:      cfg,
			FeedName:    syncFeedFilter,
			UserAgent:   ua,
			Force:       syncForce,
			Concurrency: syncJobs,
			OnFeedDone: func(i, tot int, fr syncer.FeedResult) {
				fmt.Fprintln(os.Stderr,
					liveLine(live, nameW, counterW, i, tot, fr))
			},
		})
		if res == nil {
			return err
		}
		printSyncSummary(os.Stdout, newStyler(os.Stdout), res,
			time.Since(start))

		commit := cfg.AutoCommit
		if cmd.Flags().Changed("commit") {
			commit = syncCommit
		}
		if !commit {
			return err
		}
		// Commit even after a partial failure: what was written is
		// complete, and leaving it out would only defer it to a later run.
		return errors.Join(err, commitSync(cmd, v, res))
	},
}

// commitSync commits the files the sync wrote and the sidecars changed
// since the last commit (read state, favorites, aliases), and reports it.
func commitSync(cmd *cobra.Command, v *vault.Vault, res *syncer.Result) error {
	ctx := cmd.Context()
	paths := res.Written()
	updated, err := changedSidecars(ctx, v, paths)
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	if len(paths)+len(updated) == 0 {
		return nil
	}
	msg := res.CommitMessage(len(updated))
	err = git.Commit(ctx, v.Root, msg, append(paths, updated...))
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	subject, _, _ := strings.Cut(msg, "\n")
	fmt.Printf("  committed: %s\n", subject)
	return nil
}

// changedSidecars returns the vault's sidecars created, changed or deleted
// since the last commit, except new ones whose article stays uncommitted
// (written is what this commit adds).
func changedSidecars(
	ctx context.Context, v *vault.Vault, written []string,
) ([]string, error) {
	modified, err := git.Modified(ctx, v.FeedsDir())
	if err != nil {
		return nil, err
	}
	untracked, err := git.Untracked(ctx, v.FeedsDir())
	if err != nil {
		return nil, err
	}
	pending := make(map[string]bool, len(untracked))
	for _, p := range untracked {
		pending[p] = true
	}
	for _, p := range written {
		delete(pending, p)
	}
	var sidecars []string
	for _, p := range slices.Concat(modified, untracked) {
		base, ok := strings.CutSuffix(p, ".meta.toml")
		if ok && !pending[base+".md"] {
			sidecars = append(sidecars, p)
		}
	}
	return sidecars, nil
}

// feedLayout returns the widest selected feed name and how many feeds are
// selected, so the live log can align its columns before syncing starts.
func feedLayout(cfg *config.Config, filter string) (nameW, total int) {
	for _, f := range cfg.Feeds {
		if filter != "" && f.Name != filter {
			continue
		}
		if len(f.Name) > nameW {
			nameW = len(f.Name)
		}
		total++
	}
	return nameW, total
}

// liveLine renders one uv-style progress line as a feed finishes:
//
//	26/61  yandex           +3 new
//
// counter dim, name left-padded, status green (+N new) / dim
// (· up to date, · archived) / red (✗ error) / green (· manual).
func liveLine(
	st styler, nameW, counterW, i, total int, fr syncer.FeedResult,
) string {
	counter := st.dim(fmt.Sprintf("%*d/%d", counterW, i, total))
	var status string
	switch {
	case fr.Err != nil:
		status = st.red("✗ error")
	case fr.Archived:
		status = st.dim("· archived")
	case fr.Manual:
		status = st.green("· manual")
	case fr.New > 0:
		status = st.green(fmt.Sprintf("+%d new", fr.New))
	default:
		status = st.dim("· up to date")
	}
	return fmt.Sprintf("  %s  %-*s  %s", counter, nameW, fr.Name, status)
}

// printSyncSummary writes the résumé to w: a one-line header, only the
// feeds that gained new items (most first), the manual feeds still due
// an update, any errors, and a footer of aggregate counts.
func printSyncSummary(
	w io.Writer, st styler, r *syncer.Result, elapsed time.Duration,
) {
	var changed, errored, manual []syncer.FeedResult
	var totNew, unchanged, archived, nameW int
	for _, fr := range r.Feeds {
		switch {
		case fr.Err != nil:
			errored = append(errored, fr)
		case fr.Archived:
			archived++
		case fr.Manual:
			manual = append(manual, fr)
			if len(fr.Name) > nameW {
				nameW = len(fr.Name)
			}
		case fr.New > 0:
			changed = append(changed, fr)
			totNew += fr.New
			if len(fr.Name) > nameW {
				nameW = len(fr.Name)
			}
		default:
			unchanged++
		}
	}
	sort.Slice(changed, func(i, j int) bool {
		if changed[i].New != changed[j].New {
			return changed[i].New > changed[j].New
		}
		return changed[i].Name < changed[j].Name
	})

	feeds := "feeds"
	if len(r.Feeds) == 1 {
		feeds = "feed"
	}
	fmt.Fprintf(w, "%s synced %d %s in %s\n",
		st.bold("✓"), len(r.Feeds), feeds, fmtDur(elapsed))

	if len(changed) > 0 {
		fmt.Fprintln(w)
		for _, fr := range changed {
			fmt.Fprintf(w, "  %-*s  %s   %s\n", nameW, fr.Name,
				st.green(fmt.Sprintf("+%d", fr.New)),
				st.dim(fmt.Sprintf("(%d total)", fr.Total)))
		}
	}
	// Hand-curated feeds are never fetched, so say plainly that they only
	// move when the user updates them.
	if len(manual) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %s update by hand (no feed to poll):\n",
			st.green("·"))
		sort.Slice(manual, func(i, j int) bool {
			return manual[i].Name < manual[j].Name
		})
		for _, fr := range manual {
			fmt.Fprintf(w, "    %-*s  %s\n", nameW, fr.Name,
				st.dim(fmt.Sprintf("(%d total)", fr.Total)))
		}
	}
	if len(errored) > 0 {
		fmt.Fprintln(w)
		for _, fr := range errored {
			fmt.Fprintf(w, "  %s %s: %v\n", st.red("✗"), fr.Name, fr.Err)
		}
	}

	fmt.Fprintln(w)
	errPart := fmt.Sprintf("%d errors", len(errored))
	if len(errored) > 0 {
		errPart = st.red(errPart)
	}
	// "new" counts articles but the rest count feeds, so say so — side by
	// side they otherwise read as the same unit.
	line := fmt.Sprintf("  %d new · %s unchanged",
		totNew, plural(unchanged, "feed"))
	if len(manual) > 0 {
		line += fmt.Sprintf(" · %s manual", plural(len(manual), "feed"))
	}
	if archived > 0 {
		line += fmt.Sprintf(" · %d archived", archived)
	}
	fmt.Fprintf(w, "%s · %s\n", line, errPart)
}

// fmtDur renders an elapsed duration compactly, e.g. "6.2s".
func fmtDur(d time.Duration) string {
	return d.Round(100 * time.Millisecond).String()
}

func init() {
	syncCmd.Flags().StringVar(&syncFeedFilter, "feed", "",
		"sync only this feed name")
	syncCmd.Flags().BoolVar(&syncForce, "force", false,
		"ignore cache and refetch even if not modified")
	syncCmd.Flags().IntVarP(&syncJobs, "jobs", "j", 0,
		"number of feeds to fetch in parallel (0 = default)")
	syncCmd.Flags().BoolVar(&syncCommit, "commit", false,
		"commit new articles and changed sidecars to git "+
			"(default: autocommit in hr.toml)")
	rootCmd.AddCommand(syncCmd)
}

HR
==

A file-first RSS / blog reader. Articles live as plain markdown files in a
git-syncable vault. There is no database — find, grep, git, and your editor
are the UI.


-------------------------------------------------------------------------------
COMMANDS
-------------------------------------------------------------------------------

    hr init <name> [dir]      scaffold a vault
    hr vault                  print the vault hr would use here
    hr doctor                 check the vault for inconsistencies
    hr sync                   fetch feeds, write new articles
    hr mv <feed>... <group>   move feed folders into a group under feeds/
    hr groups                 folders under feeds/ + feed/article/unread
                              counts
    hr list                   list articles (pretty, --tsv, --json;
                              --fav, --bookmarked and more filter)
    hr recent                 articles you opened, latest first (takes
                              hr list's flags; --since = opened within)
    hr opened <path>...       record an open (hr.vim runs it on load)
    hr read <path>...         mark as read (sidecar mutation)
    hr unread <path>...
    hr fav <path>...          toggle favorite
    hr alias <path> [name]    local display label (no [name] = clear)
    hr bookmark <path> [L[:C]]
                              set (or show) the reading place: 1-based
                              line, 0-based byte column; --clear drops it
    hr bookmark --last        newest bookmark as PATH<TAB>LINE:COL: where
                              to resume reading
    hr bookmarks              bookmarked articles, newest bookmark first
                              (takes hr list's flags; --since = bookmarked
                              within)

Nvim panel: hr.vim (github.com/isdg/hr.vim), a sidebar with its own
buffer keys (? lists them).

-------------------------------------------------------------------------------
VAULT LAYOUT
-------------------------------------------------------------------------------

    <vault>/
      hr.toml                                     config (committed)
      feeds/<feed>/<date>-<slug>-<id>.md          article content (committed)
      feeds/<feed>/<date>-<slug>-<id>.meta.toml   mutable state (committed;
                                                  absent while all
                                                  defaults)

    ~/.hr/ ($HR_HOME)                             this machine's, outside
                                                  every vault
      .hrrc                                       your default vault and
                                                  settings for every vault;
                                                  hr never writes it
      <name>/                                    one per vault, by the
                                                  name in its hr.toml
        cache.json                                ETag / Last-Modified per
                                                  feed
        index.json                                listing cache
        raw/<feed>/<date>-<slug>-<id>.html        original HTML, keyed by
                                                  feed name
        err.txt                                   one line per error during
                                                  sync
        history.jsonl                             one line per open, for
                                                  hr recent; NOT
                                                  regenerable
        bookmarks.jsonl                           one line per reading-
                                                  place change, latest
                                                  wins; NOT regenerable

Vault names must be unique on a machine: hr init refuses a name whose
state directory exists. Older hr kept the per-vault state in <vault>/.hr/,
which moves into ~/.hr on first use, and its settings in ~/.hrrc or
~/.hr/config.toml, which move to ~/.hr/.hrrc when it is missing; a
scratch $HR_HOME never takes ~/.hrrc. hr doctor reports what could not
move.

The <id> segment is the first 8 hex chars of sha1(GUID || URL || title), so
the same item lands on the same path across machines.

-------------------------------------------------------------------------------
GROUPING FEEDS INTO FOLDERS
-------------------------------------------------------------------------------

Feed directories may be nested under feeds/ to any depth, so a large vault
can be shelved by kind:

    feeds/
      humans/matklad/…        personal blogs
      sites/lobsters/…        aggregators, project blogs
      books/classics/knuth/…  hand-curated, never synced

The directory is the source of truth. hr finds a feed by its directory name
wherever it sits, so you regroup with plain mv / git mv and nothing else has
to change — no config edit, no re-sync, no rewritten article paths. Feed
directories hr never fetched (a shelf of essays you assembled by hand) group
exactly like synced ones.

hr mv does the move for you, in batches:

    > hr mv matklad danluu regehr humans
    > hr mv lobsters lwn sites
    > hr mv knuth turing books/classics
    > hr mv lobsters /                  # back to the top level
    > hr mv --dry-run matklad humans    # show the plan, touch nothing

The group is created if missing, and group folders a move leaves empty are
removed. The whole batch is validated before anything moves, so an unknown
feed or an occupied destination fails with the vault untouched. Only
directories move — articles, sidecars, ids and read state are untouched, and
it's a plain rename, so git records it as one.

For feeds declared in hr.toml, hr mv also updates their group key in place,
preserving every comment and the rest of the formatting (--no-config skips
it).

-------------------------------------------------------------------------------
READING A SHELF OR AN AUTHOR
-------------------------------------------------------------------------------

hr groups summarizes the tree; hr list and hr feed take --group and --feed
filters, both repeatable:

    > hr groups                                    # counts per folder
    > hr list --group humans                       # includes humans/archive
    > hr list --group books --group humans/archive # stacks (union)
    > hr list --group books,sites                  # comma form works too
    > hr list --group / --unread                   # top-level feeds only
    > hr list --feed matklad                       # one author
    > hr list --feed matklad,danluu                # several authors (union)
    > hr list --feed matklad --group humans        # both must hold
    > hr feed --group sites --feed lobsters        # unread TSV, scoped

--group and --feed are each repeatable and union their own values, while the
two flags intersect with each other — so several shelves or several authors
widen the result, but combining a shelf with an author narrows it. Both
compose with --unread, --tag, --since and the rest rather than replacing
them.

--group matches by subtree, so --group humans covers humans/archive; a bare
/ means the feeds root only. --feed matches the feed name exactly — it's a
name, not a path or a prefix.

hr groups counts each group's own feeds, not its subgroups, so the numbers
sum to the vault total instead of double-counting nested shelves. --tsv
output gained the group as a trailing seventh column, so anything indexing
fields 1-6 keeps working.

A directory under feeds/ holding at least one file is a feed; one holding
only subdirectories is a group. Two directories sharing a name are
ambiguous — sync and hr doctor both report it rather than guessing.

group in hr.toml is optional and does one thing: it decides where a feed
with no directory yet gets created on its first sync.

    [[feeds]]
    url   = "https://lobste.rs/newest.rss"
    name  = "lobsters"
    group = "sites"

Once the directory exists, its location on disk wins over group.

~/.hr/<name>/raw/<feed>/ stays flat, keyed by feed name — it's a local
cache, so moving a feed never has to touch it.

-------------------------------------------------------------------------------
HAND-CURATED FEEDS
-------------------------------------------------------------------------------

Not every source has a feed worth polling. A site with no RSS, or one whose
feed carries only headlines, can still live in the vault as a manual feed:
declare it with the manual tag and no url.

    [[feeds]]
    name  = "paulgraham"
    tags  = ["manual"]
    group = "humans"

hr sync never fetches such a feed. It reports it instead, so a sync run
tells you which feeds only move when you update them by hand:

      1/2  paulgraham  · manual
      2/2  lobsters    +3 new

      · update by hand (no feed to poll):
        paulgraham  (18 total)

      3 new · 0 feeds unchanged · 1 feed manual · 0 errors

Declaring them has a second benefit: hr doctor stops reporting the directory
as "not declared in hr.toml". A url alongside the tag is flagged as
contradictory, since sync skips the feed either way.

Some hand-curated feeds are finished: an author's collected works, or a site
that went quiet. Add the archive tag and sync stops listing them as due for
an update, counting them in the footer instead:

    [[feeds]]
    name  = "godel"
    tags  = ["manual", "archive"]
    group = "humans"

      3 new · 0 feeds unchanged · 1 feed manual · 1 archived · 0 errors

archive only means something on a manual feed; hr doctor warns about one
without the manual tag.

-------------------------------------------------------------------------------
COMMITTING WHAT SYNC FETCHED
-------------------------------------------------------------------------------

With autocommit = true in hr.toml, hr sync commits the articles it just
wrote to the git repository holding the vault, along with every sidecar
created, changed or deleted since the last commit (read state,
favorites, aliases set by hr read, hr fav, the nvim panel). A new sidecar
waits while its article itself is uncommitted:

    sync: 9 new articles

    lobsters  +5
    bun       +2
    lwn       +2

    update 3 sidecars

With no new articles, only changed sidecars, the commit is
"sync: update 3 sidecars".

Only those files go in. Anything else you have staged or modified, edited
article bodies included, is left alone, and nothing is pushed.
hr sync --commit or --commit=false overrides the setting for one run.

A failed commit (a hook rejecting it, say) fails the sync but keeps the
articles on disk. Later syncs won't pick them up, since they only commit
what they themselves write, so commit those by hand. Changed sidecars are
picked up by the next sync either way.

-------------------------------------------------------------------------------
WHICH VAULT DOES HR USE?
-------------------------------------------------------------------------------

Resolved in priority order: -C <dir> flag > walking up from the current
directory for an hr.toml (like git finds .git, so a cloned vault works as
soon as you cd into it) > $HR_VAULT > the vault line of ~/.hr/.hrrc, your
default for everywhere else. hr vault prints the result.

Nothing hr runs changes which vault another process uses: hr never
writes .hrrc, and hr init never selects the vault it makes. To use
another vault, prefix one command (HR_VAULT=~/other hr list) or pass
-C. A scratch hr, like a test, sets $HR_HOME: that moves .hrrc and the
state together.

hr doctor checks a vault's hr.toml and feed directories for inconsistencies
(malformed sidecars, orphaned files, tombstone contradictions, id
collisions, undeclared feed dirs) without modifying anything.

-------------------------------------------------------------------------------
YOUR SETTINGS AND THE VAULT'S
-------------------------------------------------------------------------------

ordering, autoread, showread, user_agent and autocommit can be set in
~/.hr/.hrrc as your defaults for every vault, or in a vault's hr.toml
for that vault alone. A key the vault sets wins, even when it sets
false; a key it leaves out keeps your value:

    # ~/.hr/.hrrc
    vault = "~/feed"
    ordering = "desc"
    user_agent = "hrb/0.1"

In hr.toml they must sit above the first [[feeds]]: TOML gives a key below
that header to the feed, and hr doctor flags it. hr only reads .hrrc,
so it can be a link into a dotfiles repo; hr doctor flags a key it
ignores.

-------------------------------------------------------------------------------
STORAGE DESIGN
-------------------------------------------------------------------------------

  - Plain text everywhere. Markdown body, YAML frontmatter, TOML config &
    sidecar, JSON cache. If hr disappears, the vault stays usable in any
    editor.
  - Immutable content, mutable state. Article .md files are written once at
    fetch time and never touched again. Per-article state (read, read_at,
    favorite, tags, alias) lives in a sibling .meta.toml that only hr
    read/unread/fav/alias modifies. Small files, mergeable in git. An
    article whose state is all defaults (unread, not a favorite, nothing
    else) has no sidecar; hr fmt removes any that say only that.
  - No DB lock-in. Migrate the vault by mv. Bulk operations are
    grep/find/fd.
  - Git is the sync layer. Commit the vault, push, pull on another machine;
    hr reads it without any cross-machine state.
  - Raw HTML archive. ~/.hr/<name>/raw/<id>.html keeps the original feed
    body. If a future conversion bug or readability improvement warrants
    reprocessing, the source is local.

-------------------------------------------------------------------------------
KNOWN LIMITATIONS
-------------------------------------------------------------------------------

  - Lossy HTML to markdown conversion. Body goes through
    JohannesKaufmann/html-to-markdown plus Kunde21/markdownfmt/v3
    canonicalization plus our cleanBody (strips leading nav <h1>s). The raw
    HTML archive (see STORAGE DESIGN) is the recovery path.
  - No schema version field. Adding sidecar/frontmatter fields is
    forward-compatible (zero values are fine); removing them silently
    orphans data. Bump schema = N if/when this matters.
  - Partial test coverage. Feed-directory resolution, moves, config
    editing, group filtering, sync, tombstones and sidecar defaults are
    covered; alias toggling still isn't.

-------------------------------------------------------------------------------
FORMATTING
-------------------------------------------------------------------------------

There's no Go library for markdown-aware hard column wrap (checked:
Kunde21/markdownfmt doesn't expose one; yuin/goldmark is a parser toolkit;
muesli/reflow is markdown-blind). So:

  - Canonicalize structure during sync via markdownfmt.Process. This
    normalizes heading style, list markers, link refs, and code fences
    across feeds.
  - No hard wrap on disk. Paragraphs from html-to-markdown stay as produced
    (typically one line per paragraph). This keeps .md files
    source-of-truth and avoids reformatting churn on resyncs.
  - Display wrap in nvim via a markdown filetype autocmd (wrap + linebreak
    + breakindent). User config; not shipped by hr.

Same trade-off nom makes — it doesn't pre-wrap either, just renders nicely
at view time with glamour. hr's "view time" is nvim.

-------------------------------------------------------------------------------
REFRESH RESOLUTION
-------------------------------------------------------------------------------

hr sync is idempotent and never overwrites user-mutable state:

    File                        Created by      Modified by     On re-run
    hr.toml                     hr init         user            untouched
    feeds/.../*.md              first hr sync   user (if they   skipped if
                                                choose)         exists
    feeds/.../*.meta.toml       hr read/fav/    hr read/fav/    untouched;
                                alias, corrupt  alias, user     removed when
                                                                all defaults
    (in ~/.hr/<name>/)
    cache.json                  hr sync         hr sync         overwritten
                                                                each run
    raw/<feed>/<name>.html      first hr sync   —               skipped if
                                                                exists
    err.txt                     hr sync (on     —               appended
                                error)
    history.jsonl               hr opened       —               appended
    bookmarks.jsonl             hr bookmark     —               appended

Implications:

  - You can edit article files freely. Sync won't clobber them. Frontmatter
    additions (summary: "...", custom tags) survive.
  - Sidecar state persists across resyncs. Mark something read, resync 100
    times, still read.
  - Aliases are committed. They live in .meta.toml, which is in the vault,
    which is in git, so aliases sync to other machines. If you want truly
    machine-local aliases, the design needs to move them to
    ~/.hr/<name>/aliases.json. Not done yet.
  - Stale content stays stale. If a feed corrects a published item (typo
    fix, retitling), hr keeps the original on disk — same GUID, same hash,
    same path, skipped. To force a fresh copy, delete the .md and
    .meta.toml and run hr sync. A future hr refresh <path> command would do
    this targeted.
  - Cross-machine merge conflicts can only happen on .meta.toml (everything
    else is either immutable or outside the vault). The files are small and
    line-oriented; most conflicts auto-merge. Real collisions need human
    attention via git.

The bias is stability over freshness: existing files are the source of
truth, sync only adds new ones.

-- impl cache

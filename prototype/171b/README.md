# 171b: live refresh

Functional prototype of task `171b`. It answers one question: can tablotui learn that HEAD or `.tableaux` changed, and hand that change to the Bubble Tea loop without blocking, racing or reloading twice for one commit?

The answer is yes, with the standard library and `git`, and no new dependency. CI does not run this prototype's tests: the directory is a nested module (`go.mod` pins Bubble Tea v1.3.10 and Lip Gloss v1.1.0), which `go test ./...` at the root skips. Run them here.

## Run

```
export PATH=$PATH:/usr/local/go/bin
go test ./...                 # the tests build repositories under t.TempDir()
go run . -C /path/to/repo     # live: the working tree on HEAD, refreshing
go run . --at v1.0            # pinned to the commit v1.0 names, no refresh
go run . --at main --follow   # follows the ref main as it moves
go run . --dump               # load once, print the project data, exit
go run . --dump --chrome      # the same, with what View() draws about itself
```

Long options take two hyphens; a single hyphen is for `-C` only. `-dump` and its kind are refused with status 2 and a message naming the `--` form. The usage text:

```
Usage: 171b [options]

Options:
  -C string    the repository directory (default ".")
  --at string  view the project at this ref and stop refreshing
  --follow     with --at, follow the ref as it moves (default false)
  --dump       load once, print the project data and exit (default false)
  --chrome     with --dump, print what View returns, chrome included (default false)
```

The view takes no keys but `q` and no size, so there are no `--keys` or `--size` options.

## What it shows

One screen. The default `go run . --dump` in this repository prints the project data alone:

```
commit   8fe12c3a63ddc7fb374063deaf4027c312f6e186
digest   b5ae48a9b823 (15 files)
ref      HEAD
mode     live: watching HEAD and .tableaux
```

With `--chrome` the dump is exactly what `View()` returns:

```
tablotui 171b live refresh

commit   8fe12c3a63ddc7fb374063deaf4027c312f6e186
digest   b5ae48a9b823 (15 files)
ref      HEAD
mode     live: watching HEAD and .tableaux
reloads  0
loaded   14:02:26

q quits
```

Chrome here: the title line, the `reloads` and `loaded` lines, the `error` line and the `q quits` hint. Project data: `commit`, `digest`, `ref` and `mode`.

A failed reload keeps the last good lines and adds `error   <message> (showing the last good view)`. `reloads` counts successful reloads after the first load. `--at HEAD` shows `mode     pinned to HEAD: no refresh`.

The `Loader` interface (`source.go`) is the seam for tablo's library call. The stand-in `GitLoader` resolves the commit and digests the `.tableaux` tree: live reads the working-tree files, pinned and following read `git ls-tree -r <commit> -- .tableaux`. The two digests differ by construction (file contents against blob ids), so compare digests within a mode only.

## Question 1: detecting that HEAD moved

**Choice: poll `git rev-parse HEAD --abbrev-ref HEAD` once per second with the standard library.** One process answers both the commit and the branch name (`HEAD` when detached). Git does the lookup, so each case below passes with no case of its own; each has a test against a real repository.

| Case                                           | Test                              |
|------------------------------------------------|-----------------------------------|
| New commit, loose ref file changes             | `TestCommitOnBranch`              |
| Ref in `packed-refs`, then a rewrite of it     | `TestCommitWithPackedRefs`        |
| Checkout of another branch (HEAD changes)      | `TestCheckoutOtherBranch`         |
| Checkout of a branch at the same commit        | `TestCheckoutBranchAtSameCommit`  |
| Detached HEAD, and a commit on it              | `TestDetachedHead`                |
| Linked worktree (`.git` is a file)             | `TestLinkedWorktree`              |
| Submodule (`.git` is a file)                   | `TestSubmodule`                   |
| Repository breaks and heals                    | `TestBrokenRepositoryIsAChange`   |

What each approach misses:

- **Watching files** (fsnotify on `.git/HEAD` and `.git/refs`) must find the right directory first: in a worktree or submodule `.git` is a file naming a gitdir, and the refs live in the common dir. It must handle a ref that moves into `packed-refs` (a rename over the file, so a watch on the file itself is lost) and a new branch directory that appears after the watch starts. fsnotify is not recursive, and its inotify backend is unreliable on WSL, network and some container mounts. It adds a dependency and the `.git` layout knowledge git already holds. It wins on latency and idle cost, which a one-second status screen does not need.
- **Polling with git** misses a change that reverts within one interval, and reports a change up to one interval late. Neither matters here. It misses nothing in the table above.
- **Stat polling of `.git/HEAD` and the ref file** is cheaper than a process but repeats the file-watching gaps.

I add no dependency: the question does not need one.

## Question 2: an uncommitted edit under `.tableaux/`

The watcher also fingerprints the working-tree `.tableaux` by walking it and hashing name, size and modification time (no file reads). `TestUncommittedTableauxEdits` covers a status file edited, a task file added, a task file removed, and an edit that keeps the size and differs only by mtime. It misses an edit that keeps the size and lands inside the file system's timestamp granularity (a few milliseconds on ext4); a content hash would close that gap at the cost of reading every file each poll.

**Question for the design gate: does the live view show the working tree or HEAD?** The prototype shows the working tree: a person editing a status file sees the tableau change at once, which is what refresh is for, and `--at HEAD` gives the committed view. The cost is that the view then describes no commit, and the README "History" (events derived from the Git log, "the history is that of the branch in view") bounds only committed work, so the history pane lags the grid until the commit. The alternative, HEAD only, is consistent with history but ignores a file saved in an editor. The `Source.Worktree` field switches between them.

## Question 3: delivery into Bubble Tea

- `watchCmd` is a `tea.Cmd` that calls `Watcher.Next` and returns one `ChangedMsg`. Bubble Tea runs each Cmd in its own goroutine, so the loop never blocks. `Update` re-arms it on every `ChangedMsg`, so one waiter exists at a time and it starts from the fingerprint it just reported.
- A burst collapses in `Next`: after the first difference it waits `Settle` (250 ms; tests use 100 to 400 ms) and re-reads until two reads agree. `TestBurstCollapses` writes five files 20 ms apart and sees one result containing all of them, then none.
- A reload runs in its own Cmd. Each carries a sequence number, and `Update` drops a `LoadedMsg` older than the newest request, so out-of-order loads cannot show an old view.
- A failed load sets `err` and keeps the view (`TestFailedReloadKeepsLastGoodView`). A git error is part of the fingerprint, so a broken repository becomes a change, reloads, shows the error, and heals the same way.
- Quit: the context passed to `Next` cancels when the program ends; a cancelled waiter returns `nil`, which Bubble Tea ignores.
- `go test -race` passes. Model tests drive `Update` with messages and never start a `tea.Program`; watcher tests use a 15 s timeout.

## Question 4: `--at <ref>`

The view pins to the commit the ref resolves to, read once at start through `rev-parse --verify <ref>^{commit}`, and the watcher does not run. Proposal: **pinned means no refresh by default**, since a tag or commit id never moves and a pinned view that changes under the reader is not pinned. `--follow` (a prototype flag) instead watches the ref and reloads from the committed tree when the ref moves, with the working tree ignored; this suits `--at main`. The design gate decides whether the follow behaviour ships and under what name, or whether `--at <branch>` follows by itself and only tags and commit ids pin.

## Cost on a large repository

Per poll: one `git rev-parse` process (about 2.5 ms here) plus one `stat` per entry under `.tableaux`. `rev-parse HEAD` reads HEAD and one ref (or `packed-refs`); it reads neither the index nor the work tree, so it does not grow with the file count. A very large `packed-refs` makes the lookup cost more; I did not measure on such a repository. The stat walk grows with the plan's size (74 entries in the umbrella's `.tableaux`), not with the repository. Idle cost is about one process per second. A reload costs what tablo's load costs, once per settled change. The stand-in reads every task file in live mode.

## Leaves out

Rendering any view, tablo's library call, the selection and panes, a terminal resize beyond the width, key handling beyond `q`, and a refresh of the history pane. The timings (1 s, 250 ms) are not tuned.

## For the design gate

1. Working tree or HEAD in the live view (Question 2).
2. Whether `--at` pins or follows, and the flag for the other (Question 4).
3. Polling interval and settle time; whether to add fsnotify later to cut idle cost, keeping the poll as the fallback.
4. Charm major: the prototype uses v1 only and learned nothing about v2.

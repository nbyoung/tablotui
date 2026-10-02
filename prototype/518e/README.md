# 518e: actions prototype

## Question

Does the two-step shape of tabloio's write commands (compose a `Commit`, show it, apply it after confirmation) fit a Bubble Tea model? A key on the selected task opens an action. The model shows the exact commit message, trailers and file diff. `y` or Enter applies it; `n` or Esc cancels and changes nothing.

Answer: yes. Both git steps run in a `tea.Cmd`, never in `Update`, and the tests drive the whole flow through `Update` against a throwaway repository.

## Run

```
cd prototype/518e
go run . --repo /path/to/repo [--model claude-sonnet-5-5 --name "Claude Sonnet 5.5"]
go test ./...
go test -v -run Sample ./...     # the rendered sample below
```

Keys: `j`/`k` select, `r` record, `v` review, `a` reaffirm, `u` authorise, `q` quit. In the form: Tab and Shift+Tab move, Left and Right choose, Enter shows the commit, Esc cancels. In the preview: `y` or Enter applies, `n` or Esc cancels. An error shows until Enter.

## Files

- `write.go`: verbatim copy of tabloio's `prototype/b618/write.go` at tabloio commit `f502ddd` (only the package clause changes, from `b618` to `main`; a header comment says so). Its logic is untouched; tabloio is unchanged.
- `main.go`: the model, the two commands and the views.
- `main_test.go`: the tests. They build repositories under `t.TempDir()` and read nothing else.
- Stand-in: a plain list of the tasks and their status, read from `.tableaux/` as text. The real interface takes tasks, gates, states and reasons from tablo's view data (the gate definition view). The gate, state and reason choices come from `gates.yaml` here, which is the only reason the stand-in reads it.

## What it shows

Flow: `Update(key)` on the list opens a form (record, review) or composes at once (reaffirm, authorise). Enter in the form returns `composeCmd`, which calls `Record`, `Review`, `Reaffirm` or `Authorise` and then `Show` (`Show` runs `git diff --no-index`, so it blocks too). The `composedMsg` moves the model to the preview. `y` returns `applyCmd` and sets the mode to "applying"; the `appliedMsg` returns to the list, reloads the stand-in data and reports the commit, or shows the error. A test checks that after `y` the log is unchanged until the command runs.

The tests check, with `git log`, `git status --porcelain` and `git interpret-trailers --parse`, that:

- a confirmed record makes a commit whose parsed trailers equal those shown (`Reviewed:` above `Model:` and `Co-Authored-By:`), the file holds the new text and the comment stays;
- Enter confirms as `y` does; review, reaffirm and authorise make their empty commits with `Reviewed: b618 function`, `Reaffirmed:` and `Authorised:`;
- `n`, Esc in the preview and Esc in the form leave the log unchanged and `git status` clean;
- an unknown gate (refused by `Record`/`Review` before any write) shows the error.

### Input for recording (question 2)

The smallest input that works: five fields. Gate, state and reason are choices (Left/Right) among the keys in `gates.yaml` (reason adds "none"); the note is a Bubbles `textinput`; a "self-review" toggle sets `Status.SelfReview`, which adds `Reviewed: <id> <gate>`. The form starts at the task's current gate, state and reason, and feeds `Status{ID, Gate, State, Reason, Note, SelfReview}` to `Record`. Review needs only the gate. Reaffirm and authorise need no input. The note stays a single line; `Record` quotes it.

### Failure (question 3)

Tests for three failures confirm the model shows the error, `git status` is clean, the log is unchanged and the status files hold their old bytes:

- no identity (`user.useConfigOnly` with empty identity variables): `git commit: ... empty ident name`;
- a `commit-msg` hook that exits 1: the hook's text appears in the error;
- nothing to commit (record the status as it stands): `nothing to commit`.

Finding: `Apply` writes and stages the files before `git commit`, and on a failure it returns the error and leaves the changed, staged files behind. A bare call would leave the repository dirty. The prototype wraps it: `applyCmd` reads each file first, and on error restores the bytes (or removes a file that did not exist) and runs `git reset -q -- <path>`. A fifth test covers the removal of a new file. The design gate should move this into the exported package: `Apply` should restore on failure, or compose in a way that makes a failed commit leave nothing.

Rendered sample (form, then preview of the record, from `TestSample`):

```
> b618  Leaf one                 defined/nominal

record task b618
> gate         < defined >
  state        < nominal >
  reason       < none >
  note         note (optional)
  self-review  < no >

--- commit message ---
Record task b618 at the function gate

Model: claude-sonnet-5-5
Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
--- .tableaux/status/b618.yaml ---
@@ -1,3 +1,3 @@
 # kept
-gate: defined
+gate: function
 state: nominal

Apply this commit? y/Enter apply, n/Esc cancel
```

## Checks the prototype skips (question 4)

README "Roles" says nothing prevents a commit from another hand: roles are audit-only, and the audit reports where files and history disagree. The interface may still decline to offer an action, and the layers split as follows. The prototype offers every action on every task and implements none of these.

| Check | Belongs in |
|---|---|
| Who may record or reaffirm: the contributor at the junction (the assignee when none is stated) | The interface hides or dims the key, from the task and junction data in the view; the write command warns; the audit reports a mismatch |
| Who may review: the junction's `reviewer`, else the assignee for an agent contributor | The same split, from the same view data; `Review` must also decline a gate the task does not apply to (a not-applicable junction) |
| Who may authorise: an authority (an ancestor's assignee) or the owner; the root only by the owner | The same split |
| The trunk: authorisation counts only on the trunk | The write command, which knows the repository and its branch; the interface only shows the branch |
| The status may not pass a gate before review; whether a gate applies | Tablo's view data tells the interface (the task's next gate and reviewer); the write command refuses an inconsistent record |
| Reaffirm or review of a task with no status, or already at or past the gate | Open in b618's README; the interface can hide the key from the same view data |
| A task that is not on screen or does not exist | The write command (it checks; the prototype shows the error) |

The interface should check only what the view data already carries, and only to avoid offering what would obviously be wrong. The write command and tablo own the rules, because the Git command line and tableaud reach the same commits.

Identity: the commit takes its author and committer from the caller's Git configuration and environment, since `Apply` adds nothing. `main` passes no environment, so a person's own identity signs a commit made in the interface; `--model` and `--name` add `Model:` and `Co-Authored-By:` only when an agent runs it. The tests set the author and committer through environment variables on the git commands, for the throwaway repository only. The interface must not set an identity itself, since the role checks read it.

## Leaves out

The grid and panes, tablo view data, the role checks above, multi-task selection (the functions accept several ids; the keys act on one), the trunk check, editing a task file, a multi-line note, `Propose`, scrolling a long diff (a viewport would hold it), colour choices beyond plus and minus, and the commit's confirmation when `git` is missing.

CI does not run this prototype's tests: it is a nested module that the root `go vet ./...` and `go test ./...` skip.

## For the design gate

- **Importing tabloio or running its binary.** Once tabloio releases, tablotui reaches the write commands one of two ways. Import: typed `Commit` data, so the model can show the message and diff and apply them in two steps with no parsing, and the failure is a Go error; it adds a require on tabloio (and, through it, its dependencies) and ties tablotui to tabloio's release cadence and Go API. Binary: tablotui and tabloio stay decoupled and the user's installed `tabloio` is the one that ran, matching the "run the tabloio write commands" wording of the README and the task; but the two-step flow needs a `tabloio ... --dry-run` that prints the commit, the interface must parse or display that text, and errors arrive as exit codes and text. This prototype needs the typed, composed `Commit`, so it favours the import, and a `--dry-run` that prints what `Show` prints would serve the binary route. The owner decides.
- Whether `Apply` restores the repository on failure (see above).
- The keys (`r`, `v`, `a`, `u`) and the form; the layout of the form and the preview in the panes (a pop-up, a pane or a full screen); a viewport for long diffs.
- Which role checks the interface makes (table above) and how it shows an action it hides.
- Whether `SelfReview` is a toggle or set from the roles; whether the interface takes `Model:` from flags, the environment or configuration.
- Charm versions: Bubble Tea v1.3.10, Lip Gloss v1.1.0 and Bubbles v1.0.0 resolve and work together. `textinput` needs `Cursor.SetMode(cursor.CursorStatic)` for a test to drive `Update` without a blink timer command. Nothing in the flow needs the v2 modules; the prototype does not test them.

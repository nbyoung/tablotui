# Design 0afa: Detail panes

Four panes stand beside the grid and follow its selection: the task definition, the work queue of the person at the keyboard, the work-blockage tree and the history. Each is one instance of one pane type that draws a scrolling document of lines; a builder per view turns tablo's data into those lines in the words tabloio's renderers already fix (D6). The other choices that shape the task: no pane stands open on arrival, and `1` to `4` open, focus and close the panes as global keys (D1, D2); one layout function places the open panes: to the right of the grid from 120 columns, beneath it from 12 body lines, and one pane at a time below that, which is also what `z` gives at any size (D3, D4); a pane opens at the level tablo resolves for the viewer's role, and `d` and `p` change it (D7); `enter` on a line that names a task moves the grid's selection there, through the one message this design adds to the frame (D5); the queue draws no brief and the history takes no range (D8, D9).

The drafts under [`0afa/`](0afa/) fix the data and the bytes; each lands under `internal/ui/detail/testdata/` with its name unchanged, but `model.md`, which the implementation turns into code.

| Draft | Holds |
|-------|-------|
| [`model.md`](0afa/model.md) | Every Go declaration, the three functions the golden files bind (`wrapLine`, `foldRuns`, `layout`), the grid's `goTo`, and the lines each pane draws at each level |
| Sixteen `*.json` files, two of them in [`kinds/`](0afa/kinds/) and [`empty/`](0afa/empty/) | The fixtures, in tabloio's input types: the Tableaux tooling plan at `3cdae52`, the commit the grid's fixture draws. [T1](#tests) lists them |
| Twenty-one document files, `<view>-<task>.<level>.txt` and the like | The golden documents: a pane's title and every row of its lines at one level and one width |
| Seven `*-<w>x<h>.txt` files | The golden screens: the grid and the panes at a size after a key sequence |
| [`layout.txt`](0afa/layout.txt) | Thirteen cases of the layout: a body size, the open panes, the focus, and the rectangles |

The six task fixtures are tabloio's own, from `internal/render/testdata/task/` at `5449b0e`: each as it stands but for `params.person`, and each again cut to the detail level. The ten others are hand-made from `queue.md`, `blockage.md`, `history.md` and tabloio's drafts. None is output of tablo.

## The model

### Packages

| Path | Holds |
|------|-------|
| `internal/view/list/` (new) | The data of the four views: `Task`, `Queue`, `Blockage`, `History`, their shared `Head` with the legend, `Level`, and `Decode` |
| `internal/source/lists.go` (new) | `ListRequest`, the `Lists` interface, `ErrNoPerson`, and the four methods that make `source.File` a `Lists` |
| `internal/ui/detail/` (new) | The pane, the four builders, the commands for `1` to `4` and `z`, and the layout |
| `internal/ui/messages.go`, `internal/ui/grid/grid.go` (changed) | `ui.GotoMsg` and the grid's answer to it: the one addition to the code of `679b` (D5) |
| `cmd/tablotui/main.go` (changed) | Hands the panes, the commands and the layout to the frame |

`detail` imports `ui`, `source` and `list`, and not `grid`: the layout takes the first open pane for the home pane. [`model.md`](0afa/model.md) declares everything; this section states what the declarations do.

### The data and the source

```go
package source

// ListRequest names one list-shaped view and the parameters that focus it.
// It names no ref, no person and no role: the source holds them.
type ListRequest struct {
	View  string // "task", "queue", "blockage" or "history"
	Task  string // the task in focus; "" on the queue
	Level string // "detail" or "provenance"; "" is the level the viewer's role opens the view at
}

// Lists loads the list-shaped views. A call blocks; a pane runs it in a command.
type Lists interface {
	Task(ctx context.Context, r ListRequest) (list.Task, error)
	Queue(ctx context.Context, r ListRequest) (list.Queue, error)
	Blockage(ctx context.Context, r ListRequest) (list.Blockage, error)
	History(ctx context.Context, r ListRequest) (list.History, error)
}

// ErrNoPerson is the error Queue returns when the viewer has no email.
var ErrNoPerson = errors.New("the work queue needs a person, and the viewer states none")
```

The types of `list` are tabloio's input types for the same four views, tag for tag (D10): the data arrives from one tablo, so the two front ends read one contract, and each data object carries its `level`, its `params` and the `legend` from which a pane takes every symbol. A pane holds no table of symbols. `source.File` serves a request from `<view>`, then `-<task>` when the request names one, then `-provenance` when it asks for that level, then `.json`: `task-e9c6.json`, `task-e9c6-provenance.json`, `queue.json`.

The request is small because the source holds the rest. The tablo adapter at the integrate gate derives each view from the snapshot the frame stores, which fixes the ref (679b decision 11), and it knows the viewer and the role, so a queue with no person answers for the viewer and a request with no level answers at the level the role opens the view at (A2, A11).

### The keys that open a pane, and zoom

The frame hands a key to a pane only while that pane has the focus, after its own keys and the commands. A key that must work from the grid is therefore a `ui.Command`, and `detail.New` returns five with the panes:

```go
package detail

// Options configures the panes.
type Options struct{ Source source.Lists }

// Set is what the command hands the frame: the panes after the home pane,
// the commands and the layout.
type Set struct {
	Panes    []ui.Pane    // task, queue, blockage, history, in that order, all closed
	Commands []ui.Command // 1, 2, 3, 4 and z
	Layout   ui.Layout
}

// New builds the four panes, closed, with their commands and their layout.
func New(o Options) Set
```

| Key | Help | Does |
|-----|------|------|
| `1` | `task pane` | Sends the unexported `toggleMsg{pane}`, which the frame broadcasts and the one pane answers. A closed pane opens with the focus (`ui.FocusMsg`) and asks for its data. An open pane without the focus takes it. The pane that has the focus closes (`ui.ShowPaneMsg`), and the frame returns the focus to the grid |
| `2`, `3`, `4` | `queue pane`, `blockage pane`, `history pane` | The same, for their pane |
| `z` | `zoom` | Turns zoom on or off and says so: `zoom on: the focused pane takes the body`, `zoom off` |

So `1` shows the task, `esc` returns to the grid with the pane still open and following, `1` again returns to the pane to scroll it, and `1` once more closes it. `tab` and `shift+tab` cycle the open panes as the frame already does. A pane keeps its own `open` flag, set and cleared as it sends those two messages, because a closed pane must ask for nothing (A13).

Zoom is state that `ui.Layout` cannot see: the frame calls it with the size, the open panes and the focus. `New` therefore makes one `arrangement{zoom bool}`, and the `z` command and the layout share it by pointer (D4). The command changes it inside `Update`, and Bubble Tea v2.0.10 calls `View` after `Update` on the same goroutine (`tea.go`, the event loop), so nothing races. Two frames built from two calls of `New` share nothing; a test that copies a `ui.Model` shares the zoom between the copies, and builds a fresh `Set` instead.

### The layout

`arrangement.layout(w, h, open, focus)` returns rectangles that tile the body, the terminal less its three lines of chrome. `open[0]` is the grid and the rest are the detail panes in the order they opened, which is also the order `tab` visits them. The whole function stands in [`model.md`](0afa/model.md#the-layout); [`layout.txt`](0afa/layout.txt) fixes thirteen cases.

| Form | When | The grid | The panes |
|------|------|----------|-----------|
| grid | No detail pane is open | The whole body | — |
| side | A pane is open and the body is at least 120 columns wide | The left, `w − dw` columns | A region on the right, `dw = w/3` held between 40 and 80 columns, the full height |
| stack | A pane is open, the body is narrower than 120 columns and at least 12 lines high | The top, `h − h/2` lines | A region beneath, `h/2` lines, the full width |
| single | Zoom is on; or a pane is open and the body is narrower than 120 columns and lower than 12 lines | The whole body while it has the focus, else nothing | The focused pane takes the whole body |

A region of `rw` by `rh` cells holds `cols = max(1, rw/40)` panes across and `rows = max(1, rh/5)` down, so no pane in a split is narrower than 40 columns or lower than 5 lines. It shows `k = min(n, cols × rows)` of the `n` open panes: the last `k` to open, moved by the least that includes the focused pane. They fill `⌈k/cols⌉` rows in order, `cols` to a row; the panes of a row share its width and the rows share the height, the odd cells going to the first. An open pane that the region does not show gets no rectangle, draws nothing and still follows.

So an 80×24 terminal draws the grid in 11 lines over one pane of 80×10, or two of 40×10, or four of 40×5; 120×24 draws the grid in 80 columns beside a column of 40 with up to four panes; 160×45 gives the grid 107 columns and the panes 53; and the 40×8 floor draws one pane in 40×5. The grid tolerates every width it gets: below its natural width it narrows the task column, then scrolls its gate columns (679b). The arrival is the grid alone, exactly as `679b` draws it (D2), and the grid keeps every key it has.

### Following the selection

Each pane receives every `SelectionMsg` and `ReloadMsg` and keeps the latest selection, open or closed. After every message an open pane derives its request and asks for it unless something already answers it:

| Pane | The request for a selection of task T | A move to another gate cell | No row |
|------|----------------------------------------|-----------------------------|--------|
| Task | `{task, T, level}` | No request: the task is the same | No request; the pane draws `no task selected` |
| Queue | `{queue, "", level}`, once on opening | No request | The same request |
| Blockage | `{blockage, T, level}`: the causes that hold T or lie in its subtree (A7) | No request | As the task pane |
| History | `{history, T, level}`: the events of T, replayed from its children for a parent (A8) | No request | As the task pane |

1. **When a pane asks.** It asks when it is open and the request exists, and always after a `ReloadMsg`. Otherwise it does not ask when the load in flight is for this very request; nor when no load is in flight and the last answer serves: either it failed for this very request, or its data is for the same task and holds the level wanted (no level chosen, or `data.Level` at least the chosen one, or the data answers this very `Level`). Each ask takes the next sequence number and runs through `Context.Load(id, seq, f)`. A `LoadedMsg` whose `Seq` is not the newest changes nothing.
2. **What a selection costs.** A move of the row cursor costs one load per open pane that follows the task, at most three. A move of the column cursor costs none. The queue follows the viewer, not the row: a selection costs it no load, and it only moves the mark `>` to the items that name the selected task. A closed pane costs nothing; it asks when it opens. No pane debounces, since an older answer drops by its sequence number and a load is a derivation in memory (A5).
3. **While a load is in flight** the pane draws the data it has, under the title of that data, and the title ends ` · loading…`. With no data it draws `loading…`.
4. **After a load fails.** When the pane holds data for the same task, as after a reload or a change of level, it keeps the view and sends `NoticeMsg{"<pane>: <message> (showing the last good view)", Err: true}`, as the grid does. Otherwise the data in hand belongs to another task, so the pane drops it and draws `error: <message>` in `Styles.Error`; it asks again when the selection changes or a `ReloadMsg` arrives. `ErrNoPerson` draws its sentence, `The work queue needs a person, and the viewer states none.`, in plain text.
5. **A `ReloadMsg`** makes an open pane ask its request again at once, also while a load is in flight, whose answer then drops; a closed pane asks when it next opens.
6. **The cursor after an answer.** For the same task the cursor stays on the first line of the item it stood on, and arrives afresh when that item left. For another task it arrives: on the first row, and on the last row for the history, which lists the oldest event first as VIEWS.md orders it and so shows the newest (D9).

### Levels

A pane draws at `min(the level the data states, the level the viewer chose, the deepest the pane draws)`. Until the viewer chooses, the request carries no level and the pane shows what arrives, so the role decides, in tablo, as VIEWS.md has it. `d` shows detail from glance and glance from anything else; `p` shows provenance, and detail from provenance: the grid's two keys with the grid's meaning. A choice holds for that pane through every later selection. A request with a chosen level asks for `provenance` from the first time the viewer opens it in that pane, else for `detail`, and never for glance: the levels nest, so the pane draws glance from detail data and `d` costs no load. The queue's deepest level is detail; `p` there answers `the work queue's provenance is the brief of one item, which this pane does not draw` (D8).

### A pane on screen

`View(w, h)` returns `h` lines of `w` cells: the title rule, then `h − 1` rows behind a gutter.

```
┌─ 1 Task e9c6 · detail ─────────────────────────────── 1–12 of 55 ─
│ e9c6 Abstract views
│ assignee     noreply@anthropic.com
```

- **The title** reads `<digit> <Name>`; with data `<digit> <Name> <subject>`, then the count and the level, joined by ` · `: `2 Queue ada@example.org · 60 items · detail`, `3 Blockage 437e · 2 causes · glance`, `4 History e9c6 · 8 events · detail`; then ` · loading…` while a load is in flight. It draws in `Styles.Title`, and in `Styles.Cursor` while the pane has the focus.
- **The title rule** is `┌─ `, the title, a space, `─` to the width, and, when the rows outnumber the pane's, ` <a>–<b> of <n> ─` at its end. Where the range does not fit beside the title it goes; where the title still does not fit, `Measure.Fit` cuts it with `…`.
- **A row** is `│ ` and the text fitted to `w − 2` cells; a pane narrower than four columns draws blank lines. A heading draws in `Styles.Header`, the cursor row in `Styles.Cursor` while the pane has the focus, and the error line in `Styles.Error`. No fact rests on colour: the status line names the focused pane, and the range stands in the title.
- **The wrap.** A line breaks at spaces into rows; a continuation row stands `Hang` cells further in, so a tree keeps its shape (the prototype's seventh open point). A label pads to `Hang` cells and the text follows it. A word wider than a row breaks at the row's width, so a 40-character hash survives in a pane of 40 columns. The indent never takes more than a quarter of the width and the hang never more than half.
- **The cursor** is one row. `up` `k` and `down` `j` move it by a row, `pgup` and `pgdown` by the pane's rows, `home` `g` and `end` `G` to the first and the last; the rows scroll by the least that keeps it in view. The pane stores the cursor and the first row as a line and a row within it, so a resize, which reaches no pane, cannot strand them: `View` settles a copy for the size the frame gives it and `Update` stores what it settles, through one function.
- **The other keys.** `enter` selects the cursor line's task in the grid (below). `f` unfolds every run fold of the pane, and folds them again.
- **The status line**, while the pane has the focus: `<pane>[ <subject> · <level>] · lines <a>–<b> of <n>[ · enter selects <id>[ × <gate>]]`, and `lines 0 of 0` with no row.
- **The help line**: `↑↓ line`, `enter select task`, `d detail`, `p provenance`, `f fold`. The frame's help line lists the focused pane's keys alone, so `1` to `4` and `z` show in the help mode under Commands, as the keys of `518e` and `f394` will (D11).

### The lines

A builder per view returns the title's subject and count and `[]Line`; [`model.md`](0afa/model.md#the-lines-each-pane-draws) fixes every line at every level, and the golden documents fix the bytes. The rules that hold across the four:

1. **tabloio's words.** A gate, a state and a reason print as symbol, space, key (e3ed R2); the status phrase, a task, a count, a requirement's edge and its condition follow R3, R4, R8 and R16; the sentences of a cause, a held task, an effect and a model reading are those of `a3cc` and `9167`.
2. **A table row is a line.** What the Markdown draws as one row of a table, a pane draws as one line: the cells that are not empty, joined by ` · `. An absent value leaves no cell (R7). A table of fields and values is a column of labels.
3. **The run fold.** A run of more than eight consecutive entries alike but for the task shows three and one line `… and <n> more: <ids>`, as the owner ruled for every front end. It applies where tabloio applies it: the queue's items, and the task's Requires and Dependents.
4. **A line names a task** where it is about one: a queue item and everything beneath it, a cause and each task it holds, a history line, a requirement, a child, a junction of the task itself with its gate.
5. **The empty forms** are tabloio's sentences: `The queue is empty.`, `No cause holds any task.`, `No event in view.`; an empty section reads `Requires: none`.

### The grid follows a pane

One selection exists, the grid's, and `518e` acts on it. A pane never sends a `SelectionMsg`. `enter` on a line that names a task sends `ui.GotoMsg{Task, Gate}`; on any other line it says `this line names no task`. The grid answers: it unfolds the task's ancestors, selects the row, moves the column cursor to the gate when the gate shows as a column of its own, and announces the selection as after any key; for a task its tableau lacks it says `<id> is not in the tableau in view`. The focus stays in the pane, so the viewer walks the queue with `down` and `enter` while the other panes follow. This is the smallest addition that does it: `grid.StartMsg` would also reset the folds, the level and the column cursor (D5).

### The command, determinism, the prototype

`cmd/tablotui` builds one `source.File`, hands it to the grid and to `detail.New`, and passes `append([]ui.Pane{grid}, set.Panes...)`, `set.Commands` and `set.Layout` to `ui.Options`; `--fixture`'s directory now also holds the list files. `View` is a function of the model and the shared zoom alone: no clock, no map order, no terminal query.

From prototype `0afa` the design keeps the four panes that follow, the queue that follows the person and marks the selection, the title with the scroll range, zoom, the digits, and clipping by `x/ansi` and not by Lip Gloss. It drops the Lip Gloss borders, which cost two columns and two lines a pane, the 2×2 grid beside a list, `p` cycling the person (that is `f394`'s), `--as`, `--dump`, `--keys`, `--size`, `--chrome` and `--strip-vs16`, and the newest-first history.

## Conformance to the texts

| Pane | VIEWS.md section | Mockup | tabloio design | glance | detail | provenance |
|------|------------------|--------|----------------|--------|--------|------------|
| Task | [Task definition](https://github.com/nbyoung/tableaux/blob/main/VIEWS.md#task-definition) | `task.md` | [`e3ed`](https://github.com/nbyoung/tabloio/blob/main/design/e3ed.md) | The task and its fields | Description, place in the tree, requires, dependents, every junction, the snapshot | The two commits, roll-up, linkages, sources, reviews, models, newest events, commands |
| Queue | [Contributor work queue](https://github.com/nbyoung/tableaux/blob/main/VIEWS.md#contributor-work-queue) | `queue.md` | [`a3cc`](https://github.com/nbyoung/tabloio/blob/main/design/a3cc.md) | One line per item, under its kind | Each item's gate, junction, references, requirements and status | None: the brief (D8) |
| Blockage | [Work-blockage tree](https://github.com/nbyoung/tableaux/blob/main/VIEWS.md#work-blockage-tree) | `blockage.md` | `a3cc` | One line per cause | The tree, then what is not yet due | The facts and the commands of each cause |
| History | [History](https://github.com/nbyoung/tableaux/blob/main/VIEWS.md#history) | `history.md` | [`9167`](https://github.com/nbyoung/tabloio/blob/main/design/9167.md) | One line per commit and task | The status after, note, effect, model, committer, subproject events | The commit's hash, subject, actors, trailers, files, commands |

- **Parameters.** The queue's person is the viewer, as the owner ruled; the other three name nobody and mark no one. Every pane lists every gate its data names and takes no window. The ref is the snapshot's.
- **Levels.** VIEWS.md lets the terminal fold and unfold and lets the role name the level a view opens at: the role does, through tablo, and `d` and `p` fold and unfold.
- **A pane shows what the Markdown does not:** the cursor and the task `enter` selects; the mark `>` on the queue items that name the grid's selection; the scroll range; the loading and error forms; a fold that a key opens.
- **The Markdown shows what a pane does not:** the page frame (the question, the parameter line, the legend line, See also, the closing command), since the grid's context line names the project and the ref; the count lines of the queue, the tree and the history, since the title carries the total and a kind with no item prints nothing; the key line under the junctions; links; a person in bold; the queue's `Waits for:` line, whose cause stands in the item's own line, and its command lines, whose actions are the keys of `518e`; the brief; the history's day table, day headings and day fold, which is tabloio's own rule (9167 decision 1), and the task of a line where it is the subject.
- **One sentence differs.** `a3cc` writes `Next: <n> requirements are not yet due, <m> of them unmet.`, which reads wrongly for one; the pane writes `Next: 1 requirement not yet due, 0 unmet.`
- **Stale against a ruling.** `queue.md` folds from five alike rows; the ruling is more than eight, and the mockup stays as drawn.
- **Where the texts are silent.** VIEWS.md defines no form for a view with no subject, a load in flight or a load that failed; the panes take `loading…` and `error: <message>` from 679b decision 9 and add `no task selected`. It does not say whether the task parameter of the tree, given a leaf, means the causes that hold it or the causes inside it; the pane draws what tablo sends (A7).

## Assumed interfaces

What other designs state about this task:

| # | Design and row | Reading |
|---|----------------|---------|
| C1 | `679b`, "What the later tasks may assume", the row of `0afa` | Amended twice. The task adds panes and a `Layout` through `Options`, and also five `Commands`, because `1` to `4` and `z` must work while the grid has the focus. It changes no line of `frame.go`, and adds `ui.GotoMsg` to `messages.go` and one case to the grid (D5). The rest stands |
| C2 | `679b` A9 and decision 11; `171b` A4 and decision 7 | Confirmed: on `ReloadMsg` each open pane asks its request again, and the source derives it from the stored snapshot |
| C3 | `679b` decision 13 | Confirmed: `Context.Width` and `Height` let `Update` settle the cursor while no pane stores a width |
| C4 | `171b`, the paragraph on `--ref`: "a range belongs to the history pane (0afa)" | Amended: the history pane takes no range in this task (D9) |
| C5 | tabloio `e3ed` decision 3: the one wording "binds … tablotui's panes if the owner wants the three front ends to agree" | Confirmed: the panes take it (D6) |
| C6 | tabloio `e3ed` decision 8, and VIEWS.md's ruling on the long list | Confirmed: more than eight alike, three shown, every id; `f` unfolds |
| C7 | tabloio `e4c7` decision 2: one `brief` object from tablo, read by tablotui's `0afa` | Amended: no pane reads it in this task (D8) |
| C8 | tabloio `5ca9` decision 3: the default person | Confirmed: the viewer on the queue, nobody on the other three |
| C9 | tabloio `a3cc` decision 3 and `9167` decision 2: the front end words a cause, a held task, an effect and a model reading from keys | Confirmed, with the same words |

What this design assumes:

| # | Of | Assumption |
|---|----|------------|
| A1 | tablo `4ed9` | `Snapshot.View` returns an envelope whose `Data` the adapter maps into the `list` types at the integrate gate; a `*NotFoundError` or a `*ReadError` reaches the pane as an error a person can read |
| A2 | tablo `4ed9` | With no `Level` and a `Viewer`, a view answers at the level the viewer's roles open it at, and its data states that level. With no person and no viewer the queue is a usage error, which the adapter returns as `source.ErrNoPerson` |
| A3 | tablo `493e`, `886d`, `8ed1` | Every view's data carries `level`, `project`, `ref`, `params` and `legend` (tabloio `e3ed` A2); the legend holds the symbol of every gate, state, reason and mark at the ref, so the old symbols and the missing state symbols of the prototype's fourth gap go; the fifth gap, the ref, is `ref` |
| A4 | tablo `493e`, `886d`, `8ed1` | The levels nest in the data: what a view holds at detail it holds at provenance |
| A5 | tablo `4ed9` | A view derives from the snapshot in memory, each fact once, so three views per move of the row cursor cost no load of the repository. The integrate gate measures it |
| A6 | tablo `493e` | The task data holds the fields of `list.Task` with the values tabloio `e3ed` A8 to A10 list |
| A7 | tablo `886d` | The blockage data holds the fields of `list.Blockage` (tabloio `a3cc` A4), and the task parameter filters it: the causes that hold the task or lie in its subtree, every cause for the root. The prototype's first gap: `886d`'s prototype ignores the parameter |
| A8 | tablo `8ed1` | The history data holds the fields of `list.History` (tabloio `9167` A1, A2): a parent's history replays its children, and a line carries the status after it, the committer where it differs, and the model. The prototype's second and third gaps |
| A9 | tablo `886d` | The queue data holds the fields of `list.QueueItem` (tabloio `a3cc` A5) and states the person it resolved in `params.person` |
| A10 | tablotui `679b`, `171b`, at integrate | The frame stores one snapshot per settled change and sends one `ReloadMsg`; the adapter behind `source.Lists` reads that snapshot, so a list request names no ref |
| A11 | tablotui `f394` | It resolves the viewer and the role, sets them on the source the command builds, and sends `ui.ReloadMsg` after `R` changes the role, so every open pane asks again. It binds `R` and none of `1` to `4`, `z`, `d`, `p`, `f`. The panes read no viewer from the tableau |
| A12 | tablotui `518e` | Its commands bind `r`, `v`, `a` and `u` and act on the frame's latest `SelectionMsg`, which the grid alone sends; they run while a detail pane has the focus. After a write it sends `ui.ReloadMsg`. It needs no message of `detail` |
| A13 | tablotui `f394`, `518e` | Neither opens or closes a detail pane with `ui.FocusMsg` or `ui.ShowPaneMsg`: `1` to `4` are the only way, since each pane tracks whether it is open |
| A14 | tabloio `e4c7` | `internal/brief` stays private (its decision 10), which D8 rests on |

## Tests

Tests drive `Update` and `View` with messages and sizes through the real frame and the real grid, start no terminal, and use no network and no clock. A stub `source.Lists` wraps `source.File`, records each request, and fails on demand.

| # | Proves | How |
|---|--------|-----|
| T1 | The lines of every view at every level, and the wrap | For each golden document: build the pane from its fixture, wrap at the width, and compare the title and the rows. The table below lists them |
| T2 | The screens | For each golden screen: the grid on `../grid/testdata`, the panes on the fixtures, the size and the keys; strip ANSI and trailing spaces; compare |
| T3 | Every line fits | A sample of sizes from 40×8 to 170×46, every size below 44×12, six key scripts, both measures: `View` has height lines, each exactly width cells |
| T4 | The layout tiles | Every body from 40×5 to 260×60, zero to four panes, every focus, zoom on and off: the rectangles lie inside the body, do not overlap, cover it, and include the focus; in a split no pane is under 40×5. Then [`layout.txt`](0afa/layout.txt) |
| T5 | The new symbols | `┌` `│` `─` `›` `→` `–` `…` `·` `>` measure one cell under both methods |
| T6 | The wrap | A table of cases over `wrapLine`: a label, a hang, the two caps, a word wider than the row, an empty text, a symbol at the edge, and `Marked` |
| T7 | The run fold | Eight alike, nine, nine with one that differs in the middle; `f` unfolds and folds again; the fold line carries `>` when its ids hold the selection |
| T8 | Following | Closed panes ask nothing; each pane's request for a row; a move of the column cursor asks nothing; a `ReloadMsg` asks once per open pane, also with a load in flight; a pane closed through a reload asks when it opens; no row draws `no task selected` |
| T9 | The queue | One load however the selection moves; the mark follows the selection; `ErrNoPerson` draws its sentence and asks once; the empty queue |
| T10 | Sequence and failure | With twelve answers held back, the title reads `· loading…` and the eleven older answers change nothing; a failed reload keeps the view and sends its notice; a failed load for another task draws `error:` and drops the old view; a first load that fails |
| T11 | Levels | Data that states glance draws glance; `d` asks `detail` once and then costs nothing; `p` asks `provenance`, and `p` again returns to detail with no load; the choice survives a new selection; `p` on the queue says why not |
| T12 | `1` to `4` and `z` | Open with the focus, `esc`, focus, close; zoom gives the focused pane the body at 100×30 and returns; the two notices; `ui.CheckKeys` passes with stub commands on `R`, `r`, `v`, `a` and `u` |
| T13 | The cursor | Each key; the history arrives on its last row; the cursor stays in view through a resize to 60×12 and back; it stays on its item through `d`, `f` and a reload; the status line in each state |
| T14 | `enter` and `GotoMsg` | From the queue at glance the grid unfolds two ancestors, selects `e9c6` and its `implementation` column, and sends that `SelectionMsg`; the focus stays; a gate in a folded column leaves the column cursor; a task the tableau lacks and a line with no task each give their notice |
| T15 | Colour | The stripped view equals the view under zero `Styles`; the title and the cursor row carry reverse only while the pane has the focus; a heading carries bold |
| T16 | `source.File` as `Lists` | Each name `ListName` gives; a missing file; a file that does not parse names itself; a cancelled context |
| T17 | `list.Decode` | Every fixture decodes; `Decode` ignores an unknown key and refuses an unknown level |
| T18 | The command | With `--fixture` the frame holds five panes and five commands and `CheckKeys` passes |
| T19 | The grid stands | Every test of `679b` passes unchanged with `goTo` in place |
| T20 | Determinism | The same fixture, size and keys give the same bytes twice, from two fresh `Set`s |

| Golden | Fixture | Level, width | Notes |
|--------|---------|--------------|-------|
| `task-e9c6.glance.txt`, `.detail.txt`, `.detail.38.txt`, `.provenance.txt` | `task-e9c6.json`, `-provenance.json` | glance by choice, 78; as stated, 78 and 38; provenance asked, 78 | The run fold of 21 dependents; the label column at the floor's width |
| `task-5fe3.provenance.txt`, `task-595e.provenance.txt` | `task-5fe3-provenance.json`, `task-595e-provenance.json` | 78 | A parent with ten children and a roll-up; recursive junctions, a snapshot and a linkage |
| `queue.glance.txt`, `queue.detail.txt`, `queue.unfolded.txt` | `queue.json`, 60 items | 78 | Two folds; the mark on `e9c6`, and on a fold line for `bb7c`; `f` |
| `queue-kinds.detail.txt`, `queue-empty.txt` | `kinds/queue.json`, `empty/queue.json` | 78 | One item of each kind; the empty form |
| `blockage-437e.glance.txt`, `.detail.txt`, `.provenance.txt`, `blockage-e9c6.detail.txt` | `blockage-437e.json`, `-provenance.json`, `blockage-e9c6.json` | 78 | Two causes, a tree three deep, what is not yet due; no cause |
| `history-e9c6.glance.txt`, `.detail.txt`, `.provenance.txt`, `.provenance.38.txt` | `history-e9c6.json`, `-provenance.json` | 78 and 38 | Seven lines; a full hash broken at 34 cells |
| `history-595e.detail.txt`, `history-5fe3.txt` | `history-595e.json`, `history-5fe3.json` | 78 | A first pin, subproject events, a proposal, an unknown effect key; no event |
| `stack-100x30.txt` | all | `E`, `down` four times, `1` | The task pane beneath the grid |
| `pair-80x24.txt` | all | `E`, `down` four times, `2 4` | Two panes of 40 columns; the history on its last row |
| `side-160x30.txt`, `four-160x45.txt` | all | `E`, `down` four times, `1 2 esc`; `1 2 3 4` | The column on the right, two panes and four |
| `floor-40x8.txt` | all | `E`, `down` four times, `1`, `down` three times | One pane in 40×5 |
| `zoom-100x30.txt` | all | `3 z p` | The tree at provenance over the whole body |
| `goto-120x24.txt` | all | `2 d`, `down` three times, `enter` | The grid unfolded to `e9c6` with its column cursor on 🧱 |

**Tried already.** A trial outside the repository copies `main` at `21084d3`, adds the three packages, `ui.GotoMsg`, the grid's case and the command's wiring as this design declares them, and passes T1 to T5, T8 to T14, T19 and T20 in the form above, with the frame's and the command's own tests, under `go vet`, `golangci-lint` and, for every test but the two sweeps, `go test -race`. The trial wrote every golden file and `layout.txt`. T3 ran 5,760 screens over 480 sizes and T4 about 370,000 layouts. The prototype's own tests ran on Bubble Tea v1.3.10 and Lip Gloss v1.1.0 with borders, at three sizes and over 1 to 140 columns; its code ports and does not copy.

**Not checked.** No terminal emulator ran the trial: how one draws `┌`, `│` and `─` when it draws ambiguous-width characters wide, whether one answers the mode query, and the keys on a keyboard stay with the validate gate, as for the grid. tablo's data, a plan of a thousand rows, a history of a thousand lines, Windows, and the panes beside the code of `f394` and `518e` are untried. The trial does not run T6, T7 and T15 to T18 as written, though the golden files exercise the wrap and the fold, and the trial's command builds with the wiring T18 names.

**Later gates.** The unit gate runs T1 to T20. The integrate gate adds the tablo adapter behind `source.Lists`, proves A1 to A9 on the corpus entry `weather-station` and on the umbrella's plan, replaces the hand-made fixtures by tablo's output, and times a move of the row cursor with four panes open (A5). The validate gate, which the owner reviews, runs in terminals.

## Implementation notes

**Order**, each step building alone:

1. `internal/view/list` with the fixtures (T17).
2. `internal/source/lists.go` (T16).
3. `detail/lines.go` and `words.go` (T5, T6, T7).
4. The four builders, against the golden documents (T1).
5. `detail/pane.go` and `keys.go` (T8 to T11, T13, T15).
6. `detail/detail.go`: `New`, the commands, the layout (T4, T12).
7. `ui.GotoMsg` and `grid.goTo` (T14, T19).
8. `cmd/tablotui/main.go` (T2, T3, T18, T20).

**Files.** New: the files of the three packages as [`model.md`](0afa/model.md) names them, their tests, and `internal/ui/detail/testdata/` from the drafts. Changed: `internal/ui/messages.go`, `internal/ui/grid/grid.go` and a test beside it, `cmd/tablotui/main.go` and its test, and `README.md`: two rows of the layout, `internal/view/list/` and `internal/ui/detail/`. `prototype/0afa/` goes with the commit that records `implementation`. `go.mod` gains nothing: the panes use `key` from Bubbles and not its viewport, whose scroll knows no cursor row.

**The plan and the texts**, not edited here:

- The integrate gate of `0afa` needs tablo's `493e`, `886d` and `8ed1` at implementation and `4ed9`; no cross-project requirement says so, as `679b` found for the grid.
- `518e` requires this task from implementation to design, yet an agent writes its design beside this one; A12 and A13 are what it must meet.
- No task of tablotui draws the gate definition, so the program explains no symbol; `?` could carry the legend, which every list view's data already holds.
- tabloio's fixture `task/5fe3.json` holds the title of the first child alone; the golden file draws the nine others by their ids.

## Decisions at review

1. **`1` to `4` open a pane with the focus, then focus it, then close it,** as commands that work from every pane. The alternative opens a pane without the focus, so that one key shows it and the grid keeps moving; a pane then takes two keys to scroll, and at a size that shows one pane at a time the key seems to do nothing. Binds `518e` and `f394`, whose keys share the commands. Accepted by the owner on 2026-10-07, as stated.
2. **No pane stands open on arrival.** The grid arrives as `679b` draws it, and the open panes do not persist between sessions. The alternative opens the task pane wherever a split fits, or keeps the open set in the settings file. Accepted by the owner on 2026-10-07, as stated.
3. **The layout:** beside the grid from 120 columns, in a third of the width held between 40 and 80; beneath it from 12 body lines, in half the height; one pane at a time below both; a pane of 40×5 at least; a title rule and a gutter in place of a box. The alternative is the prototype's: a narrow list and a 2×2 grid of bordered panes from 90 columns. Accepted by the owner on 2026-10-07, as stated.
4. **Zoom lives beside the frame.** The `z` command and the layout share one flag, because `ui.Layout` sees the size, the open panes and the focus alone. The alternative puts the flag in the frame: a `ZoomMsg` and a field in `ui.Model`, about fifteen lines of `frame.go`, which keeps every bit of layout state in the model. Accepted by the owner on 2026-10-07, as stated.
5. **A pane moves the grid's selection through `ui.GotoMsg`,** which adds one type to the frame's messages and one case and one method to the grid: the only change to the code of `679b`. The first alternative leaves the panes read-only, so the queue cannot lead to a task. The second lets a pane send its own `SelectionMsg`, which gives `518e` a selection the grid does not show. Binds `518e`. Accepted by the owner on 2026-10-07, as stated.
6. **The panes speak tabloio's words** (e3ed decision 3 offers it): its symbol-and-key form, its status phrase, its sentences and its empty forms, with a table row drawn as one line of cells joined by ` · `. The alternative words the panes afresh for narrow columns. Accepted by the owner on 2026-10-07, as stated.
7. **A pane opens at the level the viewer's role opens its view at,** resolved by tablo, and `d` and `p` then set that pane's level for the session. The alternative opens every pane at detail and leaves the role to the grid. Binds `f394`, which supplies the role (A11). Accepted by the owner on 2026-10-07, as stated.
8. **The queue pane draws no brief.** Its levels are glance and detail, and `p` says so. This amends tabloio's `e4c7` decision 2 for this task. The alternative opens the brief of the item under the cursor as a mode, which needs tabloio's `internal/brief` exported, the alternative of its decision 10, or its wording copied here. Accepted by the owner on 2026-10-07, as stated.
9. **The history lists the oldest event first, arrives on its newest, folds nothing and takes no range.** This amends the remark in `171b` that a range belongs to this pane. The alternatives: the prototype's newest-first order, against VIEWS.md; and a range, which needs an input mode and a second parameter on the request. Accepted by the owner on 2026-10-07, as stated.
10. **The view types are tablotui's own copy of tabloio's,** in `internal/view/list`, with the same JSON tags, as tabloio's `e3ed` decision 1 chose for itself. The alternative waits for tablo's Go types and imports them at the integrate gate, which leaves the unit gate without fixtures. Accepted by the owner on 2026-10-07, as stated.
11. **The help line stays the focused pane's.** The frame's help line lists no command, so `1` to `4` and `z`, and later the keys of `518e` and `f394`, show only in the help mode. The alternative changes `frame.go` to put the commands in the help line, where the grid's twelve bindings alone need 144 columns. Binds `518e` and `f394`. Accepted by the owner on 2026-10-07, as stated.

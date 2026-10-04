# Prototype 0afa: detail panes

Task `0afa` Detail panes, gate `function`.

## The question

Do panes for the task definition, the blockage tree, the work queue and the history follow a selection, and does a split layout of a list and bordered, scrolling panes hold in Bubble Tea and Lip Gloss at several terminal sizes, with wide emoji symbols intact?

Answer: yes on all three counts. A test drives `Update` and `View` and checks the width and height of every line at a wide, a narrow and a short size, and over a sweep of 1 to 140 columns.

## Run

```
export PATH=$PATH:/usr/local/go/bin
go run .                                      # the interface
go run . --as ben@example.org                 # another current person
go run . --dump --keys j,j --size 110x26      # print the project data after the keys
go run . --dump --keys j,j --chrome           # print exactly what View() returns
go run . --strip-vs16                         # drop the emoji variation selector
go test ./...
```

Keys: `j`/`k` or arrows move the selection when the list has focus and scroll when a pane has; `pgup`, `pgdown`, `home`, `end` likewise; `tab`/`shift+tab` cycle focus over the list and the four panes; `1`-`4` focus a pane; `esc` returns to the list; `z` zooms the active pane; `p` cycles the current person; `q` quits.

CI does not run this prototype's tests: it is a nested module (`go.mod`, `go.sum`) that the root `go vet ./...` and `go test ./...` skip.

## What it shows

Options use two hyphens; a single hyphen before a long name (`-dump`) is refused with status 2. `--dump` prints the view after `--keys` (comma separated, `comma` names the comma key) at `--size` and exits.

**The default dump** prints the project data: for each pane the layout shows, in layout order (definition, blockage, queue, history; one pane in the single mode), a plain title line that names the view and the task or person, then the pane's whole content wrapped to the width. Chrome, which the default leaves out: the task list (a stand-in with its cursor), borders, the focus mark, scroll offsets and ranges, clipping to the pane height, the key-help line and padding. The `▸` mark in the queue is project data (the item names the selected task) and stays. `--chrome` prints exactly what `View()` returns at `--size`; the exact-size tests apply to that form.

```
Task 3c5d
3c5d Dashboard
assignee dan@example.org
parent   a1c0 Weather station, order 0
status   📝 defined, nominal, 2026-09-22
authorised proposed ()
authorities ada@example.org

A web page on the gateway that charts the last week of readings and
the current values, with the battery state alongside.

requires
  7b2e Gateway, function to integrate, pending: Readings API

references
  Dashboard overview docs/overview.md#dashboard

junctions
  ❔ undefined      🧑 dan@example.org
  📝 defined        🧑 dan@example.org
  📌 mockup         🧑 dan@example.org
  ⚙️ function       🧑 dan@example.org
  ⚡ performance    🧑 dan@example.org
  ⚓ reliability    🧑 dan@example.org
  📐 design         🧑 dan@example.org
  🛠️ implementation 🧑 dan@example.org
  🧩 unit           🧑 dan@example.org
  🖼️ integrate      🧑 dan@example.org
  🌍 validate       🧑 dan@example.org
  🚀 release        🧑 dan@example.org

Blockage 3c5d
3c5d Dashboard is proposed
  ada@example.org resolves, holds 1
  action: commit a trailer: Authorised: 3c5d
  - 3c5d Dashboard at mockup

not yet due
  3c5d requires 7b2e, function to integrate: Readings API

Queue ada@example.org
▸ authorisation owed: 3c5d Dashboard
    proposed; the deciding commit is not by an authority
  work ready: 7b2e Gateway at defined
  reaffirmation: 9f31 Sensor board at function
  work waiting: 9f31 Sensor board at performance
    blocked: Barometer ICs on 14-week backorder

History 3c5d
2026-09-27 1b8cfb1 task
  by dan@example.org
2026-09-22 55f57c1 status defined nominal
  by dan@example.org
2026-09-21 74bc0b8 authorised
  by ada@example.org
2026-09-20 06c6679 task
  by dan@example.org
```

Sample of the `--chrome` form at 110x26 with the selection on `9f31` (the grid stand-in is the list at the left: the ids and titles of `global-tableau.json`, a selection that moves by key):

```
╭─ Tasks ─────────────────╮╭─ Task 9f31 ───────────────────────────╮╭─ Blockage 9f31 ────────────────────────╮
│ a1c0 Weather station    ││9f31 Sensor board                      ││9f31 Sensor board is stalled at         │
│   4e2b Sensor node      ││assignee ada@example.org               ││function: Barometer ICs on 14-week      │
│     9f31 Sensor board   ││parent   4e2b Sensor node, order 1     ││backorder                               │
│     c07d Node firmware  ││status   ⚙️ function, stalled, blocked,││  ada@example.org resolves, holds 2     │
│   7b2e Gateway          ││2026-09-28                             ││  action: clear the blocked reason, or  │
│   3c5d Dashboard        ││note     Barometer ICs on 14-week      ││record a new status                     │
│                         ││backorder                              ││  - 9f31 Sensor board at performance    │
│                         ││authorised authorised (merge)          ││  - c07d Node firmware at implementation│
│                         ││authorities ben@example.org,           ││                                        │
│                         ││ada@example.org                        ││9f31 Sensor board has not passed design │
│                         │╰───────────────────────────── 1-10/44 ─╯╰────────────────────────────── 1-10/13 ─╯
│                         │╭─ Queue ada@example.org ───────────────╮╭─ History 9f31 ─────────────────────────╮
```

1. **Panes follow the selection.** Moving the selection reloads the definition, the blockage tree and the history for the new task and resets their scroll. The test moves from `9f31` to `c07d` and asserts the first task's text is gone. The queue does not reload: it marks (`▸`) the items that name the selected task.
2. **Layout.** One list box and four panes, each a Lip Gloss rounded border with the title in the top border and the scroll range (`1-10/44`) in the bottom one. Width and height are exact in all three modes (`TestSplitLayoutExactSize`, `TestEverySize`): grid at 120x40, stack at 50x30, a single pane at 80x12, and a "Terminal too small" screen below 40x8, each padded to the full size.
3. **Focus.** `tab` moves focus and the focused pane draws a magenta border. A focused pane scrolls with its own offset, clamped to its content; the list keeps its cursor in view. The queue is the current person's (`--as`, then `p`), not the selection's: a test moves the selection and the queue stays, then changes the person and the queue changes.

### The seam, and what tablo's call must take and return

`source.go` defines `Source.Load(view, Params) ([]byte, error)`, with `Params{Task, Person, Ref}` and `ErrNoData`. `FSSource` reads the files in `testdata/`, embedded in the binary. Tablo's library call replaces it. The call must take:

- the **view**: `global-tableau`, `task-definition`, `work-blockage-tree`, `history`, `contributor-work-queue`;
- the **focusing parameters** of VIEWS.md: `task` (definition, blockage, history), `person` (queue), and `ref`; the panes ask for detail level and, for the global tableau, the default window;
- the **ref**, empty for HEAD.

It must return the view's JSON as the tablo prototypes emit it (an envelope per tablo prototype 4ed9 later), and a distinguishable "nothing for these parameters" error for a task the ref lacks, which the panes show as `No data.` A view that exists but is empty (a queue with no items, a task nothing blocks) returns data with an empty list and shows its own sentence.

For a task with no data, a pane shows its title, `No data.` and `The source holds no history view for a1c0.`; the other panes of that task keep their content. `a1c0` has no history file for this purpose.

### Test data (`testdata/`)

| File | Produced by |
|------|-------------|
| `global-tableau.json` | copy of tablo `prototype/886d/output/global-tableau.json` |
| `queue-ada.json`, `queue-ben.json`, `queue-dan.json` | copies of `prototype/886d/output/queue-*.json` |
| `task-definition-<id>.json` for a1c0, 4e2b, 9f31, c07d, 7b2e, 3c5d | tablo `go run ./prototype/493e -repo <corpus>/weather-station -ref main -view task -task <id> -level detail` |
| `history-<id>.json` for 4e2b, 9f31, c07d, 7b2e, 3c5d (none for a1c0) | tablo `go run ./prototype/8ed1 -repo <corpus>/weather-station -view history -task <id> -range ..main` |
| `blockage-<id>.json` for the six tasks | filtered copies of `prototype/886d/output/work-blockage-tree.json` (see gaps) |

## Gaps in the tablo data

- 886d's `blockage` ignores `-task`: it emits the whole tree. I filtered it by script to the causes whose task, or one of whose held tasks, lies in the selected task's subtree (everything for the root). The task parameter of VIEWS.md must do that in tablo.
- 8ed1's history for a parent lists the parent's own events, not the replay of its children that VIEWS.md describes (`4e2b` shows one event).
- The history JSON has no status-after-each-event, committer or model (detail level); the pane shows what is there.
- The definition JSON has no state or reason symbols, only gate symbols, so the status line spells the state in words. It carries the old gate symbols (`⚙️`, `🛠️`, `🧩`); the pane renders what the JSON holds.
- The queue and definition JSON carry no ref other than the one the prototype passes.

## Layout choices left for the mockups and the design gate

1. Which panes show at once: the proposal shows all four when there is room, else the stack of four, else the active pane alone; `z` zooms one.
2. Arrangement: list on the left (20 to 34 columns), panes to the right in a 2x2 grid (definition and blockage above, queue and history below) from 70 columns of panes and 12 body rows; a vertical stack below that if 24 body rows exist.
3. History order: newest first here, VIEWS.md lists author-time order.
4. Level: the panes show the glance and detail fields; no key switches the level or the role.
5. Keys as above; focus cycle order, a key to hide a pane, resizing the split and mouse support are open.
6. Whether the list's selection also marks the queue (here it does) or the queue is unmarked.
7. Wrapped lines do not indent; a tree needs indentation preserved.
8. The emoji variation selector: see below.

## Charm findings

- Bubble Tea v1.3.10 and Lip Gloss v1.1.0 suffice. I did not use Bubbles: the list and panes are a few lines each, and the viewport's scroll is in the pane model because the clipping must stay inside the border.
- Lip Gloss `Style.Width`/`Height` is not used for the clipping: it pads, but I clip with `github.com/charmbracelet/x/ansi` (`Truncate`, `Wrap`, `StringWidth`), which Lip Gloss already requires. It never splits a wide symbol: at an odd column a wide symbol that does not fit moves to the next line. The one added direct dependency is that package; the implementation will need it or the v2 equivalent.
- Lip Gloss and `x/ansi` count a base plus U+FE0F (`⚙️`) as two cells. Terminals that count one (see tabloio prototype e0f7) would push the right border out. `--strip-vs16` removes the selector so every terminal agrees; the test passes both ways. The design gate decides the policy, as for e0f7.
- Bubble Tea's `Update` and `View` run without a terminal, so the tests use no `tea.Program`.

## What it leaves out

The real grid (the stand-in is a list). Refresh, any ref other than the data's own, hide or show of columns. Colour theming. Mouse. The provenance level. The real tablo call. Tests do not check how a terminal draws a symbol, only its computed width.

# Prototype 679b: the tableau grid

Task `679b` builds the global tableau as a scrollable grid. This prototype answers three questions, in order of risk:

1. Does a grid of emoji cells stay aligned under Lip Gloss, including symbols made of a narrow base plus a variation selector, when the tree column indents and rows scroll?
2. Do expand and collapse over the task tree, vertical scrolling in a fixed-height window and a moving selection work as a Bubble Tea model over the real global-tableau view data?
3. Does the gate window fold the outside columns to a count, and does a key hide or show a column, with the choice written to and read back from a settings file?

Answers: yes to 2 and 3. Question 1 is yes with one condition: Lip Gloss v1.1.0 counts a narrow base plus U+FE0F as two cells, so the grid is aligned on terminals that widen the cluster and misaligned on older ones that keep one cell. `--strip-vs16` removes the selector, and then every terminal agrees.

## Run

```
cd prototype/679b
go run . [--settings FILE] [--strip-vs16] testdata/global-tableau.json testdata/global-tableau-window0.json
go run . --dump --size 100x10 --keys down,left,4,w testdata/global-tableau.json testdata/global-tableau-window0.json
go test ./...
```

The first file opens; `w` cycles through the others. `--dump` prints the view after a scripted `--keys` sequence (`up down left right enter pgup pgdown home end space` or a single character). Without `--settings` the hidden columns are not kept.

Keys: up/down or j/k move; pgup/pgdown, home/end (g/G) jump; right/l expands a collapsed parent or enters an expanded one; left/h collapses an expanded parent or moves to the parent; enter or space toggles; `e` expands all; `c` collapses to depth one (the glance level); `1`-`9` hide or show the n-th column of the window; `0` shows all; `w` cycles the window variant; `q` quits.

## Test data

Copies of tablo prototype 886d's output for the corpus entry `weather-station` at `main`, from `/home/nbyoung/Projects/Tableaux/tablo/prototype/886d/output/`, made by `go run ./prototype/886d` in tablo:

| File | 886d command |
|------|--------------|
| `testdata/global-tableau.json` | `tableau` (window 1) |
| `testdata/global-tableau-window0.json` | `tableau -window 0` |
| `testdata/global-tableau-state-at-next.json` | `tableau -cell state-at-next` |

## What it shows

The default dump (no chrome) after `--keys down,down,down,down,4,w` with `--settings`, the function column hidden and the window 0 variant open (the folded column before the window shows `‹1`, the hidden column `⊘1`, the fold after `0›`):

```
Id   Task                ‹1 📝  📌  ⚡ ⚓ 📐 🛠️ ⊘1 0›
──── ─────────────────── ── ── ──── ── ── ── ── ── ──
a1c0 ▾ Weather station      🟢  🧑  🧑 🧑 🧑 🧑
4e2b   ▾ Sensor node        🧑  🧑  🧑 🧑 🧑 🧑
9f31       Sensor board     🧑 🧑👀 🧑 🧑 🧑 🧑
c07d       Node firmware    🧑  🧑  🧑 —  🟢 🪆
7b2e     Gateway            🧑  🧑  🧑 🧑 🧑 🧑
3c5d     Dashboard          🧑  🧑  🧑 🧑 🧑 🧑
```

With `--chrome` the dump is exactly `View()`: the same table cut to the window height, the selected row reversed, and below it

```
main window 0, rows 1-5 of 6, selected 7b2e | folded before undefined..undefined 1 | folded after u…
columns 1📝 2📌 (3⚙️) 4⚡ 5⚓ 6📐 7🛠️
↑↓ move  ←→ collapse/expand  enter toggle  e/c all/glance  1-9 column  0 show all  w window  q quit
```

The chrome is the selection cursor (the reversed row), the padding to the window height, and the status, column and help lines. The header, the rule, the rows, the symbols and the fold counts are project data. `--size` bounds the width in both forms.

Options take two hyphens; a single hyphen before a long name (`-dump`) exits with status 2 and names the `--` form. `--help` prints the usage.

The settings file then reads `{"hidden_gates": ["function"]}`. Collapsing `4e2b` in the default window gives `▸ Sensor node (2)`, the count of rows it hides.

What the tests prove (`model_test.go`, no terminal, `Update` and `View` only):

- **Alignment.** Every header, rule and body line has the same display width, at widths 120 to 50, heights 9 to 24, over every window variant, after scrolling, collapsing, hiding, and with ANSI from the selected row. The test measures width with its own model, apart from Lip Gloss. A synthetic tree seven levels deep with long titles also stays aligned and truncates with an ellipsis.
- **The selector.** `lipgloss.Width("⚙️")` is 2, and the grid pads as if the terminal draws it in two cells. Under a narrow terminal model the header measures short, which the test asserts. With `StripVS16` the lines align under both models.
- **Tree, scroll, selection.** Collapse, expand, move to parent, expand all and collapse to glance; a three-row body over six rows scrolls with the selection, keeps its height, and survives a resize.
- **Window and columns.** The default variant folds after to a count of 0; window 0 shows 7 columns and folds `‹1` before. Hiding a column drops its cells, shows `⊘n` with the count of leaf tasks standing at hidden gates, and writes `hidden_gates` to a file under `t.TempDir()`. A new model reads it back, also in another window variant. Malformed and unwritable settings show a message and do not crash.

Marks shown, all taken from the cells of the view data: the state symbol in the current gate cell (🟢, 🔴⛔, ⚪), the contributor mark (🧑, 🤖), the reviewer mark (👀), the subproject mark (🪆), and — for an inapplicable gate. The opening data holds the old symbols ⚙️ 🛠️ 🧩, which the grid renders as they arrive; the corpus has not caught up with the new symbols.

## Left out

- Marks the view data does not carry: the person marks, the historical-junction cells, the provenance level, and a mark for the reason beyond what the cell symbol holds. No colour per state; the selected row reverses only.
- The row date and note; the glance level as a separate view (`c` approximates it).
- The panes, the refresh and `--as`/`--at`; the window is not computed here (the model draws the window the data carries and `w` swaps between files tablo emitted).
- Terminal size detection beyond `tea.WindowSizeMsg`; horizontal scrolling when the gate columns alone exceed the width; wrapping.
- Settings per project; the proposal below keeps one file.

CI does not run this prototype's tests: the root module skips the nested module.

## Layout choices VIEWS.md leaves open

- Folded columns show their count in the header (`‹1`, `0›`), one column per side, with the gate range in the status line; a hidden column folds into one `⊘n` column that counts leaf tasks.
- A collapsed parent shows its hidden row count in parentheses; ▾ and ▸ mark parents.
- Column keys are positional digits, so the digit of a gate shifts between window variants. The design gate may prefer a column cursor.
- Hidden columns are one set of gate names, kept across window variants.

## Proposed settings location

`os.UserConfigDir()/tablotui/settings.json`, so `~/.config/tablotui/settings.json` on Linux, with `{"hidden_gates": [...]}`. The prototype takes the path from `--settings` and never writes to the home directory itself.

## What the design gate must decide

- Whether to accept the wide-terminal assumption, strip the selector (every terminal agrees, the glyph shows in text style), or move to the new symbols that carry no selector (the repository already accepted wrench, brick, ruler and link).
- Charm major versions: this prototype uses Bubble Tea v1.3.10 and Lip Gloss v1.1.0, which pull `muesli/termenv`, `rivo/uniseg` and `charmbracelet/x/ansi`; nothing needed Bubbles v1 for a hand-drawn grid. Lip Gloss v2 and `charm.land` are untried. A width fix for the selector belongs to Lip Gloss, so the owner should check whether v2 measures it differently.
- Positional digits or a column cursor; the settings location and whether it is per project.
- Whether the fold counts leaves only (886d's choice, kept here).

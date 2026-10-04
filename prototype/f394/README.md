# Prototype f394: role and context

Task `f394` Role and context, gate `function`.

This prototype demonstrates the function only. Its tree, marks and keys are a stand-in for the grid of task 679b, and its subject is the derivation of the starting state from the person. The layout belongs to the mockups and the design gate.

## The question

Can the front end derive its starting state from the view data alone, without reading a task file? The starting state is the person's role, the expanded set of the tree and the gate window. The prototype also shows that the flag, the Git default and the role key work, and that a switch of role leaves the hidden columns alone.

## Run

```
cd prototype/f394
go test ./...
go run . --as ben@example.org                       # the full-screen program (needs a terminal)
go run . --as ben@example.org --dump                # print the starting data and exit
go run . --as ada@example.org --dump --keys E,r     # send keys, then print
go run . --dump --chrome                            # include the chrome; as `git config user.email`
```

Options: `--as <email>` (default: `git config user.email`), `--dump`, `--keys <k1,k2,...>` (comma is named `comma`), `--chrome`. The help prints each with two hyphens, and the program refuses `-dump` with status 2. The view does not depend on the window size, so there is no `--size`.

Keys: `j`/`k` row, `h`/`l` column, `enter` expand or collapse a parent, `E` expand every task, `C` collapse every task, `x` hide the column at the cursor, `X` show every column, `r` switch the role, `q` quit.

The role key cycles through the roles the person holds, from the view data: the owner holds `owner`, and `contributor` too with assigned tasks (Ada); a contributor who is not the owner holds `contributor` (Ben, Dan); a person who appears nowhere, or has no task (opus), holds `observer`. With one role the key changes nothing and the status line says "one role only". The observer starts in the owner's layout under the observer label and never reads as `owner`.

The chrome is the status line (window, expanded, hidden, roles held, notices), the cursor `>`, the column underline and the blank line. The project data is the title line that names the person and the role (the subject of this prototype), the column headers and the task rows with their marks. `--dump` prints the data alone, trimmed; `--dump --chrome` prints exactly what `View()` returns.

## What derives from which view

| Fact | View and fields |
|------|-----------------|
| The owner | Authority delegation, glance: the `assignee` of the row at `depth` 0 (the root). Nothing else carries an assignee for every task. |
| The email appears in the project | Task assignment, detail: an entry in `people[].email`. |
| The assigned tasks | Task assignment, detail: `people[].assigned[].id` (parents included: `a1c0` is Ada's, `4e2b` Ben's). |
| The tasks a person contributes at next | Task assignment, detail: `people[].contributes[]` where `next` is true (`task`, `gate`). |
| The parent of a task | Global tableau: none. The prototype derives it from `rows[].depth` and the pre-order of `rows`. |
| A task's next gate | Global tableau: `rows[].next_gate`. A parent's is rolled up, so the window reads leaves only (`rows[].parent` false). |
| The gate order | Gate definition, glance: `gates[].key`. The global tableau's `columns` hold only the columns inside its own window, and `folded[].by_gate` is a JSON map, so it loses the order of the folded gates. |
| The task's status gate (the `*` mark) | Global tableau: `rows[].status.gate`. |

The starting states:

- The owner: `expanded` is empty, so the root shows alone. The window spans one gate either side of the next gates of every leaf, since the owner views the whole project (`undefined..unit`, as the global tableau's own window).
- A contributor: `expanded` holds every proper ancestor of each assigned task and of each task contributed to next (Ben and Ada: `a1c0`, `4e2b`; Dan: `a1c0`). The window spans one gate either side of the next gates of the assigned leaves and of the `next` contributions (Ben `design..unit`, Dan `defined..function`, Ada `undefined..reliability`).
- Anyone else (an observer, or a known email with no task such as `opus@example.org`): the owner's state, labelled observer.

Ada is both the owner and a contributor; she starts as the owner, and `r` shows her contributor state.

Cross-checks in the tests: the contextual tableau for Ben (`contextual-person-ben.json`) has the same spine rows as the expanded set and `c07d` as the one corner row. The work queue's ready and waiting items for Ada and Dan name the same (task, gate) pairs as the `next` contributions.

## What the view data does NOT carry (for tablo to add)

1. **The owner.** No view states it. The prototype reads the root's assignee from the authority delegation view, a view built for authorities. A `viewer` or `roles` field on any view, or the root's assignee in the global tableau header, would serve.
2. **The assignee of each task, and the parent id, in the global tableau rows.** The rows carry neither; the prototype joins three views and rebuilds the parent from depth and order. A `parent` id and an `assignee` on each row remove both joins.
3. **The person's role as a fact.** The role (owner, contributor, observer) is the prototype's derivation, and so is "appears nowhere". A `person` object in the person-parameterised views (`role`, `known`) would settle both for every front end.
4. **The gate order beside the rows.** The global tableau gives only the window's columns, and `folded[].by_gate` is unordered. The prototype takes the order from the gate definition. An ordered list in the folded entries, or the full list in the tableau, avoids the fourth view.
5. **The next gates of a contributor whose junction is recursive.** Ben's `c07d` has a recursive junction at its next gate (`implementation`): the work queue is empty for him and `contributes_next` is zero. Only `rows[].next_gate` shows the gate, and only the assignee link shows the task is his. The data does not say who does the work in the subproject.
6. **The viewer's default.** `person` defaults to "the viewer", so the viewer's email is an input to tablo. The view output holds `person` only when the caller passes it (`contextual-person-ben.json` does; the global tableau does not). The front end must pass the email it resolves.
7. **Reviewer and authority roles.** A contributor start uses only assigned and contributed tasks. A reviewer's tasks sit in `people[].reviews[]`, an authority's in `authority_over`; the prototype ignores both.

## What it shows

Ada (owner and contributor), `--dump --keys E`: the owner's starting state, expanded.

```
tablotui f394  as ada@example.org  role: owner
task                              undef defin mocku funct perfo relia desig imple unit
- a1c0 Weather station            .     *     >     .     .     .     .     .     .
  - 4e2b Sensor node              .     .     .     *     >     .     .     .     .
      9f31 Sensor board           .     .     .     *     >     .     .     .     .
      c07d Node firmware          .     .     .     .     .     .     *     >     .
    7b2e Gateway                  *     >     .     .     .     .     .     .     .
    3c5d Dashboard                .     *     >     .     .     .     .     .     .
```

Ben (contributor), `--dump`: the parents of his tasks expanded, the window around `implementation`.

```
tablotui f394  as ben@example.org  role: contributor
task                              desig imple unit
- a1c0 Weather station            .     .     .
  - 4e2b Sensor node              .     .     .
      9f31 Sensor board           .     .     .
      c07d Node firmware          *     >     .
    7b2e Gateway                  .     .     .
    3c5d Dashboard                .     .     .
```

A stranger, `--dump --chrome --keys r`: the owner's layout under the observer label, and the key changes nothing.

```
tablotui f394  as nobody@example.org  role: observer
window undefined..unit  expanded -  hidden -  roles observer
one role only: observer
  task                             undef defin mocku funct perfo relia desig imple unit
>+ a1c0 Weather station            .     *     >     .     .     .     .     .     .
```

`*` marks a task's status gate and `>` its next gate. `TestSwitchKeepsHiddenColumns` drives `Update` with key messages: Ada hides a column, switches to contributor and back, and the hidden set stays.

## Test data

All in `testdata/`, copied from tablo (`/home/nbyoung/Projects/Tableaux/tablo`, commit `00f8f68`) at the corpus's `weather-station`:

- `global-tableau.json`, `contextual-person-ben.json`, `queue-ada.json`, `queue-ben.json`, `queue-dan.json`: prototype `886d`, files from `prototype/886d/output/`.
- `task-assignment.json`: prototype `493e`, `go run ./prototype/493e -repo <corpus>/weather-station -ref main -view assignment -level detail`.
- `authority-delegation.json`: the same with `-view authority -level detail`.
- `gate-definition.json`: the same with `-view gate -level glance`.

## What it leaves out

The grid, its cells and symbols (the stand-in prints the gate key, so the symbol-width question of `e0f7` does not arise; the data still carries the old symbols), the panes, persistence of hidden columns between sessions, `--at`, refresh, and a call to tablo. The views load from embedded copies. Reviewer and authority starts. Viewing as a role the person does not hold (a preview), such as Ben looking at the owner's layout: a question for the design gate. CI does not run this prototype's tests: it is a nested module that the root `go test ./...` skips.

## What the design gate must decide

- **`--as` against `person`.** README "What it does" names the flag `--as <email>`; VIEWS.md names the parameter `person`, "the viewer by default", and tabloio's queue example uses `--person`. Question for the owner: is `--as` a front-end flag that sets the `person` parameter (the prototype's reading), or should the flag be `--person`?
- The owner's window: the prototype windows the whole project's next gates. VIEWS.md says "the next gates of the tasks in view", which for a collapsed root is the root's rolled-up gate alone.
- The expanded set: ancestors of the assigned tasks only (the prototype), or the assigned parents too (so `4e2b` opens for Ben).
- A person who is both owner and contributor starts as the owner and switches between those two roles.
- Whether the observer shows the owner's layout (the prototype does, labelled observer), and whether a preview of a role not held is wanted.
- Whether tablo adds the facts in the list above, or the front end keeps joining views.
- Charm majors: Bubble Tea v1.3.10 and Lip Gloss v1.1.0 serve; `Update` and `View` run without a terminal, so tests need no program. Nothing here depends on the major.

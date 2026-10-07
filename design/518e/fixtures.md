# Fixtures of design 518e

The tests of `internal/ui/act` build these values in Go. `fakeFacts` implements `act.Facts`, `fakeRepo` implements `act.Repo` and records every call, and a stub home pane supplies the context line `Weather station · working tree on main at 3cdae52` and counts each `ui.ReloadMsg`.

## The project and the tasks

`Project`: the gates `undefined defined mockup function design implementation unit integrate validate release`; the states `undefined nominal at_risk stalled complete`; the reasons `overloaded blocked review`; the trunk `main`.

Every junction is plain with the contributor `ada@example.org` unless the table says otherwise. `Task` on any other id returns the error `task <id>: no such task`.

| Id | Title | Junctions that differ | Status | Children |
|----|-------|----------------------|--------|----------|
| `9f31` | Sensor board | `design`: reviewer `ben@example.org` | `function`, `at_risk`, `blocked`, note `The supplier slips the prototype boards to November` | none |
| `c07d` | Node firmware | `mockup`: not-applicable; `implementation`: recursive, url `firmware`, a submodule | `design` alone | none |
| `b618` | Write commands | none | no file | none |
| `e0aa` | Enclosure | none | `validate`, `nominal` | none |
| `07e0` | Sensor node | none | no file | `9f31`, order 1 |

## The plans

Every plan of `fakeRepo` has the branch `main`, the base `3cdae52f0a1b2c3d4e5f60718293a4b5c6d7e8f9`, and `Ada Lovelace <ada@example.org>` as author and committer. `Apply` returns the commit `0881b37c2f9d4e1a8b7c6d5e4f3a2b1c0d9e8f7a` unless a test sets another answer.

| Call | Subject | Trailers | Diff |
|------|---------|----------|------|
| `Record` | `Record sensor board (9f31) at design` | none | The diff that [`confirm-80x24.txt`](confirm-80x24.txt) shows, from `diff --git` to the last `+` line |
| `Review` | `Accept sensor board (9f31) at <gate>` | `Reviewed: <id> <gate>` | empty |
| `Reaffirm` | `Reaffirm sensor board (9f31)` | `Reaffirmed: <id>` | empty |
| `Authorise` | `Authorise sensor board (9f31)` | `Authorised: <id>` | empty |

## The record form: keys to request (T5)

Each row selects the task with the cursor on the gate named, presses `r`, the keys, then `enter`, and compares the `action.Recording` that `Record` receives. `N` stands for the note of `9f31` above. A field the row leaves out is its zero value.

| # | Task | Cursor gate | Keys after `r` | Gate | State | Reason | Note |
|---|------|-------------|----------------|------|-------|--------|------|
| 1 | `9f31` | none | none | `function` | `at_risk` | `blocked` | `N` |
| 2 | `9f31` | `defined` | none | `function` | `at_risk` | `blocked` | `N` |
| 3 | `9f31` | `design` | none | `design` | `nominal` | `NoReason` | `NoNote` |
| 4 | `9f31` | none | `right` | `design` | `nominal` | `NoReason` | `NoNote` |
| 5 | `9f31` | none | `right left` | `function` | `at_risk` | `blocked` | `N` |
| 6 | `9f31` | none | `down right down left right right up up right` | `design` | `stalled` | `review` | `NoNote` |
| 7 | `9f31` | none | `down down down ctrl+u x up up up right` | `design` | `nominal` | `NoReason` | `x` |
| 8 | `9f31` | none | `down down left left` | `function` | `at_risk` | `NoReason` | `N` |
| 9 | `c07d` | none | none | `design` | | | |
| 10 | `c07d` | none | `down` | `design` | | | |
| 11 | `c07d` | none | `left` | `function` | `nominal` | `NoReason` | `NoNote` |
| 12 | `b618` | none | none | `undefined` | `undefined` | `NoReason` | `NoNote` |
| 13 | `b618` | none | `right down` | `defined` | `nominal` | `NoReason` | `NoNote` |
| 14 | `e0aa` | `release` | none | `release` | `complete` | `NoReason` | `NoNote` |
| 15 | `e0aa` | none | `left` eight times | `undefined` | `undefined` | `NoReason` | `NoNote` |

Row 6 shows a touched state and reason that survive a change of gate while the untouched note takes the new gate's default. Rows 9 and 10 show the recursive next junction: the request holds the gate alone, and `down` finds no other field to stand on.

## The goldens (T1)

Each file is the whole screen with ANSI sequences and trailing spaces removed.

| Golden | Size | Options | Selection | Keys | The fake answers |
|--------|------|---------|-----------|------|------------------|
| [`form-80x16.txt`](form-80x16.txt) | 80×16 | none | `9f31`, `design` | `r` | |
| [`form-refused-80x16.txt`](form-refused-80x16.txt) | 80×16 | none | `9f31`, `design` | `r down down down`, the text `The boards arrive`, `enter` | `Record` refuses: code `unreviewed`, task `9f31`, gate `design`, rule `S11`, text `the status passes design, which ben@example.org reviews and has not accepted; record the reason review to hand the work off` |
| [`form-note-60x12.txt`](form-note-60x12.txt) | 60×12 | none | `9f31`, no gate | `r down down down` | |
| [`form-held-40x8.txt`](form-held-40x8.txt) | 40×8 | none | `c07d`, no gate | `r` | |
| [`confirm-80x24.txt`](confirm-80x24.txt) | 80×24 | none | `9f31`, `design` | `r down down down`, the text `The boards arrive and pass the bench test`, `enter` | |
| [`confirm-warning-60x10.txt`](confirm-warning-60x10.txt) | 60×10 | `Viewer: noreply@anthropic.com`; `Uncommitted` returns `.tableaux/tasks/c07d.yaml` | `9f31`, no gate | `a` | `Reaffirm` adds the warning `not-contributor`, text `ada@example.org is neither the contributor at design nor the assignee` |
| [`refused-80x12.txt`](refused-80x12.txt) | 80×12 | none | `9f31`, `design` | `v` | `Review` refuses: code `wrong-hand`, task `9f31`, gate `design`, rule `H2`, text `ben@example.org reviews this junction, and the commit is ada@example.org's` |
| [`failed-80x16.txt`](failed-80x16.txt) | 80×16 | none | `9f31`, no gate | `u y` | `Apply` returns `*action.ExecError{Tool: "git", Args: ["commit", "--cleanup=verbatim"], Exit: 1}` with the two lines of standard error the golden shows |

The pager lines of the two `confirm` goldens come from `Plan.String()` of tabloio's package. A trial wrote the goldens against a stand-in that follows tabloio's `design/b618/plan.txt`; where the real package prints a plan otherwise, the package's text stands, the test's expectation is `Plan.String()` wrapped as the design states, and the golden changes with it.

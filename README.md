# tablotui

The Tableaux terminal front end: a full-screen tableau with panes for the task, the queue, the blockage tree and the history.

The name plays on *Tableaux*: it joins the pronunciation to *tui*, a terminal user interface.

## Place in the family

| Project                                            | Role                                                     |
|----------------------------------------------------|----------------------------------------------------------|
| [tableaux](https://github.com/nbyoung/tableaux)    | The language: method, syntax, schemas, corpus, mockups   |
| [tablo](https://github.com/nbyoung/tablo)          | The backend: library and plumbing command                |
| [tabloio](https://github.com/nbyoung/tabloio)      | The command line: textual output and Git input           |
| [tablotui](https://github.com/nbyoung/tablotui)    | The terminal user interface                              |
| [tableaud](https://github.com/nbyoung/tableaud)    | The local daemon: HTML views with progressive disclosure |

The split follows Git's own: `tablo` is plumbing that reads a repository and
emits data, and the three front ends are porcelain that presents it. A front end
never reads a task file itself.

## What it does

- **Show** the tableau as a grid with expand and collapse over the task tree and a gate window whose columns the viewer hides or shows, kept between sessions.
- **Follow** the selection into panes for the task definition, the work queue, the blockage tree and the history.
- **Start** where the person at the keyboard works: the owner at the root, a contributor at the parents of their tasks.
- **Refresh** when the repository changes, and view any ref.

```
tablotui                          # the working tree on HEAD, as the Git identity
tablotui --as ada@example.org     # as another participant
tablotui --ref v1.0               # at a tag
tablotui --settings <file>        # keep the column choices in another file
```

## Plan

The project's plan is the Tableaux project in [`.tableaux/`](.tableaux/). The
[Tableaux tooling plan](https://github.com/nbyoung/tableaux/blob/main/PLAN.md)
in the `tableaux` repository pins this project as the submodule
`subprojects/tablotui` and tracks its root task through a recursive junction, states the review policy every task here inherits, and
proposes Go as the implementation language.

## Layout

```
cmd/tablotui/       # the command: main.go and its tests
internal/view/      # the tableau as the grid reads it, and its decoder
internal/view/list/ # the data of the four list views the detail panes draw, and its decoder
internal/source/    # the Tableaux, Lists and Viewers interfaces and the fixture source
internal/identity/  # the person at the keyboard: --as, else the Git identity
internal/settings/  # the per-viewer column choices and their file
internal/ui/        # the frame: the Bubble Tea root model, panes, modes, measure and styles
internal/ui/grid/   # the tableau grid pane
internal/ui/detail/ # the four detail panes, the keys that open them and their layout
internal/ui/role/   # the role pane that never opens: the arrival of each role and the R key
internal/watch/     # the Git-polling watcher: what counts as a change, with its test clock
internal/refresh/   # the Bubble Tea component that reloads once per settled change
.github/workflows/  # ci.yml on push and pull request, release.yml on a tag
.goreleaser.yaml    # the cross-compilation and release matrix
.golangci.yml       # the lint configuration
.tableaux/          # the plan
```

The module is `github.com/nbyoung/tablotui`. The four subprojects share this
shape: one command under `cmd/`, packages under `internal/`, and the same
workflow, release and lint files.

A prototype under `prototype/` is the function gate's demonstration. It goes when its task records `implementation`: the design's account of what it kept from the prototype and `git log -- prototype/<id>` keep what it showed, and the trunk builds what it ships.

## Build and test

Go is the only build dependency, and only a developer needs it; a user runs a
release binary. `go.mod` names the Go release the module requires, and CI reads
it through `go-version-file`, so one edit moves every job.

```
go build ./cmd/tablotui        # the binary, which prints its version
go test ./...                  # the tests
go vet ./... && gofmt -l .     # the checks CI runs
golangci-lint run              # the linters in .golangci.yml
```

CI runs those four checks on every push and pull request. The golangci-lint
job uses the official action at its latest release, and `.golangci.yml` keeps
the standard linter set. A pull request merges when CI passes.

## Release

A tag `v*` pushed to GitHub runs `release.yml`, which hands the repository to
GoReleaser. `.goreleaser.yaml` builds `cmd/tablotui` with `CGO_ENABLED=0` for
Linux, macOS and Windows on amd64 and arm64, packs each as a tar.gz, or a zip
on Windows, and publishes the six archives with `checksums.txt` to a GitHub
release for the tag. The build sets `main.version` from the tag, so the binary
reports it; a build from source reports the module version that `go install`
records, or `dev`.

Tags follow semantic versioning. While the major version is `0`, a minor bump
may change the interface.

## Dependencies

| Module                                  | Path                                  | Major | Role                        |
|-----------------------------------------|---------------------------------------|-------|-----------------------------|
| [tablo](https://github.com/nbyoung/tablo) | `github.com/nbyoung/tablo`          | v0    | The view data this front end renders |
| Bubble Tea                              | `charm.land/bubbletea/v2`             | v2    | The program and its event loop |
| Bubbles                                 | `charm.land/bubbles/v2`               | v2    | The key bindings; later the viewport and the text input |
| Lip Gloss                               | `charm.land/lipgloss/v2`              | v2    | The styles                  |
| x/ansi                                  | `github.com/charmbracelet/x/ansi`     | v0    | The two width methods, cutting and stripping |
| colorprofile                            | `github.com/charmbracelet/colorprofile` | v0    | The colour profiles the tests fix; Bubble Tea requires it already |

The Charm modules are the v2 line under `charm.land/`, which reports the
terminal's grapheme mode to the model, so the grid measures a symbol as the
terminal draws it. `go.mod` carries their `require` lines; the owner accepted
the major at the review of the tableau grid design.

**tablo.** This front end never reads a task file; it takes view data from
tablo. tablo has no release yet, and its bootstrap runs alongside this one, so
`go.mod` carries no `require` for it until it tags. The policy then:

- Go modules select the minimum version, so a declared range is a floor plus a
  ceiling that convention keeps. The `require` line names the floor, the oldest
  tablo release whose view data this front end reads.
- While tablo is at major `0`, the range is one minor: `v0.N.x`. A tablo minor
  bump may change the data, so a tablotui release follows it deliberately by
  raising the floor.
- From tablo `v1`, the range is the major: `v1.x.y`, and Go's import path
  carries any later major.
- No `replace` directive enters `go.mod`. Day-to-day work across the siblings
  uses the personal `go.work` in the directory above the `tableaux` repository
  that its plan describes, which points at the local `tablo` checkout and stays
  uncommitted. A release builds against the tagged tablo alone.

## Licence

[MIT](LICENSE).

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
tablotui                          # the project at HEAD, as the Git identity
tablotui --as ada@example.org     # as another participant
tablotui --at v1.0                # at a tag
```

## Plan

The project's plan is the Tableaux project in [`.tableaux/`](.tableaux/). The
[Tableaux tooling plan](https://github.com/nbyoung/tableaux/blob/main/PLAN.md)
in the `tableaux` repository pins this project as the submodule
`subprojects/tablotui` and tracks its root task through a recursive junction, states the review policy every task here inherits, and
proposes Go as the implementation language.

## Licence

[MIT](LICENSE).

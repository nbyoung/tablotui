# tablo's golden data

The JSON files here are tablo's golden data of the views the detail panes
draw, copied unchanged from `github.com/nbyoung/tablo` at commit
`620473c299bd693af258804146c44d28bd3d737b`, the module version
`v0.0.0-20261017000822-620473c299bd`, from
`internal/views/testdata/weather-station/`. Each holds the `data` of one
envelope: the view of the corpus entry `weather-station` at `main`.

| File | Command | Parameters |
|---|---|---|
| `task-3c5d.detail.json` | `task` | task `3c5d` |
| `task-c07d.provenance.json` | `task` | task `c07d`, person ben@example.org |
| `queue-ada.glance.json` | `queue` | task `a1c0`, person ada@example.org |
| `queue-dan.glance.json` | `queue` | task `a1c0`, person dan@example.org |
| `blockage.detail.json` | `blockage` | task `a1c0` |
| `history.glance.json` | `history` | none: the root |
| `history-4e2b.detail.json` | `history` | task `4e2b` |
| `history-c07d.provenance.json` | `history` | task `c07d` |

The three history files are also the golden files of tablo's design
`8ed1`, under `design/8ed1/golden/`; there the two that hold a line taken
from a subproject still lack its task's title, which 8ed1 decision 8 adds.

`TestTabloGoldens` in `tablo_test.go` wraps each file in the envelope that
tablo's own schema test builds around its golden data (the source `main`
with no commit, no project, an observer), with the parameters above and the
level the name ends in. It decodes the envelope through `list.Decode` and
draws the pane at the level the data holds, as a document at the widths of
the golden documents and as a view at the sizes of the pane's own fit test,
under both measures. These files are the oracle of the view types: change
none of them by hand. When tablo's golden data changes, copy the files again
and name the new commit here.

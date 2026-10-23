# tablo's golden data

The JSON files here are tablo's golden data of the two tableaux, copied
unchanged from `github.com/nbyoung/tablo` at commit
`21d423c4bdc1323125f2fffc41133435ea010764`, the module version
`v0.2.0`, from
`internal/views/testdata/weather-station/`. Each holds the `data` of one
envelope: the view of the corpus entry `weather-station` at `main`.

| File | Command | Parameters |
|---|---|---|
| `tableau.detail.json` | `tableau` | window 1 |
| `context-ben.detail.json` | `context` | person ben@example.org, window 1 |
| `context-4e2b.glance.json` | `context` | task `4e2b`, window 1 |

`TestTabloGoldens` in `tablo_test.go` wraps each file in the envelope that
tablo's own schema test builds around its golden data (the source `main`
with no commit, no project, an observer), with the parameters above and the
level the name ends in. It decodes the envelope through `view.Decode` and
draws the grid at each level the data holds, at the sizes of the golden
screens, under both measures. These files are the oracle of the view types:
change none of them by hand. When tablo's golden data changes, copy the
files again and name the new commit here.

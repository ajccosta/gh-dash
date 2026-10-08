# ajccosta/gh-dash

A fork of [dlvhdr/gh-dash](https://github.com/dlvhdr/gh-dash) (branch `arrow-nav`, based on v4.26.0)
that adds an optional way to drive the preview pane with the arrow keys and to read a PR's commits one at a time.

## The option

```yaml
defaults:
  preview:
    navigation: arrows   # "keys" or unset: upstream behaviour
```

With `navigation: arrows`, the PRs view is three panes and the arrow keys move between them:

```
┌──────────────── sections bar ────────────────┐
├─ PR list ──────────────┬─ preview ───────────┤
│                        │ Overview … Files    │
└────────────────────────┴─────────────────────┘
```

| Focus | Key | Action |
|---|---|---|
| PR list | `↑` / `↓` | move between PRs; `↑` on the first PR focuses the sections bar |
| PR list | `→` | focus the preview |
| sections bar | `←` / `→` | switch section |
| sections bar | `↓` / `Enter` | back to the pane you came from |
| preview | `←` / `→` | previous / next tab; `←` on Overview focuses the PR list |
| preview | `↑` / `↓` | Commits tab: select a commit; other tabs: scroll one line. `↑` at the top focuses the sections bar |
| preview | `Enter` or `d` | on the Commits tab: show the selected commit's diff |
| any | `Tab` | toggle between the PR list and the preview |
| any | `Esc` | back to the PR list |

The focused bar shows its selected tab in reverse video. `h`/`j`/`k`/`l` and every other key keep their usual meaning.

The commit diff comes from `gh api` (no local clone needed) and is shown with `pager.diff`
(`less` by default, coloured here; `delta` and similar get the raw diff). Every other key keeps its usual meaning.

## Building without a global Go

```bash
mkdir -p .toolchain && curl -fsSL https://go.dev/dl/go1.27.1.darwin-arm64.tar.gz | tar -xz -C .toolchain
source ./env.sh.local   # GOROOT, GOPATH, GOCACHE, GOMODCACHE all under .toolchain/
go build -o gh-dash .
gh extension install .  # after `gh extension remove dash`
```

`env.sh.local` sets `GOROOT=.toolchain/go`, `GOPATH`/`GOCACHE`/`GOMODCACHE` under `.toolchain/`, `GOTOOLCHAIN=local`.

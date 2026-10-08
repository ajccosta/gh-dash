# ajccosta/gh-dash

A fork of [dlvhdr/gh-dash](https://github.com/dlvhdr/gh-dash) (branch `arrow-nav`, based on v4.26.0)
that adds an optional way to drive the preview pane with the arrow keys and to read a PR's commits one at a time.

## The option

```yaml
defaults:
  preview:
    navigation: arrows   # "keys" or unset: upstream behaviour
```

With `navigation: arrows`, on the PRs view with the preview open:

| Key | Action |
|---|---|
| `←` / `→` | cycle the preview tabs (Overview … Files Changed), wrapping around; `h` / `l` still switch sections |
| `Tab` | focus the preview; `Tab` or `Esc` gives focus back to the PR list |
| `↑` / `↓` (focused) | Commits tab: select a commit; other tabs: scroll one line |
| `Enter` or `d` (focused, Commits tab) | show the selected commit's diff in the pager |

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

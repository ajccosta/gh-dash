# ajccosta/gh-dash

A fork of [dlvhdr/gh-dash](https://github.com/dlvhdr/gh-dash) (branch `arrow-nav`, based on v4.26.0)
that adds an optional way to drive the preview pane with the arrow keys and to read a PR's commits one at a time.

## Switching views

The top bar starts with a view switcher, `Notifications │ PRs │ Issues  s`, with the current view
highlighted and the key that cycles views (`s` unless rebound) after it. Upstream had it as icons
in the footer; the footer now shows the hint instead (`s next view`).

## The option

```yaml
defaults:
  preview:
    navigation: arrows   # "keys" or unset: upstream behaviour
```

With `navigation: arrows`, the PRs view is four panes and the arrow keys move between them:

```
┌ views │ sections bar ────────────────────────┐
├─ PR list ──────────────┬─ preview ───────────┤
│                        │ Overview … Files    │
└────────────────────────┴─────────────────────┘
```

| Focus | Key | Action |
|---|---|---|
| PR list | `↑` / `↓` | move between PRs; `↑` on the first PR focuses the sections bar |
| PR list | `→` | focus the preview |
| sections bar | `←` / `→` | switch section |
| sections bar | `↑` | focus the view switcher |
| view switcher | `←` / `→` | switch view (Notifications, PRs, Issues) |
| view switcher | `↓` / `Enter` | back to the sections bar |
| sections bar | `↓` / `Enter` | back to the pane you came from |
| preview | `←` / `→` | previous / next tab; `←` on Overview focuses the PR list |
| preview | `↑` / `↓` | Commits tab: select a commit; other tabs: scroll one line. `↑` at the top focuses the first button of the button row (the sections bar if there is none) |
| preview | `Enter` or `d` | on the Commits tab: show the selected commit's diff |
| approve button | `Enter` | approve, as `v` does (prompt for an optional comment, `Ctrl+d` submits) |
| merge button | `Enter` | merge, as `m` does (asks `(y/N)` first, then runs `gh pr merge`) |
| button row | `←` / `→` | between Approve and Merge; `←` on the first button focuses the PR list |
| button row | `↑` / `↓` | sections bar / back to the tabs |
| any | `Tab` | toggle between the PR list and the preview |
| any | `Esc` | back to the PR list |

The focused bar shows its selected tab in reverse video. `h`/`j`/`k`/`l` and every other key keep their usual meaning.
The Issues and Notifications views get the view switcher, the sections bar and the list
(`↑` on the first row focuses the sections bar); `←`/`→` and `Tab` in their list keep their usual meaning.

The commit diff comes from `gh api` (no local clone needed) and is shown with `pager.diff`
(`less` by default, coloured here; `delta` and similar get the raw diff). Every other key keeps its usual meaning.

## Approve and Merge buttons

The PR preview shows `[ ✓ Approve ]` (green) at the right of the status line, above the tabs, on
every tab. Clicking it, or `Enter` on it in arrows navigation, does exactly what `v` does: it opens
the "Approve with comment…" box (`Ctrl+d` submits, `Esc` cancels). It shows `✓ Approved`, dimmed,
when your latest review of the PR approves it, and is hidden on your own PRs (GitHub's
`viewerDidAuthor`, fetched with the PR) and on merged or closed ones. On your own PR `v` shows
`✗ Can't approve your own PR #N` instead of opening the box. The footer hint mentions `v approve`
while a PR you can approve is selected.

The result shows in the footer: `✓ Approved #4`, or `✗ Approve #4 failed: <gh's error message>`.
Errors stay for 12 s, take priority over background fetches, and are cut to fit the footer line.

Next to it, `[ ⇣ Merge ]` (on the merged-PR purple) does exactly what `m` does: the footer asks
`Are you sure you want to merge this PR? (y/N)`, and only `y` runs `gh pr merge` (which asks for
the merge method in the terminal). It shows on open, non-draft PRs, your own included, and is hidden
on draft, merged and closed ones. When the loaded PR data already says the merge is blocked, it is
dimmed with the reason: `Merge (conflicts)`, `Merge (changes requested)`, `Merge (review required)`
(still pressable: admins can bypass, and gh says why otherwise). The result shows as `✓ Merged #N`
or `✗ Merge #N failed: <gh's error>`. The footer hint adds `m merge`. The focused button is in
reverse video.

## Plain glyphs

No Nerd Font needed: every icon is a plain character (`✓ ✗ ● ○ ◆ ± ✎ … ↑ ↓ ← →` and ASCII),
and icon-only column headers are short words (`Repo`, `CI`, `Upd`, `Age`, `Labels`).

## Building without a global Go

```bash
mkdir -p .toolchain && curl -fsSL https://go.dev/dl/go1.27.1.darwin-arm64.tar.gz | tar -xz -C .toolchain
source ./env.sh.local   # GOROOT, GOPATH, GOCACHE, GOMODCACHE all under .toolchain/
go build -o gh-dash .
gh extension install .  # after `gh extension remove dash`
```

`env.sh.local` sets `GOROOT=.toolchain/go`, `GOPATH`/`GOCACHE`/`GOMODCACHE` under `.toolchain/`, `GOTOOLCHAIN=local`.

## Diff viewer

Set `pager.diff` to [ghdiff](../ghdiff) (`~/Work/ghdiff/ghdiff`) for a GitHub-style view of PR and
commit diffs. gh-dash passes the diff's name in `GHDIFF_TITLE`, and the PR in `GHDIFF_REPO`,
`GHDIFF_PR`, `GHDIFF_COMMIT` (commit diffs) and `GHDIFF_REPO_PATH` (from `repoPaths`), so ghdiff's
Ask Claude can read the local clone. Other pagers ignore these.

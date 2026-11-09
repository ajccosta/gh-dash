<br />
<p align="center">
  <a  class="underline: none;" href="https://gh-dash.dev">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="./docs/public/logo.png">
      <img alt="Text changing depending on mode. Light: 'So light!' Dark: 'So dark!'" width="600" src="./docs/public/logo-light.png">
    </picture>
  </a>
</p>

<p align="center">
    <a href="https://gh-dash.dev" target="_blank">→ https://gh-dash.dev ←</a>
</p>
<p align="center">
  A rich terminal UI for GitHub that doesn't break your flow.
  <br />
  <br />
  <a href="https://github.com/dlvhdr/gh-dash/releases"><img src="https://img.shields.io/github/release/dlvhdr/gh-dash.svg" alt="Latest Release"></a>
  <a href="https://discord.gg/SXNXp9NctV"><img src="https://img.shields.io/discord/1413193703476035755?label=discord" alt="Discord"/></a>
  <a href="https://github.com/sponsors/dlvhdr"><img src=https://img.shields.io/github/sponsors/dlvhdr?logo=githubsponsors&color=EA4AAA /></a>
  <a href="https://www.jetify.com/docs/devbox" alt="Built with Devbox"><img src="https://www.jetify.com/img/devbox/shield_moon.svg" /></a>
</p>

<br />

<img src="./docs/src/assets/overview.gif" />

## This fork

This is [ajccosta/gh-dash](https://github.com/ajccosta/gh-dash) (branch
`arrow-nav`), a fork of [dlvhdr/gh-dash](https://github.com/dlvhdr/gh-dash).
Everything below this section is upstream's README. The fork adds the
following.

### Navigation and mouse

- Arrow-key navigation (`defaults.preview.navigation: arrows`). The arrows move
  focus between the view switcher, the sections bar, the PR list, the preview's
  buttons and its tabs. Tab toggles between the list and the preview, and Esc
  returns to the list.
- A view switcher (Notifications │ PRs │ Issues) in the top bar.
- Mouse support, on by default (`defaults.mouse: false` turns it off). You can
  click views, sections, the `+` button, rows, preview tabs, commits, links and
  the search box. The wheel scrolls, and clickable things light up under the
  pointer.
- Drag the line between the list and the preview to resize them. The size is
  kept across restarts.
- Click a column header to sort by it, and again to reverse. Each section's
  sort is kept across restarts.
- Plain characters instead of Nerd Font glyphs, so any monospace font works.

### PR list and preview

- The PR number, in its state's colour, replaces the state icon.
- **PR stacks:** `S`, or a click on Base, shows each PR under the one it is
  based on, as a graph read top-down in merge order. PRs outside the search
  that the list is stacked on are shown greyed out, and conflicts are marked.
- The preview has two tabs. **Overview** shows the summary, commits (each one
  opens its diff), checks, files and activity in one scroll. **Commits** has
  an "All commits" entry.
- A **Conflicts** tab lists the files a conflicting PR conflicts in. Each file
  opens side by side in [ghdiff](https://github.com/ajccosta/ghdiff), with
  every conflict shown whole.
- Markdown renders as on GitHub: tables are kept, bare URLs, `#123` and
  `@user` become links, images become links, and GitHub's HTML is converted.
  Links are clickable.
- Review comments show the code they are on. Clicking `path#L…` opens the
  diff at those lines.
- Diffs are handed to the pager with the PR's repo, number, commit and local
  clone (`GHDIFF_*` variables). The screen stays covered while the pager
  starts and after it quits.
- Outside a repo, sections meant for the current repo are hidden, instead of
  searching all of GitHub.

### Acting on PRs

- **Approve**, **Merge** and **Close** buttons on the preview.
- **Approve your own PRs locally:** GitHub doesn't allow it, so the approval
  is kept on your machine for that head commit. The PR turns green until new
  commits arrive.
- Merging opens a dialog that asks for the method and remembers it. The merge
  runs in the background, and the PR stays in the list, updated in place.
- When GitHub can't merge a PR but git can, the PR is merged with its base
  locally, pushed, then merged. A blocked Merge button is amber with ⚠.
- PRs whose mergeability GitHub is still checking keep their last known state
  until GitHub answers.
- **Batch merge:** select PRs with `*` or Ctrl/Option/Shift-click (`Esc`
  clears), then `m` asks once and merges them one at a time, a PR before
  those stacked on it. It stops at the first that fails.
- **Delete a merged PR's branch** (`B`, or "Delete branch" in the merge
  dialog). PRs based on that branch are moved onto its base first. The default
  branch is never deleted.
- **Remove a commit from a PR** (`U`): revert it, or drop it and force-push
  when that is safe.

### Custom keybindings

- `form:` asks for choices in a dialog first; the command reads them as
  `{{.Form.<name>}}`. The dialog starts from the last choices made.
  `optionsCommand` fills a field's options from a command, and `input: true`
  lets you type a value. In a typed field, ←→ picks a shown option, typing
  filters the options, and tab completes.
- `progress: true` runs the command in the background, and the dialog draws a
  bar from the command's `progress PCT TEXT` lines. An `ask TEXT<tab>OPTIONS`
  line shows a question, and the answer goes to the command's stdin.
- `footer: true` shows the keybinding in the footer hint.

### Fixes

- A section's follow-up fetches are no longer dropped when it finishes
  loading. Before, the stack fetch spun forever and kept the CPU busy.
- The content fills the screen, with no empty row under the footer.
- Upstream's branding is gone: the donate link in the footer, the DASH badge
  in the top bar, its check for new upstream releases, the `sponsors`
  command, and the logo and gh-dash.dev link in `--help`.


## ❤️ Sponsoring

If you enjoy `DASH` and want to help, consider supporting the project with a
donation at the [sponsors page](https://github.com/sponsors/dlvhdr).

Thank you to all past and existing sponsors! 🙏🏽

<!-- sponsors:begin -->
<!-- this list is auto-generated by https://github.com/goreleaser/sponsors -->

## Sponsors

<h3 align="center">TUI Innovator</h3>

<p align="center">
<a href="https://github.com/denopink?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/12867159?s=160&u=61c67cd6aab705e030389253a7c326384a6c6a1b&v=4" alt="Owen Cabalceta" width="160"/></a>&nbsp;&nbsp;
</p>

<h3 align="center">TUI Visionary</h3>

<p align="center">
<a href="https://engineering.tryarchitect.com?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/179040529?s=128&v=4" alt="Architect" width="128"/></a>&nbsp;&nbsp;
<a href="http://steipete.me?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/58493?s=128&u=95a4fccef3d341039661c427695691a664e333ba&v=4" alt="Peter Steinberger" width="128"/></a>&nbsp;&nbsp;
</p>

<h3 align="center">TUI Power User</h3>

<p align="center">
<a href="https://alexandergarber.com?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/9858440?s=50&u=3f0f3072d5c4e7ee4c0cd72e5f8ad2edd01033be&v=4" alt="Alexander Garber" width="50"/></a>&nbsp;&nbsp;
<a href="https://bjg.io?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/1212?s=50&u=9af0530b0e65983da61672445f974432cc1ca8ea&v=4" alt="Brian Gianforcaro" width="50"/></a>&nbsp;&nbsp;
<a href="https://github.com/cdxn?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/2151520?s=50&u=eb74f479693b7f172506e678a169a34ead03796b&v=4" alt="cdxn" width="50"/></a>&nbsp;&nbsp;
<a href="https://github.com/kenosando?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/13559092?s=50&u=efb2ab2cd3c739182f76876e2e88a369ac71d65b&v=4" alt="Ken Sanders" width="50"/></a>&nbsp;&nbsp;
<a href="https://rollans.sh?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/4101096?s=50&u=8330a195570652f1bcd39e89bc5571ee41b15386&v=4" alt="Luke Rollans" width="50"/></a>&nbsp;&nbsp;
<a href="https://github.com/mchisolm0?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/39521893?s=50&u=2ab5b762d1573b0aecc776a30a93cc8ab04fdbad&v=4" alt="Matthew Chisolm" width="50"/></a>&nbsp;&nbsp;
<a href="https://nikolay.com?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/2882?s=50&u=b5278753f2aeee2a074f17c163df9179d3640b03&v=4" alt="Nikolay Kolev" width="50"/></a>&nbsp;&nbsp;
<a href="https://philippeserhal.com/?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/1377702?s=50&u=97edad0b41ef9cb95fd1825a62e244e6648c4681&v=4" alt="Philippe Serhal" width="50"/></a>&nbsp;&nbsp;
<a href="https://ames.sh?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/8071752?s=50&u=d66ecf5d4a8d8cdd3cc6bab4c5524a0bdd0d9b21&v=4" alt="Scott Ames" width="50"/></a>&nbsp;&nbsp;
<a href="https://github.com/sideshowbarker?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/194984?s=50&v=4" alt="sideshowbarker" width="50"/></a>&nbsp;&nbsp;
<a href="https://github.com/scjudd?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/167754?s=50&u=b094905c1930afc3b307f39776d5fc4250d02298&v=4" alt="Spencer Judd" width="50"/></a>&nbsp;&nbsp;
<a href="https://github.com/candril?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/404352?s=50&v=4" alt="Stefan Lüdin" width="50"/></a>&nbsp;&nbsp;
<a href="https://stevie.sh?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/6165538?s=50&u=2d9fc263793ca4050bc0a76c3e6c297fd72ce71c&v=4" alt="Stevie Huh" width="50"/></a>&nbsp;&nbsp;
<a href="http://dangel.im?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/350922?s=50&v=4" alt="Ulrich Dangel" width="50"/></a>&nbsp;&nbsp;
<a href="https://github.com/vosahloj?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/566194?s=50&u=0bca2b2bfa091749aa09df75d2d588de1fdb42b8&v=4" alt="vosahloj" width="50"/></a>&nbsp;&nbsp;
<a href="https://github.com/roninjin10?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/35039927?s=50&u=94370136033f1a4d900b4857c2ae13964bf51f96&v=4" alt="Will Cory" width="50"/></a>&nbsp;&nbsp;
</p>

<h3 align="center">TUI Backer</h3>

<p align="center">
<a href="https://jeffry.in?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/1074042?s=40&u=81cb6e662a6d3254850200c4498ed07e7d59061e&v=4" alt="Jeff Wainwright" width="40"/></a>&nbsp;&nbsp;
<a href="https://purajit.com?utm_source=dlvhdr&utm_medium=sponsor&utm_campaign=homepage&utm_content=github" target="_blank" rel="noopener sponsored" ><img src="https://avatars.githubusercontent.com/u/7026198?s=40&u=5b76b661dba1677a00ae40a83f19c42250b81f5a&v=4" alt="purajit" width="40"/></a>&nbsp;&nbsp;
</p>

<!-- sponsors:end -->

## 🌟 Features

- User-defined, per-repo, PRs & issues sections
- Overridable vim-style keyboard hotkeys
- Custom actions to perform your specific workflow needs
- Everything you can do on GitHub - diff, comment, checkout, push, update etc.
- Control every setting with a YAML config file

If you like quickly navigating with your keyboard, seeing the PRs and issues you need and you <strong>love the terminal</strong> - <code>DASH</code> is for you! 🫵🏽

## 📃 Docs

`DASH` has an extensive docs site at [gh-dash.dev/getting-started](https://gh-dash.dev/getting-started).

## 👥 Discord

Have questions? Join our [Discord community](https://discord.gg/SXNXp9NctV)!

## 🙏 Contributing

See the contribution guide at [https://www.gh-dash.dev/contributing](https://www.gh-dash.dev/contributing/).

## 🛞 Under the hood

`DASH` uses:

- [bubbletea](https://github.com/charmbracelet/bubbletea) for the TUI
- [lipgloss](https://github.com/charmbracelet/lipgloss) for the styling
- [glamour](https://github.com/charmbracelet/glamour) for rendering markdown
- [vhs](https://github.com/charmbracelet/vhs) for generating the GIF
- [cobra](https://github.com/spf13/cobra) for the CLI
- [gh](https://github.com/cli/cli) for the GitHub functionality
- [delta](https://github.com/dandavison/delta) for viewing PR diffs

## Authors

Dolev Hadar ([@dlvhdr](https://github.com/dlvhdr)) and the [community](https://github.com/dlvhdr/gh-dash/graphs/contributors).

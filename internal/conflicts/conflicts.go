// Package conflicts has git work out where PRs conflict, in a repo's local
// checkout (repoPaths), without touching its work tree or asking GitHub: the
// PRs and their base are fetched into refs of their own, merged in memory with
// `git merge-tree`, and the refs removed again. The merges stay in the object
// store, so their files can be diffed afterwards, conflict markers and all.
package conflicts

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Remote is where PRs are fetched from; tests point it elsewhere.
var Remote = func(repo string) string { return "https://github.com/" + repo + ".git" }

// fetchMu keeps two fetches from running at once.
var fetchMu sync.Mutex

// Merge is one ref merged into another by git, in the checkout at Dir.
type Merge struct {
	Dir   string
	Base  string   // the commit merged into
	Head  string   // the one merged
	Tree  string   // the merge, conflict markers in its files
	Files []string // the conflicting ones; none: git merges it cleanly
}

// Git runs git in dir; an error carries what git said. merge-tree finding
// conflicts is an *exec.ExitError, with its output.
func Git(dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Stdout, c.Stderr = &stdout, &stderr
	err := c.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 && args[0] == "merge-tree" {
		return stdout.String(), err // conflicts
	}
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			err = errors.New(msg)
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return stdout.String(), nil
}

// gitCode runs git in dir; code is 1 when merge-tree found conflicts.
func gitCode(dir string, args ...string) (out string, code int, err error) {
	out, err = Git(dir, args...)
	var exit *exec.ExitError
	if args[0] == "merge-tree" && errors.As(err, &exit) {
		return out, 1, nil
	}
	return out, 0, err
}

// Fetch fetches refspecs from repo into the checkout at dir.
func Fetch(dir, repo string, refspecs ...string) error {
	fetchMu.Lock()
	defer fetchMu.Unlock()
	_, _, err := gitCode(dir, append([]string{"fetch", "-q", "--no-tags", "--no-write-fetch-head",
		Remote(repo)}, refspecs...)...)
	return err
}

// MergeRefs merges ref head into ref base in memory. The conflict markers
// name the sides as the refs, without refs/.
func MergeRefs(dir, base, head string) (Merge, error) {
	res := Merge{Dir: dir}
	out, _, err := gitCode(dir, "rev-parse", base, head)
	if err != nil {
		return res, err
	}
	if oids := strings.Fields(out); len(oids) == 2 {
		res.Base, res.Head = oids[0], oids[1]
	}
	base, head = strings.TrimPrefix(base, "refs/"), strings.TrimPrefix(head, "refs/")
	out, code, err := gitCode(dir, "merge-tree", "--write-tree", "--name-only", "--no-messages", base, head)
	if err != nil {
		return res, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	res.Tree = lines[0]
	if code == 1 {
		res.Files = lines[1:]
	}
	return res, nil
}

// ExpandHome expands a leading ~/ of a repoPaths entry.
func ExpandHome(dir string) string {
	if rest, ok := strings.CutPrefix(dir, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return dir
}

// Peer is an open PR another one conflicts with, once Peer is merged.
type Peer struct {
	Number int
	Merge  // the other PR merged into the base with this one merged in
}

// WithPeers finds, for each PR of mine, which PRs of peers into the same base
// it conflicts with once that one is merged: each peer that changes a file it
// changes is merged into base, then the PR into that. One that conflicts with
// base itself is left out. Refs are under refs/gh-dash/<ns>/; the conflict
// markers name the sides gh-dash/<ns>/<base>+pr-<peer> and gh-dash/<ns>/pr-<pr>.
// A pair of PRs both of mine and of peers is merged once, as the one listed
// first merged after the other.
func WithPeers(dir, repo, base, ns string, mine, peers []int) (map[int][]Peer, error) {
	dir = ExpandHome(dir)
	ref := func(name string) string { return "refs/gh-dash/" + ns + "/" + name }
	pr := func(n int) string { return ref(fmt.Sprintf("pr-%d", n)) }
	prs := slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(mine), peers...))))
	refs := []string{ref(base)}
	spec := []string{"+refs/heads/" + base + ":" + ref(base)}
	for _, n := range prs {
		refs = append(refs, pr(n))
		spec = append(spec, fmt.Sprintf("+refs/pull/%d/head:%s", n, pr(n)))
	}
	defer func() {
		for _, r := range refs {
			_, _, _ = gitCode(dir, "update-ref", "-d", r)
		}
	}()
	if err := Fetch(dir, repo, spec...); err != nil {
		return nil, err
	}
	changed := map[int][]string{}
	for _, n := range prs {
		out, _, err := gitCode(dir, "diff", "--name-only", ref(base)+"..."+pr(n))
		if err != nil {
			return nil, err
		}
		changed[n] = strings.Fields(out)
	}
	// landed is peer q merged into the base, once; "" if it conflicts with it.
	landed := map[int]string{}
	land := func(q int) (string, error) {
		if r, ok := landed[q]; ok {
			return r, nil
		}
		landed[q] = ""
		out, code, err := gitCode(dir, "merge-tree", "--write-tree", "--no-messages", ref(base), pr(q))
		if err != nil || code == 1 {
			return "", err
		}
		merged, _, err := gitCode(dir, "commit-tree", strings.TrimSpace(out), "-p", ref(base), "-p", pr(q),
			"-m", fmt.Sprintf("Merge #%d", q))
		if err != nil {
			return "", err
		}
		r := ref(fmt.Sprintf("%s+pr-%d", base, q))
		refs = append(refs, r)
		if _, _, err := gitCode(dir, "update-ref", r, strings.TrimSpace(merged)); err != nil {
			return "", err
		}
		landed[q] = r
		return r, nil
	}
	found := map[int][]Peer{}
	for i, p := range mine {
		for _, q := range peers {
			if q == p || slices.Contains(mine[:i], q) && slices.Contains(peers, p) ||
				!slices.ContainsFunc(changed[q], func(f string) bool { return slices.Contains(changed[p], f) }) {
				continue
			}
			r, err := land(q)
			if err != nil {
				return nil, err
			}
			if r == "" {
				continue
			}
			res, err := MergeRefs(dir, r, pr(p))
			if err != nil {
				return nil, err
			}
			if len(res.Files) > 0 {
				found[p] = append(found[p], Peer{q, res})
				if slices.Contains(mine, q) && slices.Contains(peers, p) {
					found[q] = append(found[q], Peer{p, res})
				}
			}
		}
	}
	return found, nil
}

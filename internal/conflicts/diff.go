package conflicts

import "fmt"

// PRDiff is PR n's diff as GitHub shows it (from where it parts from base),
// made by git in the checkout at dir: for a PR too large for GitHub to send.
// The PR and base are fetched into refs under refs/gh-dash/diff/, removed again.
func PRDiff(dir, repo, base string, n int) ([]byte, error) {
	dir = ExpandHome(dir)
	b, h := "refs/gh-dash/diff/"+base, fmt.Sprintf("refs/gh-dash/diff/pr-%d", n)
	defer func() {
		for _, r := range []string{b, h} {
			_, _, _ = gitCode(dir, "update-ref", "-d", r)
		}
	}()
	if err := Fetch(dir, repo, "+refs/heads/"+base+":"+b, fmt.Sprintf("+refs/pull/%d/head:%s", n, h)); err != nil {
		return nil, err
	}
	out, err := Git(dir, "diff", "--no-color", "--no-ext-diff", "--src-prefix=a/", "--dst-prefix=b/", b+"..."+h)
	return []byte(out), err
}

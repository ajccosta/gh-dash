package markdown

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

// Prepare turns a GitHub PR, issue or comment body into the markdown glamour
// renders well: the HTML GitHub allows becomes markdown, bare URLs, #123 and
// @user become named links, and relative links point into the repo (repo is
// "owner/name"; "" leaves them). Code is left alone.
func Prepare(body, repo string) string {
	body = htmlComment.ReplaceAllString(body, "")
	body = strings.ReplaceAll(body, "\r\n", "\n")
	var out []string
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			out = append(out, line)
			continue
		}
		if inFence {
			out = append(out, line)
			continue
		}
		out = append(out, prepareLine(line, repo))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

var (
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	imgTag      = regexp.MustCompile(`(?i)<img\s[^>]*>`)
	attr        = regexp.MustCompile(`(?i)\b(src|alt|href)\s*=\s*"([^"]*)"`)
	aTag        = regexp.MustCompile(`(?is)<a\s[^>]*>(.*?)</a>`)
	brTag       = regexp.MustCompile(`(?i)<br\s*/?>`)
	summaryTag  = regexp.MustCompile(`(?i)<summary>(.*?)</summary>`)
	kbdTag      = regexp.MustCompile(`(?i)<kbd>(.*?)</kbd>`)
	dropTags    = regexp.MustCompile(`(?i)</?(details|summary|p|div|sub|sup|picture|source|video)(\s[^>]*)?/?>`)
	mdImage     = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)[^)]*\)`)
	mdLinkDest  = regexp.MustCompile(`\]\(([^)\s]+)`)
	quotePrefix = regexp.MustCompile(`^(\s*>)+\s?`)
	alert       = regexp.MustCompile(`^(\s*>\s*)\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*$`)
	bareURL     = regexp.MustCompile(`(^|[\s(])<?(https?://[^\s<>()\[\]]*[^\s<>()\[\].,:;!?'"])>?`)
	issueRef    = regexp.MustCompile(`(^|[\s(\[])#(\d+)\b`)
	mention     = regexp.MustCompile(`(^|[\s(\[])@([A-Za-z0-9](?:[A-Za-z0-9-]{0,38}))\b`)
	ghURL       = regexp.MustCompile(`^https://github\.com/([^/]+/[^/]+)/(?:issues|pull|discussions)/(\d+)(?:[/#?].*)?$`)
	ghCommit    = regexp.MustCompile(`^https://github\.com/([^/]+/[^/]+)/commit/([0-9a-f]{7})[0-9a-f]*`)
)

var alertTitles = map[string]string{
	"NOTE": "Note", "TIP": "Tip", "IMPORTANT": "Important", "WARNING": "Warning", "CAUTION": "Caution",
}

// prepareLine rewrites the parts of a line outside `code spans`.
func prepareLine(line, repo string) string {
	quote := quotePrefix.FindString(line)
	if m := alert.FindStringSubmatch(line); m != nil {
		return quote + "**" + alertTitles[strings.ToUpper(m[2])] + "**\n" + strings.TrimSpace(quote)
	}
	// glamour drops hard line breaks, so <br> starts a paragraph (a space
	// in a table, whose row must stay on one line)
	if strings.HasPrefix(strings.TrimSpace(line), "|") {
		line = brTag.ReplaceAllString(line, " ")
	} else {
		line = brTag.ReplaceAllString(line, "\n"+strings.TrimSpace(quote)+"\n"+quote)
	}
	parts := strings.Split(line, "`")
	for i := 0; i < len(parts); i += 2 { // even parts are outside code spans
		if i == len(parts)-1 && i > 0 && len(parts)%2 == 0 {
			break // an unclosed backtick: leave the rest
		}
		parts[i] = prepareText(parts[i], repo)
	}
	return strings.Join(parts, "`")
}

func prepareText(s, repo string) string {
	s = imgTag.ReplaceAllStringFunc(s, func(tag string) string {
		src, alt := attrs(tag)["src"], attrs(tag)["alt"]
		if src == "" {
			return ""
		}
		return imageLink(alt, src)
	})
	s = aTag.ReplaceAllStringFunc(s, func(tag string) string {
		href := attrs(tag)["href"]
		text := strings.TrimSpace(aTag.FindStringSubmatch(tag)[1])
		if href == "" {
			return text
		}
		if text == "" {
			text = linkLabel(href)
		}
		return "[" + text + "](" + href + ")"
	})
	s = summaryTag.ReplaceAllString(s, "**$1**\n")
	s = kbdTag.ReplaceAllString(s, "`$1`")
	s = dropTags.ReplaceAllString(s, "")
	s = mdImage.ReplaceAllStringFunc(s, func(img string) string {
		m := mdImage.FindStringSubmatch(img)
		return imageLink(m[1], m[2])
	})
	s = nameBareURLs(s)
	if repo != "" {
		s = mdLinkDest.ReplaceAllStringFunc(s, func(m string) string {
			return "](" + resolve(mdLinkDest.FindStringSubmatch(m)[1], repo)
		})
		s = issueRef.ReplaceAllString(s, "$1[#$2](https://github.com/"+repo+"/issues/$2)")
	}
	s = mention.ReplaceAllString(s, "$1[@$2](https://github.com/$2)")
	return s
}

// nameBareURLs links the URLs that aren't already a link's destination.
func nameBareURLs(s string) string {
	var b strings.Builder
	last := 0
	for _, m := range bareURL.FindAllStringSubmatchIndex(s, -1) {
		u := s[m[4]:m[5]]
		if m[2] < m[3] && s[m[2]] == '(' && m[2] > 0 && s[m[2]-1] == ']' {
			continue // [text](url)
		}
		b.WriteString(s[last:m[0]])
		b.WriteString(s[m[2]:m[3]] + "[" + linkLabel(u) + "](" + u + ")")
		last = m[1]
	}
	return b.String() + s[last:]
}

func attrs(tag string) map[string]string {
	out := map[string]string{}
	for _, m := range attr.FindAllStringSubmatch(tag, -1) {
		out[strings.ToLower(m[1])] = m[2]
	}
	return out
}

func imageLink(alt, src string) string {
	alt = strings.TrimSpace(alt)
	if alt == "" || strings.EqualFold(alt, "image") {
		alt = "image"
	}
	return "[▣ " + alt + "](" + src + ")"
}

// linkLabel is a short name for a bare URL: owner/repo#12 for issues and
// PRs, owner/repo@abc1234 for commits, else the host and path.
func linkLabel(u string) string {
	if m := ghURL.FindStringSubmatch(u); m != nil {
		return m[1] + "#" + m[2]
	}
	if m := ghCommit.FindStringSubmatch(u); m != nil {
		return m[1] + "@" + m[2]
	}
	if strings.Contains(u, "github.com/user-attachments/") {
		return "attachment"
	}
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	l := strings.TrimPrefix(p.Host, "www.") + strings.TrimSuffix(p.Path, "/")
	if r := []rune(l); len(r) > 50 {
		l = string(r[:49]) + "…"
	}
	return l
}

// resolve makes a relative link absolute, from the repo's root (PR templates
// live in .github/ and reach the root with ../).
func resolve(dest, repo string) string {
	if dest == "" || strings.HasPrefix(dest, "#") || strings.Contains(dest, "://") || strings.HasPrefix(dest, "mailto:") {
		return dest
	}
	clean := strings.TrimLeft(path.Clean("/"+dest), "/")
	for strings.HasPrefix(clean, "../") {
		clean = strings.TrimPrefix(clean, "../")
	}
	return "https://github.com/" + repo + "/blob/HEAD/" + clean
}

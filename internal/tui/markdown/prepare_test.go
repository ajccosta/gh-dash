package markdown

import "testing"

func TestPrepare(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"issue ref", "Closes #956.", "Closes [#956](https://github.com/o/r/issues/956)."},
		{"mention", "by @someone", "by [@someone](https://github.com/someone)"},
		{"bare issue url", "see https://github.com/a/b/pull/12", "see [a/b#12](https://github.com/a/b/pull/12)"},
		{"bare url", "<https://example.com/x/>", "[example.com/x](https://example.com/x/)"},
		{"link kept", "[two](https://example.com)", "[two](https://example.com)"},
		{"relative link", "[C](../CONTRIBUTING.md)", "[C](https://github.com/o/r/blob/HEAD/CONTRIBUTING.md)"},
		{"img tag", `<img width="9" alt="shot" src="https://x.io/a.png" />`, "[▣ shot](https://x.io/a.png)"},
		{"md image", "![](https://x.io/a.png)", "[▣ image](https://x.io/a.png)"},
		{"a tag", `<a href="https://x.io">X</a>`, "[X](https://x.io)"},
		{"alert", "> [!WARNING]\n> careful", "> **Warning**\n>\n> careful"},
		{"br", "a<br>b", "a\n\nb"},
		{"br in table", "| a<br>b |", "| a b |"},
		{"details", "<details><summary>More</summary>\nx\n</details>", "**More**\n\nx"},
		{"comment", "a<!-- hidden\n-->b", "ab"},
		{"pipes kept", "`.bare`|`.git`", "`.bare`|`.git`"},
		{"code span", "`#1 @a https://x.io`", "`#1 @a https://x.io`"},
		{"fence", "```\n#1 @a\n```", "```\n#1 @a\n```"},
	}
	for _, tt := range tests {
		if got := Prepare(tt.in, "o/r"); got != tt.want {
			t.Errorf("%s: Prepare(%q) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
	}
}

package telegram

import (
	"strings"
	"testing"
)

func TestMarkdownToTelegramHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Headers",
			input:    "# Heading 1\n## Heading 2",
			expected: "<b>Heading 1</b>\n\n<b>Heading 2</b>",
		},
		{
			name:     "Bold and Italic",
			input:    "This is **bold** and *italic* text.",
			expected: "This is <b>bold</b> and <i>italic</i> text.",
		},
		{
			name:     "Inline code with special characters",
			input:    "Use `fmt.Println(\"<a> & <b>\")` here.",
			expected: "Use <code>fmt.Println(&#34;&lt;a&gt; &amp; &lt;b&gt;&#34;)</code> here.",
		},
		{
			name:     "Fenced code block with language",
			input:    "```go\nfunc main() {\n\tprintln(\"<hello> & world\")\n}\n```",
			expected: "<pre><code class=\"language-go\">func main() {\n\tprintln(&#34;&lt;hello&gt; &amp; world&#34;)\n}</code></pre>",
		},
		{
			name:     "Blockquotes",
			input:    "> This is a quote\n> Second line",
			expected: "<blockquote>This is a quote\nSecond line</blockquote>",
		},
		{
			name:     "Links and lists",
			input:    "* Item 1\n* Check [Google](https://google.com)",
			expected: "• Item 1\n• Check <a href=\"https://google.com\">Google</a>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MarkdownToTelegramHTML(tt.input)
			if strings.TrimSpace(result) != strings.TrimSpace(tt.expected) {
				t.Errorf("\nExpected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestSplitTelegramHTML(t *testing.T) {
	// Test splitting with open tags
	longCode := "```go\n" + strings.Repeat("x := 10\n", 500) + "```"
	htmlStr := MarkdownToTelegramHTML(longCode)

	chunks := SplitTelegramHTML(htmlStr, 500)
	if len(chunks) < 2 {
		t.Fatalf("Expected multiple chunks, got %d", len(chunks))
	}

	// Verify first chunk ends with closing tags and second chunk begins with opening tag
	if !strings.HasSuffix(chunks[0], "</code></pre>") {
		t.Errorf("First chunk does not cleanly close <pre><code>: %s", chunks[0][len(chunks[0])-30:])
	}
	if !strings.HasPrefix(chunks[1], "<pre") {
		t.Errorf("Second chunk does not reopen <pre>: %s", chunks[1][:30])
	}
}

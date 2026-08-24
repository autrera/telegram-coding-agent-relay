package telegram

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	// Regex patterns for Markdown parsing
	fencedCodeRegex  = regexp.MustCompile("(?s)```([a-zA-Z0-9_-]*)\r?\n?(.*?)\r?\n?```")
	inlineCodeRegex  = regexp.MustCompile("`([^`\n]+)`")
	headerRegex      = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	boldStarRegex    = regexp.MustCompile(`\*\*([^*\n]+?)\*\*`)
	boldUnderRegex   = regexp.MustCompile(`__([^_\n]+?)__`)
	italicStarRegex  = regexp.MustCompile(`(?:^|[^\w*])\*([^*\n\s](?:[^*\n]*?[^*\n\s])?)\*(?:[^\w*]|$)`)
	italicUnderRegex = regexp.MustCompile(`(?:^|[^\w_])_([^_\n\s](?:[^_\n]*?[^_\n\s])?)_(?:[^\w_]|$)`)
	strikeRegex      = regexp.MustCompile(`~~([^~\n]+?)~~`)
	linkRegex        = regexp.MustCompile(`\[([^\]\n]+)\]\(((?:https?|tg|ftp)://[^\)\s]+)\)`)
	bulletRegex      = regexp.MustCompile(`(?m)^[\*\-]\s+(.+)$`)
	hrRegex          = regexp.MustCompile(`(?m)^(?:---|\*\*\*|___)\s*$`)
	blockquoteRegex  = regexp.MustCompile(`(?m)(?:^>\s*.*(?:\n|$))+`)
)

// MarkdownToTelegramHTML converts standard Markdown into Telegram-compatible HTML.
func MarkdownToTelegramHTML(md string) string {
	if strings.TrimSpace(md) == "" {
		return ""
	}

	// 1. Extract and store fenced code blocks
	var codeBlocks []string
	md = fencedCodeRegex.ReplaceAllStringFunc(md, func(match string) string {
		submatches := fencedCodeRegex.FindStringSubmatch(match)
		lang := ""
		content := ""
		if len(submatches) >= 3 {
			lang = strings.TrimSpace(submatches[1])
			content = submatches[2]
		} else if len(submatches) >= 2 {
			content = submatches[1]
		}
		escapedContent := html.EscapeString(content)
		var blockHTML string
		if lang != "" {
			blockHTML = fmt.Sprintf("<pre><code class=\"language-%s\">%s</code></pre>", html.EscapeString(lang), escapedContent)
		} else {
			blockHTML = fmt.Sprintf("<pre><code>%s</code></pre>", escapedContent)
		}
		idx := len(codeBlocks)
		codeBlocks = append(codeBlocks, blockHTML)
		return fmt.Sprintf("@@FENCED_CODE_%d@@", idx)
	})

	// 2. Extract and store inline code
	var inlineCodes []string
	md = inlineCodeRegex.ReplaceAllStringFunc(md, func(match string) string {
		submatches := inlineCodeRegex.FindStringSubmatch(match)
		content := ""
		if len(submatches) >= 2 {
			content = submatches[1]
		}
		escapedContent := html.EscapeString(content)
		codeHTML := fmt.Sprintf("<code>%s</code>", escapedContent)
		idx := len(inlineCodes)
		inlineCodes = append(inlineCodes, codeHTML)
		return fmt.Sprintf("@@INLINE_CODE_%d@@", idx)
	})

	// 3. Process blockquotes before general HTML escaping
	md = blockquoteRegex.ReplaceAllStringFunc(md, func(match string) string {
		lines := strings.Split(strings.TrimSpace(match), "\n")
		var cleanLines []string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			l = strings.TrimPrefix(l, ">")
			cleanLines = append(cleanLines, strings.TrimSpace(l))
		}
		inner := strings.Join(cleanLines, "\n")
		return fmt.Sprintf("<blockquote>%s</blockquote>\n", inner)
	})

	// 4. Escape general HTML characters (&, <, >) in the remaining text, preserving blockquote tags
	md = html.EscapeString(md)
	md = strings.ReplaceAll(md, "&lt;blockquote&gt;", "<blockquote>")
	md = strings.ReplaceAll(md, "&lt;/blockquote&gt;", "</blockquote>")

	// 5. Transform Horizontal Rules: --- -> ——————————
	md = hrRegex.ReplaceAllString(md, "——————————")

	// 6. Transform Bullet lists: * item or - item -> • item (done BEFORE italics to avoid * collision)
	md = bulletRegex.ReplaceAllString(md, "• $1")

	// 7. Transform Headers: # Title -> <b>Title</b>
	md = headerRegex.ReplaceAllString(md, "<b>$2</b>\n")

	// 8. Transform Bold: **text** or __text__ -> <b>text</b>
	md = boldStarRegex.ReplaceAllString(md, "<b>$1</b>")
	md = boldUnderRegex.ReplaceAllString(md, "<b>$1</b>")

	// 9. Transform Italic: *text* or _text_ -> <i>text</i>
	md = italicStarRegex.ReplaceAllStringFunc(md, func(match string) string {
		sub := italicStarRegex.FindStringSubmatch(match)
		if len(sub) >= 2 {
			return strings.Replace(match, "*"+sub[1]+"*", "<i>"+sub[1]+"</i>", 1)
		}
		return match
	})
	md = italicUnderRegex.ReplaceAllStringFunc(md, func(match string) string {
		sub := italicUnderRegex.FindStringSubmatch(match)
		if len(sub) >= 2 {
			return strings.Replace(match, "_"+sub[1]+"_", "<i>"+sub[1]+"</i>", 1)
		}
		return match
	})

	// 10. Transform Strikethrough: ~~text~~ -> <s>text</s>
	md = strikeRegex.ReplaceAllString(md, "<s>$1</s>")

	// 11. Transform Links: [label](url) -> <a href="url">label</a>
	md = linkRegex.ReplaceAllString(md, `<a href="$2">$1</a>`)

	// 12. Restore inline code
	for i, codeHTML := range inlineCodes {
		placeholder := fmt.Sprintf("@@INLINE_CODE_%d@@", i)
		md = strings.ReplaceAll(md, placeholder, codeHTML)
	}

	// 13. Restore fenced code blocks
	for i, blockHTML := range codeBlocks {
		placeholder := fmt.Sprintf("@@FENCED_CODE_%d@@", i)
		md = strings.ReplaceAll(md, placeholder, blockHTML)
	}

	return strings.TrimSpace(md)
}

// StripHTML removes all HTML tags and converts entities back to plain text.
func StripHTML(s string) string {
	tagRegex := regexp.MustCompile(`<[^>]*>`)
	stripped := tagRegex.ReplaceAllString(s, "")
	return html.UnescapeString(stripped)
}

// SplitTelegramHTML splits a long HTML message into chunks of <= maxLen characters,
// safely closing and reopening tags (like <pre><code>, <blockquote>, <b>, <i>) across chunk boundaries.
func SplitTelegramHTML(text string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = 3900
	}
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= maxLen {
			chunks = append(chunks, remaining)
			break
		}

		// Find the best split position within maxLen
		splitIdx := findSplitIndex(remaining, maxLen)
		chunk := remaining[:splitIdx]
		remaining = strings.TrimLeft(remaining[splitIdx:], "\r\n")

		// Analyze open tags in chunk
		openTags := findUnclosedTags(chunk)
		if len(openTags) > 0 {
			// Close them at the end of this chunk in reverse order
			var closing strings.Builder
			for i := len(openTags) - 1; i >= 0; i-- {
				tag := openTags[i]
				closing.WriteString(fmt.Sprintf("</%s>", tag.Name))
			}
			chunk = chunk + closing.String()

			// Reopen them at the start of remaining
			var opening strings.Builder
			for _, tag := range openTags {
				if tag.Attributes != "" {
					opening.WriteString(fmt.Sprintf("<%s %s>", tag.Name, tag.Attributes))
				} else {
					opening.WriteString(fmt.Sprintf("<%s>", tag.Name))
				}
			}
			remaining = opening.String() + remaining
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}

type tagInfo struct {
	Name       string
	Attributes string
}

func findUnclosedTags(htmlStr string) []tagInfo {
	tagRegex := regexp.MustCompile(`</?([a-zA-Z0-9_-]+)(?:\s+([^>]*))?>`)
	matches := tagRegex.FindAllStringSubmatchIndex(htmlStr, -1)

	var stack []tagInfo
	for _, m := range matches {
		fullMatch := htmlStr[m[0]:m[1]]
		tagName := strings.ToLower(htmlStr[m[2]:m[3]])
		var attrs string
		if m[4] != -1 && m[5] != -1 {
			attrs = htmlStr[m[4]:m[5]]
		}

		if strings.HasPrefix(fullMatch, "</") {
			// Closing tag: pop matching from stack
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].Name == tagName {
					stack = append(stack[:i], stack[i+1:]...)
					break
				}
			}
		} else {
			// Opening tag (ignore self-closing)
			if !strings.HasSuffix(fullMatch, "/>") {
				stack = append(stack, tagInfo{Name: tagName, Attributes: attrs})
			}
		}
	}
	return stack
}

func findSplitIndex(s string, limit int) int {
	if limit >= len(s) {
		return len(s)
	}

	// Try splitting at double newline
	sub := s[:limit]
	if lastDblNL := strings.LastIndex(sub, "\n\n"); lastDblNL > limit/2 {
		return lastDblNL + 2
	}

	// Try splitting at single newline
	if lastNL := strings.LastIndex(sub, "\n"); lastNL > limit/2 {
		return lastNL + 1
	}

	// Try splitting at space
	if lastSpace := strings.LastIndex(sub, " "); lastSpace > limit/2 {
		return lastSpace + 1
	}

	// Fallback to strict limit
	return limit
}

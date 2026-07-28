package site

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// Page is one daily write-up ready for the static site.
type Page struct {
	Title    string
	Category string
	Date     time.Time
	// Body is LLM output: HTML fragment (preferred) or markdown.
	BodyMarkdown string
	Subject      string
}

var (
	fenceRE   = regexp.MustCompile("(?is)```([a-z0-9_-]*)\\s*\\r?\\n(.*?)\\r?\\n```")
	h1RE      = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	htmlH1RE  = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	scriptRE  = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	styleRE   = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)
	iframeRE  = regexp.MustCompile(`(?is)<iframe\b[^>]*>.*?</iframe>`)
	objectRE  = regexp.MustCompile(`(?is)<object\b[^>]*>.*?</object>`)
	embedRE   = regexp.MustCompile(`(?is)<embed\b[^>]*/?>`)
	onAttrRE  = regexp.MustCompile(`(?i)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	jsHrefRE  = regexp.MustCompile(`(?i)\s+(href|src|xlink:href)\s*=\s*("|')\s*javascript:[^"']*("|')`)
	dataHrefRE = regexp.MustCompile(`(?i)\s+(href|src)\s*=\s*("|')\s*data:text/html[^"']*("|')`)
)

func RenderHTML(p Page) (string, error) {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = TitleFromBody(p.BodyMarkdown, "systems-daily")
	}

	bodyHTML, err := BodyToHTML(p.BodyMarkdown)
	if err != nil {
		return "", err
	}

	dateStr := p.Date.Format("2006-01-02")
	cat := html.EscapeString(p.Category)
	escTitle := html.EscapeString(title)

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", escTitle)
	// Apply saved/system theme before paint to avoid a flash.
	b.WriteString("<script>(function(){try{var k='systems-daily-theme';var t=localStorage.getItem(k);if(t!=='light'&&t!=='dark'){t=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}document.documentElement.setAttribute('data-theme',t);}catch(e){}})();</script>\n")
	b.WriteString("<style>\n")
	b.WriteString(minimalCSS)
	b.WriteString("\n</style>\n</head>\n<body>\n")
	b.WriteString("<main>\n")
	b.WriteString("<header class=\"meta\">\n")
	b.WriteString("<div class=\"meta-row\">\n")
	b.WriteString("<p class=\"brand\">systems-daily</p>\n")
	b.WriteString("<button type=\"button\" id=\"theme-toggle\" class=\"theme-toggle\" aria-label=\"Toggle light and dark mode\">theme</button>\n")
	b.WriteString("</div>\n")
	fmt.Fprintf(&b, "<p class=\"line\"><span class=\"cat\">%s</span> · <time datetime=%q>%s</time></p>\n",
		cat, dateStr, dateStr)
	b.WriteString("</header>\n")
	b.WriteString("<article>\n")
	b.WriteString(bodyHTML)
	b.WriteString("\n</article>\n")
	b.WriteString("<footer>\n")
	b.WriteString("<p><a href=\"/today/\">today</a></p>\n")
	b.WriteString("</footer>\n")
	b.WriteString("</main>\n")
	b.WriteString(themeToggleJS)
	b.WriteString("\n</body>\n</html>\n")
	return b.String(), nil
}

// BodyToHTML converts an HTML fragment or markdown into article inner HTML.
func BodyToHTML(src string) (string, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", fmt.Errorf("empty body")
	}
	// Strip accidental full-document wrappers from the model.
	src = stripOuterDocument(src)

	if looksLikeHTML(src) {
		return sanitizeFragment(src), nil
	}
	md := prepareMarkdown(src)
	return mdToHTML(md)
}

func stripOuterDocument(src string) string {
	low := strings.ToLower(src)
	// If model wrapped in <article>, unwrap once.
	if strings.HasPrefix(strings.TrimSpace(low), "<article") {
		if m := regexp.MustCompile(`(?is)^\s*<article[^>]*>(.*)</article>\s*$`).FindStringSubmatch(src); len(m) == 2 {
			return strings.TrimSpace(m[1])
		}
	}
	// Drop doctype/html/head/body if present (keep inner).
	if strings.Contains(low, "<html") {
		if m := regexp.MustCompile(`(?is)<body[^>]*>(.*)</body>`).FindStringSubmatch(src); len(m) == 2 {
			return strings.TrimSpace(m[1])
		}
	}
	return src
}

// looksLikeHTML: prefer HTML path when the body is tag-led content.
func looksLikeHTML(src string) bool {
	s := strings.TrimSpace(src)
	if s == "" {
		return false
	}
	// Markdown headings / fences first line → markdown
	if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "```") {
		return false
	}
	if s[0] == '<' {
		return true
	}
	// Common: leading text then tags - treat as HTML if multiple block tags.
	low := strings.ToLower(s)
	n := 0
	for _, tag := range []string{"<h1", "<h2", "<p>", "<p ", "<pre", "<ul", "<ol", "<svg"} {
		if strings.Contains(low, tag) {
			n++
		}
	}
	return n >= 2
}

// sanitizeFragment removes dangerous constructs; keeps structure/SVG.
func sanitizeFragment(s string) string {
	s = scriptRE.ReplaceAllString(s, "")
	s = styleRE.ReplaceAllString(s, "") // shell owns CSS
	s = iframeRE.ReplaceAllString(s, "")
	s = objectRE.ReplaceAllString(s, "")
	s = embedRE.ReplaceAllString(s, "")
	s = onAttrRE.ReplaceAllString(s, "")
	s = jsHrefRE.ReplaceAllString(s, "")
	s = dataHrefRE.ReplaceAllString(s, "")
	s = filterUselessSVGs(s)
	return strings.TrimSpace(s)
}

// prepareMarkdown: drop mermaid; lift svg fences to raw HTML; leave text/code fences.
func prepareMarkdown(src string) string {
	return fenceRE.ReplaceAllStringFunc(src, func(match string) string {
		sub := fenceRE.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		lang := strings.ToLower(strings.TrimSpace(sub[1]))
		body := strings.TrimSpace(sub[2])
		switch lang {
		case "mermaid":
			return "\n\n"
		case "svg":
			if !strings.Contains(strings.ToLower(body), "<svg") {
				return match
			}
			return "\n\n" + body + "\n\n"
		default:
			return match
		}
	})
}

func mdToHTML(md string) (string, error) {
	gm := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(),
			gmhtml.WithHardWraps(),
		),
	)
	var buf bytes.Buffer
	if err := gm.Convert([]byte(md), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// TitleFromBody returns HTML <h1>, markdown #, or fallback.
func TitleFromBody(body, fallback string) string {
	if m := htmlH1RE.FindStringSubmatch(body); len(m) == 2 {
		t := strings.TrimSpace(stripTags(m[1]))
		if t != "" {
			return t
		}
	}
	if m := h1RE.FindStringSubmatch(body); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	if fallback != "" {
		return fallback
	}
	return "systems-daily"
}

// TitleFromMarkdown kept for callers; same as TitleFromBody.
func TitleFromMarkdown(md, fallback string) string {
	return TitleFromBody(md, fallback)
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

const minimalCSS = `
:root {
  color-scheme: light dark;
  --fg: #1a1a1a;
  --muted: #555;
  --bg: #fafafa;
  --border: #ddd;
  --code-bg: #f0f0f0;
  --pre-bg: #f4f4f4;
  --link: #1a1a1a;
  --btn-bg: #eee;
  --btn-fg: #333;
}
html[data-theme="light"] {
  color-scheme: light;
  --fg: #1a1a1a;
  --muted: #555;
  --bg: #fafafa;
  --border: #ddd;
  --code-bg: #f0f0f0;
  --pre-bg: #f4f4f4;
  --link: #1a1a1a;
  --btn-bg: #eee;
  --btn-fg: #333;
}
html[data-theme="dark"] {
  color-scheme: dark;
  --fg: #e8e8e8;
  --muted: #aaa;
  --bg: #121212;
  --border: #333;
  --code-bg: #1e1e1e;
  --pre-bg: #1a1a1a;
  --link: #e8e8e8;
  --btn-bg: #222;
  --btn-fg: #ccc;
}
@media (prefers-color-scheme: dark) {
  html:not([data-theme]) {
    --fg: #e8e8e8;
    --muted: #aaa;
    --bg: #121212;
    --border: #333;
    --code-bg: #1e1e1e;
    --pre-bg: #1a1a1a;
    --link: #e8e8e8;
    --btn-bg: #222;
    --btn-fg: #ccc;
  }
}
* { box-sizing: border-box; }
html { font-size: 17px; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--fg);
  font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  line-height: 1.55;
}
main {
  max-width: 40rem;
  margin: 0 auto;
  padding: 1.5rem 1.1rem 3rem;
}
header.meta {
  border-bottom: 1px solid var(--border);
  padding-bottom: 0.75rem;
  margin-bottom: 1.5rem;
}
.meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.25rem;
}
.brand {
  margin: 0;
  font-size: 0.8rem;
  letter-spacing: 0.04em;
  text-transform: lowercase;
  color: var(--muted);
}
.theme-toggle {
  margin: 0;
  padding: 0.15rem 0.45rem;
  font: inherit;
  font-size: 0.75rem;
  letter-spacing: 0.02em;
  text-transform: lowercase;
  color: var(--btn-fg);
  background: var(--btn-bg);
  border: 1px solid var(--border);
  border-radius: 3px;
  cursor: pointer;
}
.theme-toggle:hover {
  border-color: var(--muted);
}
.theme-toggle:focus-visible {
  outline: 1px solid var(--muted);
  outline-offset: 2px;
}
.line {
  margin: 0;
  font-size: 0.9rem;
  color: var(--muted);
}
.cat { text-transform: lowercase; }
article h1 {
  font-size: 1.45rem;
  font-weight: 650;
  line-height: 1.25;
  margin: 0 0 1rem;
}
article h2 {
  font-size: 1.1rem;
  font-weight: 650;
  margin: 1.6rem 0 0.6rem;
}
article h3 {
  font-size: 1rem;
  font-weight: 650;
  margin: 1.25rem 0 0.5rem;
}
article p { margin: 0.7rem 0; }
article ul, article ol { padding-left: 1.25rem; }
article li { margin: 0.25rem 0; }
article a { color: var(--link); text-decoration: underline; text-underline-offset: 2px; }
article code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.88em;
  background: var(--code-bg);
  padding: 0.1em 0.3em;
  border-radius: 3px;
}
article pre {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.82rem;
  line-height: 1.4;
  background: var(--pre-bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 0.75rem 0.85rem;
  overflow-x: auto;
  margin: 1rem 0;
}
article pre code {
  background: none;
  padding: 0;
  font-size: inherit;
}
article svg {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 1rem 0;
}
article blockquote {
  margin: 0.8rem 0;
  padding-left: 0.9rem;
  border-left: 3px solid var(--border);
  color: var(--muted);
}
footer {
  margin-top: 2.5rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--border);
  font-size: 0.85rem;
  color: var(--muted);
}
footer a { color: var(--muted); }
`

// themeToggleJS is a tiny shell script (not from the model).
const themeToggleJS = `<script>
(function () {
  var key = 'systems-daily-theme';
  var root = document.documentElement;
  var btn = document.getElementById('theme-toggle');
  function current() {
    var t = root.getAttribute('data-theme');
    if (t === 'light' || t === 'dark') return t;
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  function apply(t) {
    root.setAttribute('data-theme', t);
    try { localStorage.setItem(key, t); } catch (e) {}
    if (btn) btn.textContent = t === 'dark' ? 'light' : 'dark';
  }
  apply(current());
  if (btn) {
    btn.addEventListener('click', function () {
      apply(current() === 'dark' ? 'light' : 'dark');
    });
  }
})();
</script>
`

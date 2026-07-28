package content

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Saumya40-codes/systems-daily/internal/llm"
	"github.com/Saumya40-codes/systems-daily/internal/topics"
)

// Article is a generated daily write-up.
type Article struct {
	Topic     topics.Topic
	Subject   string
	Body      string // HTML fragment (preferred) or markdown
	WordCount int
	Model     string
	Generated time.Time
}

// Generator builds medium-depth systems write-ups via an LLM.
type Generator struct {
	LLM            llm.Completer
	TargetWordsMin int
	TargetWordsMax int
}

// Generate produces one article for the given topic.
func (g *Generator) Generate(ctx context.Context, topic topics.Topic) (*Article, error) {
	minW := g.TargetWordsMin
	maxW := g.TargetWordsMax
	if minW <= 0 {
		minW = 700
	}
	if maxW <= minW {
		maxW = minW + 400
	}

	body, err := g.LLM.Chat(ctx, systemPrompt(minW, maxW), userPrompt(topic))
	if err != nil {
		return nil, err
	}

	body = cleanBody(body)
	wc := wordCount(body)
	subject := buildSubject(topic, body)

	label := ""
	if g.LLM != nil {
		label = g.LLM.Label()
	}
	return &Article{
		Topic:     topic,
		Subject:   subject,
		Body:      body,
		WordCount: wc,
		Model:     label,
		Generated: time.Now(),
	}, nil
}

func cleanBody(s string) string {
	s = strings.TrimSpace(s)
	// Strip accidental markdown fences wrapping the whole reply
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 {
			// Only strip if it looks like a full-document fence (``` or ```markdown)
			first := strings.TrimSpace(lines[0])
			if first == "```" || first == "```markdown" || first == "```md" {
				lines = lines[1:]
				if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == "```" {
					lines = lines[:n-1]
				}
				s = strings.TrimSpace(strings.Join(lines, "\n"))
			}
		}
	}
	return s
}

func wordCount(s string) int {
	n := 0
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
			continue
		}
		if !inWord {
			n++
			inWord = true
		}
	}
	return n
}

func buildSubject(topic topics.Topic, body string) string {
	if t := firstTitle(body); t != "" {
		return "Systems daily: " + t
	}
	return "Systems daily: " + topic.Title
}

// firstTitle from HTML <h1> or markdown # / ##.
func firstTitle(body string) string {
	body = strings.TrimSpace(body)
	// HTML h1
	low := strings.ToLower(body)
	if i := strings.Index(low, "<h1"); i >= 0 {
		rest := body[i:]
		if gt := strings.Index(rest, ">"); gt >= 0 {
			rest = rest[gt+1:]
			if end := strings.Index(strings.ToLower(rest), "</h1>"); end >= 0 {
				return strings.TrimSpace(stripTags(rest[:end]))
			}
		}
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
		if strings.HasPrefix(line, "## ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "## "))
		}
	}
	return ""
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

func EmailBody(a *Article, readURL string, pdfAttached bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", a.Subject)
	fmt.Fprintf(&b, "Category: %s\n", a.Topic.Category)
	fmt.Fprintf(&b, "Date: %s\n", a.Generated.Format("2006-01-02"))
	if readURL != "" {
		fmt.Fprintf(&b, "\nRead: %s\n", readURL)
	} else {
		b.WriteString("\n(Read URL not configured - set SITE_BASE_URL.)\n")
	}
	if pdfAttached {
		b.WriteString("\nPDF also attached.\n")
	}
	return b.String()
}

// PlainEmail is kept for callers; prefer EmailBody for mail.
// Deprecated: use EmailBody.
func PlainEmail(a *Article) string {
	return EmailBody(a, "", false)
}

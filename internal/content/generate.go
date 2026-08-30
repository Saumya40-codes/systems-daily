package content

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/Saumya40-codes/systems-daily/internal/llm"
	"github.com/Saumya40-codes/systems-daily/internal/topics"
)

const PipelineVersion = "grounded-v3"

var citationRE = regexp.MustCompile(`\[\[([a-z0-9][a-z0-9-]*)\]\]`)
var modelLinkRE = regexp.MustCompile(`(?i)<a\b|\bhref\s*=|\bsrc\s*=\s*["']\s*//|https?://|\[[^]]+\]\([^)]+\)`)
var modelSourcesHeadingRE = regexp.MustCompile(`(?im)<h2[^>]*>\s*sources\s*</h2>|^##\s+sources\s*$`)
var htmlTagRE = regexp.MustCompile(`(?is)</?(?:h[1-6]|p|pre|code|blockquote|ul|ol|li|table|thead|tbody|tr|th|td|strong|em|b|i|span|div|section|article|br|hr|svg|g|path|rect|circle|line|polyline|polygon|text|defs|marker)\b[^>]*>|<!--.*?-->`)
var htmlCodeBlockRE = regexp.MustCompile(`(?is)<(?:pre|code)\b[^>]*>.*?</(?:pre|code)>`)
var nonProseBlockRE = regexp.MustCompile(`(?is)<(?:script|style|iframe|object|svg)\b[^>]*>.*?</(?:script|style|iframe|object|svg)>|<(?:meta|embed)\b[^>]*>`)
var unclosedNonProseRE = regexp.MustCompile(`(?is)<(?:script|style|iframe|object|svg)\b[^>]*>.*$`)
var fencedCodeRE = regexp.MustCompile("(?s)```.*?```")
var inlineCodeRE = regexp.MustCompile("`[^`]*`")
var h2RE = regexp.MustCompile(`(?im)<h2\b[^>]*>.*?</h2>|^##\s+\S`)
var editorialLeakRE = regexp.MustCompile(`(?i)\b(?:the supplied sources|source packet|evidence brief|technical critique|editorial process|documented mechanism|does not establish|(?:the )?sources (?:establish|support|show|state|document))\b`)

// Article is a reviewed daily write-up and its generation provenance.
type Article struct {
	Topic         topics.Topic  `json:"topic"`
	Subject       string        `json:"subject"`
	Body          string        `json:"body"`
	WordCount     int           `json:"word_count"`
	Model         string        `json:"model"`
	Generated     time.Time     `json:"generated"`
	Pipeline      string        `json:"pipeline"`
	SourceIDs     []string      `json:"source_ids"`
	Draft         string        `json:"draft"`
	ReviewPasses  int           `json:"review_passes"`
	ReviewReasons []string      `json:"review_rejection_reasons,omitempty"`
	Quality       QualityReport `json:"quality"`
}

type QualityReport struct {
	Passed      bool     `json:"passed"`
	Checks      []string `json:"checks"`
	CitationIDs []string `json:"citation_ids"`
}

// Generator runs a grounded draft and one review/rewrite pass.
type Generator struct {
	LLM            llm.Completer
	TargetWordsMin int
	TargetWordsMax int
}

func (g *Generator) Generate(ctx context.Context, topic topics.Topic) (*Article, error) {
	if g.LLM == nil {
		return nil, fmt.Errorf("LLM is required")
	}
	if !topic.ResearchReady() {
		return nil, fmt.Errorf("topic %q is not research-ready: add a core question and curated sources", topic.ID)
	}
	minW, maxW := g.TargetWordsMin, g.TargetWordsMax
	if minW <= 0 {
		minW = 700
	}
	if maxW <= minW {
		maxW = minW + 400
	}

	draft, err := g.chat(ctx, "draft", systemPrompt(minW, maxW), draftPrompt(topic))
	if err != nil {
		return nil, err
	}
	draft = cleanBody(draft)
	draftReport := validateArticle(draft, topic.Sources, minW, maxW)
	body, err := g.chat(ctx, "review", systemPrompt(minW, maxW), reviewPrompt(topic, draft))
	if err != nil {
		return nil, err
	}
	body = cleanBody(body)
	report := validateArticle(body, topic.Sources, minW, maxW)
	if !report.Passed {
		if draftReport.Passed {
			return buildArticle(g, topic, draft, draft, draftReport, report.Checks), nil
		}
		return nil, fmt.Errorf("draft and reviewed article failed quality gates; draft: %s; review: %s", strings.Join(draftReport.Checks, "; "), strings.Join(report.Checks, "; "))
	}

	return buildArticle(g, topic, draft, body, report, nil), nil
}

func buildArticle(g *Generator, topic topics.Topic, draft, body string, report QualityReport, reviewReasons []string) *Article {
	return &Article{
		Topic: topic, Subject: buildSubject(topic, body), Body: body,
		WordCount: wordCount(visibleArticleBody(body)), Model: g.LLM.Label(), Generated: time.Now().UTC(),
		Pipeline: PipelineVersion, SourceIDs: report.CitationIDs,
		Draft: draft, ReviewPasses: 1, ReviewReasons: reviewReasons, Quality: report,
	}
}

func (g *Generator) chat(ctx context.Context, stage, system, user string) (string, error) {
	out, err := g.LLM.Chat(ctx, system, user)
	if err != nil {
		return "", fmt.Errorf("%s: %w", stage, err)
	}
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("%s: empty response", stage)
	}
	return strings.TrimSpace(out), nil
}

func validateArticle(body string, sources []topics.Source, minW, maxW int) QualityReport {
	report := QualityReport{Passed: true}
	fail := func(msg string) { report.Passed = false; report.Checks = append(report.Checks, msg) }
	visibleBody := visibleArticleBody(body)
	visibleStructure := htmlCodeBlockRE.ReplaceAllString(fencedCodeRE.ReplaceAllString(visibleBody, " "), " ")
	wc := wordCount(visibleBody)
	if wc < minW {
		fail(fmt.Sprintf("too short: %d words, minimum %d", wc, minW))
	}
	if wc > maxW {
		fail(fmt.Sprintf("too long: %d words, maximum %d", wc, maxW))
	}
	if firstTitle(visibleStructure) == "" {
		fail("missing H1 title")
	}
	if len(h2RE.FindAllString(visibleStructure, -1)) < 2 {
		fail("fewer than two topic-specific H2 headings")
	}
	if editorialLeakRE.MatchString(body) {
		fail("article exposes the editorial process")
	}
	if modelLinkRE.MatchString(body) {
		fail("model-authored links are not allowed")
	}
	if modelSourcesHeadingRE.MatchString(body) {
		fail("model-authored Sources section is not allowed")
	}
	known := make(map[string]struct{}, len(sources))
	for _, s := range sources {
		known[s.ID] = struct{}{}
	}
	seen := map[string]struct{}{}
	visibleProse := visibleBody
	visibleProse = htmlCodeBlockRE.ReplaceAllString(visibleProse, " ")
	visibleProse = stripTags(visibleProse)
	visibleProse = inlineCodeRE.ReplaceAllString(fencedCodeRE.ReplaceAllString(visibleProse, " "), " ")
	for _, match := range citationRE.FindAllStringSubmatch(visibleProse, -1) {
		id := match[1]
		if _, ok := known[id]; !ok {
			fail("unknown source marker [[" + id + "]]")
			continue
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			report.CitationIDs = append(report.CitationIDs, id)
		}
	}
	markerCount := len(citationRE.FindAllString(visibleProse, -1))
	maxMarkers := wc/80 + len(sources) + 1
	if markerCount > maxMarkers {
		fail(fmt.Sprintf("citation density too high: %d markers, maximum %d", markerCount, maxMarkers))
	}
	if len(report.CitationIDs) == 0 {
		fail("no valid source markers")
	}
	for _, source := range sources {
		if _, ok := seen[source.ID]; !ok {
			fail("source packet is not cited: [[" + source.ID + "]]")
		}
	}
	if report.Passed {
		report.Checks = append(report.Checks, "word count, title, and citations valid")
	}
	return report
}

func visibleArticleBody(body string) string {
	return unclosedNonProseRE.ReplaceAllString(nonProseBlockRE.ReplaceAllString(body, " "), " ")
}

// SaveArtifact atomically archives the sources, intermediate stages, final
// article, model, and quality result used for a publication.
func SaveArtifact(dir string, article *Article) (string, error) {
	if article == nil {
		return "", fmt.Errorf("article is required")
	}
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("artifact directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(article, "", "  ")
	if err != nil {
		return "", err
	}
	prefix := fmt.Sprintf("%s-%s-", article.Generated.Format("2006-01-02T150405.000000000Z"), article.Topic.ID)
	tmp, err := os.CreateTemp(dir, prefix+"*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	path := strings.TrimSuffix(tmpPath, ".tmp") + ".json"
	if err := os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return path, nil
}

func cleanBody(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 {
			first := strings.TrimSpace(lines[0])
			if first == "```" || first == "```markdown" || first == "```md" || first == "```html" {
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
	s = stripTags(s)
	n, inWord := 0, false
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

func firstTitle(body string) string {
	body = strings.TrimSpace(body)
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
	}
	return ""
}

func stripTags(s string) string {
	return htmlTagRE.ReplaceAllString(s, " ")
}

func EmailBody(a *Article, readURL string, pdfAttached bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nCategory: %s\nDate: %s\n", a.Subject, a.Topic.Category, a.Generated.Format("2006-01-02"))
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

// Deprecated: use EmailBody.
func PlainEmail(a *Article) string { return EmailBody(a, "", false) }

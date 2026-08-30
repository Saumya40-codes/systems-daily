package content

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Saumya40-codes/systems-daily/internal/topics"
)

type queuedCompleter struct {
	responses []string
	prompts   []string
	errAt     int
}

func (q *queuedCompleter) Chat(_ context.Context, _, user string) (string, error) {
	q.prompts = append(q.prompts, user)
	if q.errAt > 0 && len(q.prompts) == q.errAt {
		return "", errors.New("provider failed")
	}
	if len(q.responses) < len(q.prompts) {
		return "", errors.New("no queued response")
	}
	return q.responses[len(q.prompts)-1], nil
}

func (q *queuedCompleter) Label() string { return "test-model" }

func sourcedTopic() topics.Topic {
	return topics.Topic{
		ID: "xdp", Title: "XDP", Category: "net", CoreQuestion: "Why early?",
		Sources: []topics.Source{{ID: "official", Title: "Official", URL: "https://example.com/doc", Evidence: "The hook runs early."}},
	}
}

func TestGenerateRunsEditorialStagesAndValidatesFinal(t *testing.T) {
	q := &queuedCompleter{responses: []string{
		"# Evidence brief\n\nUse the early hook [[official]].",
		"# Draft\n\none two three four five [[official]]",
		"# Technical critique\n\nExplain the avoided allocation.",
		"# Final\n\none two three four five six [[official]]",
	}}
	g := Generator{LLM: q, TargetWordsMin: 5, TargetWordsMax: 20}
	a, err := g.Generate(context.Background(), sourcedTopic())
	if err != nil {
		t.Fatal(err)
	}
	if len(q.prompts) != 4 {
		t.Fatalf("got %d calls, want 4", len(q.prompts))
	}
	if !strings.Contains(q.prompts[1], q.responses[0]) || !strings.Contains(q.prompts[3], q.responses[2]) {
		t.Fatal("later stages did not receive earlier editorial work")
	}
	if a.Pipeline != PipelineVersion || a.Model != "test-model" || len(a.SourceIDs) != 1 || !a.Quality.Passed {
		t.Fatalf("unexpected article provenance: %+v", a)
	}
}

func TestGenerateRejectsUnsourcedTopicBeforeCallingLLM(t *testing.T) {
	q := &queuedCompleter{}
	_, err := (&Generator{LLM: q}).Generate(context.Background(), topics.Topic{ID: "empty"})
	if err == nil || !strings.Contains(err.Error(), "not research-ready") {
		t.Fatalf("got %v", err)
	}
	if len(q.prompts) != 0 {
		t.Fatal("LLM should not be called")
	}
}

func TestGenerateRejectsUnknownCitation(t *testing.T) {
	q := &queuedCompleter{responses: []string{"# Brief", "# Draft", "# Critique", "# Final\n\none two three four [[invented]]"}}
	_, err := (&Generator{LLM: q, TargetWordsMin: 3, TargetWordsMax: 20}).Generate(context.Background(), sourcedTopic())
	if err == nil || !strings.Contains(err.Error(), "unknown source marker") {
		t.Fatalf("got %v", err)
	}
}

func TestQualityGateRejectsModelAuthoredLinks(t *testing.T) {
	report := validateArticle("# Final\n\none two [[official]] https://invented.example", sourcedTopic().Sources, 1, 20)
	if report.Passed || !strings.Contains(strings.Join(report.Checks, "; "), "model-authored links") {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestQualityGateRequiresEveryPacketSource(t *testing.T) {
	sources := append(sourcedTopic().Sources, topics.Source{ID: "production", Title: "Case study"})
	report := validateArticle("# Final\n\none two [[official]]", sources, 1, 20)
	if report.Passed || !strings.Contains(strings.Join(report.Checks, "; "), "[[production]]") {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestQualityGateIgnoresHiddenAndCodeCitations(t *testing.T) {
	sources := sourcedTopic().Sources
	for _, body := range []string{
		"# Final\n\none two <!-- [[official]] -->",
		"# Final\n\none two `[[official]]`",
		"<h1>Final</h1><p>one two</p><code>[[official]]</code>",
		`<h1>Final</h1><p title="[[official]]">one two</p>`,
		"<h1>Final</h1><p>one two</p><script>[[official]]</script>",
		`<h1>Final</h1><p>one two</p><meta content="[[official]]">`,
	} {
		report := validateArticle(body, sources, 1, 20)
		if report.Passed {
			t.Fatalf("hidden citation passed: %q", body)
		}
	}
}

func TestGenerateReportsStageFailure(t *testing.T) {
	q := &queuedCompleter{responses: []string{"# Brief", "unused"}, errAt: 2}
	_, err := (&Generator{LLM: q, TargetWordsMin: 1, TargetWordsMax: 20}).Generate(context.Background(), sourcedTopic())
	if err == nil || !strings.Contains(err.Error(), "draft: provider failed") {
		t.Fatalf("got %v", err)
	}
}

func TestWordCountIgnoresHTMLTags(t *testing.T) {
	if got := wordCount(`<h1 class="many fake words">One</h1><p>two three</p>`); got != 3 {
		t.Fatalf("got %d", got)
	}
}

func TestWordCountKeepsComparisonText(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int
	}{{`<p>if x < limit then retry</p>`, 6}, {`<p>if x<limit && y>floor retry</p>`, 5}} {
		if got := wordCount(tc.text); got != tc.want {
			t.Fatalf("got %d, want %d for %q", got, tc.want, tc.text)
		}
	}
}

func TestSaveArtifactIncludesProvenance(t *testing.T) {
	a := &Article{Topic: sourcedTopic(), Pipeline: PipelineVersion, Model: "model", Quality: QualityReport{Passed: true}}
	path, err := SaveArtifact(t.TempDir(), a)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"pipeline": "grounded-v1"`, `"sources"`, `"quality"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("artifact missing %s: %s", want, data)
		}
	}
}

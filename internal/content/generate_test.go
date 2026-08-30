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
		"# Draft\n\none two three four five [[official]]",
		"# Final\n\n## Packet path\n\none two [[official]]\n\n## Tradeoff\n\nthree four five six",
	}}
	g := Generator{LLM: q, TargetWordsMin: 5, TargetWordsMax: 20}
	a, err := g.Generate(context.Background(), sourcedTopic())
	if err != nil {
		t.Fatal(err)
	}
	if len(q.prompts) != 2 {
		t.Fatalf("got %d calls, want 2", len(q.prompts))
	}
	if !strings.Contains(q.prompts[1], q.responses[0]) {
		t.Fatal("review did not receive the draft")
	}
	if a.Pipeline != PipelineVersion || a.Model != "test-model" || a.ReviewPasses != 1 || len(a.SourceIDs) != 1 || !a.Quality.Passed {
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
	bad := "# Final\n\n## Path\n\none two [[invented]]\n\n## Cost\n\nthree four"
	q := &queuedCompleter{responses: []string{"# Draft", bad}}
	_, err := (&Generator{LLM: q, TargetWordsMin: 3, TargetWordsMax: 20}).Generate(context.Background(), sourcedTopic())
	if err == nil || !strings.Contains(err.Error(), "unknown source marker") {
		t.Fatalf("got %v", err)
	}
}

func TestGenerateFallsBackToValidDraftWithoutThirdCall(t *testing.T) {
	draft := "# Draft\n\n## Path\n\none two [[official]]\n\n## Limit\n\nthree four"
	q := &queuedCompleter{responses: []string{
		draft,
		"# Final\n\none two [[official]]",
	}}
	a, err := (&Generator{LLM: q, TargetWordsMin: 3, TargetWordsMax: 20}).Generate(context.Background(), sourcedTopic())
	if err != nil {
		t.Fatal(err)
	}
	if len(q.prompts) != 2 || a.Body != draft || len(a.ReviewReasons) == 0 {
		t.Fatalf("draft fallback missing: calls=%d article=%+v", len(q.prompts), a)
	}
}

func TestQualityGateRejectsModelAuthoredLinks(t *testing.T) {
	report := validateArticle("# Final\n\none two [[official]] https://invented.example", sourcedTopic().Sources, 1, 20)
	if report.Passed || !strings.Contains(strings.Join(report.Checks, "; "), "model-authored links") {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestQualityGateRejectsEditorialLeakAndCitationNoise(t *testing.T) {
	body := "# Final\n\n## Path\n\nThe supplied sources say one [[official]] two [[official]] three [[official]].\n\n## Cost\n\nfour."
	report := validateArticle(body, sourcedTopic().Sources, 1, 30)
	checks := strings.Join(report.Checks, "; ")
	for _, want := range []string{"editorial process", "citation density"} {
		if !strings.Contains(checks, want) {
			t.Fatalf("missing %q in %+v", want, report)
		}
	}
}

func TestQualityGateIgnoresHiddenLengthAndHeadings(t *testing.T) {
	for _, tc := range []struct {
		body      string
		wantShort bool
	}{
		{"# Final\n\none [[official]]<script><h2>Hidden one</h2><h2>Hidden two</h2> two three four five</script>", true},
		{"# Final\n\none [[official]]<script><h2>Hidden one</h2><h2>Hidden two</h2> two three four five", true},
		{"# Final\n\none [[official]]\n```markdown\n## Hidden one\n## Hidden two\ntwo three four five\n```", false},
	} {
		report := validateArticle(tc.body, sourcedTopic().Sources, 205, 220)
		checks := strings.Join(report.Checks, "; ")
		if !strings.Contains(checks, "fewer than two") {
			t.Fatalf("hidden headings passed for %q: %+v", tc.body, report)
		}
		if tc.wantShort && !strings.Contains(checks, "too short") {
			t.Fatalf("hidden words passed for %q: %+v", tc.body, report)
		}
	}
}

func TestQualityGateRejectsSourceCommentary(t *testing.T) {
	body := "# Final\n\n## Path\n\none two [[official]].\n\n## Limit\n\nThe sources establish the mechanism."
	report := validateArticle(body, sourcedTopic().Sources, 1, 30)
	if report.Passed || !strings.Contains(strings.Join(report.Checks, "; "), "editorial process") {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestQualityGateAllowsNaturalLimitationLanguage(t *testing.T) {
	body := "# Final\n\n## Path\n\none two [[official]].\n\n## Limit\n\nThe documented mechanism does not establish a fixed cost."
	report := validateArticle(body, sourcedTopic().Sources, 1, 30)
	if !report.Passed {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestQualityGateAllowsWordCountTolerance(t *testing.T) {
	within := "# Final\n\n## Path\n\n" + strings.Repeat("word ", 1220) + "[[official]]\n\n## Limit\n\nend"
	if report := validateArticle(within, sourcedTopic().Sources, 700, 1200); !report.Passed {
		t.Fatalf("article within tolerance failed: %+v", report)
	}
	over := "# Final\n\n## Path\n\n" + strings.Repeat("word ", 1410) + "[[official]]\n\n## Limit\n\nend"
	report := validateArticle(over, sourcedTopic().Sources, 700, 1200)
	if report.Passed || !strings.Contains(strings.Join(report.Checks, "; "), "maximum 1400") {
		t.Fatalf("article beyond tolerance passed: %+v", report)
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
	q := &queuedCompleter{responses: []string{"# Draft", "unused"}, errAt: 2}
	_, err := (&Generator{LLM: q, TargetWordsMin: 1, TargetWordsMax: 20}).Generate(context.Background(), sourcedTopic())
	if err == nil || !strings.Contains(err.Error(), "review: provider failed") {
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
	for _, want := range []string{`"pipeline": "grounded-v3"`, `"sources"`, `"quality"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("artifact missing %s: %s", want, data)
		}
	}
}

package diagrams

import (
	"context"
	"strings"
	"testing"
)

func TestProcessReplacesSVGFence(t *testing.T) {
	md := `# Title

Some intro.

` + "```svg\n" + `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 40">
  <rect x="5" y="5" width="90" height="30" fill="#ddd" stroke="#333"/>
  <text x="50" y="25" text-anchor="middle" font-size="10">hi</text>
</svg>
` + "```" + `

After.`

	res, err := Process(context.Background(), md)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Markdown, "{{figure:0}}") {
		t.Fatalf("expected figure marker, got:\n%s", res.Markdown)
	}
	if len(res.Figures) != 1 {
		t.Fatalf("figures=%d", len(res.Figures))
	}
	if len(res.Figures[0].PNG) < 50 {
		t.Fatalf("png too small: %d", len(res.Figures[0].PNG))
	}
	if res.Figures[0].Kind != "svg" {
		t.Fatalf("kind=%s", res.Figures[0].Kind)
	}
}

func TestParseFigureMarker(t *testing.T) {
	n, ok := ParseFigureMarker("{{figure:3}}")
	if !ok || n != 3 {
		t.Fatalf("got %d %v", n, ok)
	}
	if _, ok := ParseFigureMarker("nope"); ok {
		t.Fatal("expected false")
	}
}

func TestProcessLeavesPlainMarkdown(t *testing.T) {
	md := "## Hello\n\nNo diagrams here.\n"
	res, err := Process(context.Background(), md)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Figures) != 0 {
		t.Fatalf("unexpected figures: %d", len(res.Figures))
	}
	if res.Markdown != md {
		t.Fatalf("markdown changed: %q", res.Markdown)
	}
}

func TestProcessDropsFailedSVGWithoutSource(t *testing.T) {
	// Truncated XML fails oksvg parse.
	md := "Before\n\n```svg\n<svg xmlns=\"http://www.w3.org/2000/svg\"\n```\n\nAfter\n"
	res, err := Process(context.Background(), md)
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 {
		t.Fatalf("Failed=%d want 1", res.Failed)
	}
	if len(res.Figures) != 0 {
		t.Fatalf("unexpected figures: %d", len(res.Figures))
	}
	if strings.Contains(res.Markdown, "```") || strings.Contains(res.Markdown, "<svg") {
		t.Fatalf("failed source should not remain in markdown:\n%s", res.Markdown)
	}
	if strings.Contains(res.Markdown, "{{figure:") {
		t.Fatalf("should not emit figure marker:\n%s", res.Markdown)
	}
	if !strings.Contains(res.Markdown, "Before") || !strings.Contains(res.Markdown, "After") {
		t.Fatalf("surrounding prose missing:\n%s", res.Markdown)
	}
}

func TestProcessDropsMermaidWithoutNetwork(t *testing.T) {
	md := "Before\n\n```mermaid\nflowchart LR\n  A --> B\n```\n\nAfter\n"
	res, err := Process(context.Background(), md)
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 {
		t.Fatalf("Failed=%d want 1", res.Failed)
	}
	if len(res.Figures) != 0 {
		t.Fatalf("unexpected figures: %d", len(res.Figures))
	}
	if strings.Contains(res.Markdown, "mermaid") || strings.Contains(res.Markdown, "flowchart") {
		t.Fatalf("mermaid should be stripped:\n%s", res.Markdown)
	}
	if !strings.Contains(res.Markdown, "Before") || !strings.Contains(res.Markdown, "After") {
		t.Fatalf("surrounding prose missing:\n%s", res.Markdown)
	}
}

func TestProcessSVGFenceCaseInsensitive(t *testing.T) {
	md := "Before\n\n```SVG\n<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 40 20\"><rect width=\"40\" height=\"20\" fill=\"#ccc\"/></svg>\n```\n\nAfter\n"
	res, err := Process(context.Background(), md)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Figures) != 1 {
		t.Fatalf("figures=%d failed=%d md=%s", len(res.Figures), res.Failed, res.Markdown)
	}
}

func TestProcessMermaidCaseInsensitive(t *testing.T) {
	md := "```Mermaid\nflowchart LR\n  A --> B\n```\n"
	res, err := Process(context.Background(), md)
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 || len(res.Figures) != 0 {
		t.Fatalf("Failed=%d figures=%d", res.Failed, len(res.Figures))
	}
}

func TestRenderSVGRejectsHugeViewBox(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 100000000"><rect width="10" height="10"/></svg>`
	_, err := renderSVG(src, 900, 600)
	if err == nil {
		t.Fatal("expected error for huge viewBox")
	}
}

func TestRenderSVGCapsOutputPixels(t *testing.T) {
	// Wide SVG; should scale into max bounds without panic.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 5000 500"><rect width="5000" height="500" fill="#eee"/></svg>`
	png, err := renderSVG(src, 900, 600)
	if err != nil {
		t.Fatal(err)
	}
	if len(png) < 50 {
		t.Fatalf("png too small: %d", len(png))
	}
}

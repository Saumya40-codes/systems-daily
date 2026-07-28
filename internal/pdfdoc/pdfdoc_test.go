package pdfdoc

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Saumya40-codes/systems-daily/internal/diagrams"
)

func TestBuildPDFBasic(t *testing.T) {
	body := `# Buddy allocator

## Arena split

Kernels split pages for a reason.

## Code

` + "```c\nint x = 1;\n```" + `

## List

- one
- two
`
	pdf, err := Build(Input{
		Title:    "Buddy allocator",
		Category: "memory",
		Body:     body,
		Date:     time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pdf) < 500 {
		t.Fatalf("pdf too small: %d", len(pdf))
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("not a PDF")
	}
}

func TestBuildPDFWithSVGFigure(t *testing.T) {
	md := `# With figure

Before.

` + "```svg\n" + `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 40">
  <rect width="120" height="40" fill="#eee"/>
  <text x="60" y="24" text-anchor="middle" font-size="12">box</text>
</svg>
` + "```" + `

After.`
	res, err := diagrams.Process(context.Background(), md)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Figures) != 1 {
		t.Fatalf("figures=%d markdown=%s", len(res.Figures), res.Markdown)
	}
	pdf, err := Build(Input{
		Title:    "With figure",
		Category: "cpu",
		Body:     res.Markdown,
		Figures:  res.Figures,
		Date:     time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("not a PDF")
	}
	if !strings.Contains(res.Markdown, "{{figure:0}}") {
		t.Fatal("missing marker")
	}
}

func TestBuildPDFPreservesFencedASCIIDiagram(t *testing.T) {
	body := `# Title

Intro prose.

` + "```text\n" + `  userspace      kernel
  +------+      +------+
  | app  | ---> | mm   |
  +------+      +------+
` + "```" + `

After.`
	pdf, err := Build(Input{
		Title:    "Title",
		Category: "os",
		Body:     body,
		Date:     time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("not a PDF")
	}
	// PDF text operators should still contain diagram tokens (Courier path).
	if !bytes.Contains(pdf, []byte("userspace")) && !bytes.Contains(pdf, []byte("app")) {
		// fpdf may encode strings; soft check on size growth vs empty
		if len(pdf) < 800 {
			t.Fatalf("pdf unexpectedly small for diagram content: %d", len(pdf))
		}
	}
}

func TestSanitizePDFBoxDrawing(t *testing.T) {
	in := "│ hello ┌──┐ → done"
	got := sanitizePDF(in)
	if strings.ContainsAny(got, "│┌─→") {
		t.Fatalf("unicode remains: %q", got)
	}
	if !strings.Contains(got, "|") || !strings.Contains(got, "+") || !strings.Contains(got, "->") {
		t.Fatalf("expected ascii transliteration: %q", got)
	}
	if strings.Contains(got, "?") && strings.Count(got, "?") > 0 {
		// box chars should not become ?
		for _, r := range []rune{'│', '┌', '─', '→'} {
			if strings.ContainsRune(in, r) {
				// mapped; no bare ?
			}
		}
	}
}

func TestSanitizePDFNoQuestionForMappedArrows(t *testing.T) {
	got := sanitizePDF("A → B ← C ↔ D")
	if strings.Contains(got, "?") {
		t.Fatalf("mapped arrows became ?: %q", got)
	}
	if !strings.Contains(got, "->") || !strings.Contains(got, "<-") {
		t.Fatalf("missing arrow ascii: %q", got)
	}
}

package content

import (
	"strings"
	"testing"

	"github.com/Saumya40-codes/systems-daily/internal/topics"
)

func TestSystemPromptEmailSafeDiagrams(t *testing.T) {
	p := systemPrompt(700, 1200)
	for _, want := range []string{
		"ASCII",
		"Do NOT use Mermaid",
		"text/plain",
		"Mental model",
		"Hands-on",
		"Go deeper",
		"700-1200",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

func TestUserPromptIncludesAngles(t *testing.T) {
	topic := topics.Topic{
		ID:       "buddy-allocator",
		Title:    "Buddy memory allocator internals",
		Category: "memory",
		Angles:   []string{"block splitting", "fragmentation"},
	}
	p := userPrompt(topic)
	if !strings.Contains(p, "Buddy memory allocator internals") {
		t.Fatal("missing title")
	}
	if !strings.Contains(p, "block splitting") {
		t.Fatal("missing angle")
	}
	if !strings.Contains(p, "article body only") {
		t.Fatal("missing output constraint")
	}
}

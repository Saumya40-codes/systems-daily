package content

import (
	"strings"
	"testing"

	"github.com/Saumya40-codes/systems-daily/internal/topics"
)

func TestSystemPromptIsLooseTaste(t *testing.T) {
	p := systemPrompt(700, 1200)
	for _, want := range []string{
		"narrow slice",
		"plain English",
		"HTML fragment",
		"Visuals",
		"700-1200",
		"simple and direct",
		"fancy or rare words",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, bad := range []string{
		"at least two specific footguns",
		"Depth contract",
		"Self-check",
	} {
		if strings.Contains(p, bad) {
			t.Errorf("prompt still mandates structure %q", bad)
		}
	}
}

func TestUserPromptNoFixedSections(t *testing.T) {
	topic := topics.Topic{
		ID:       "watchdogs",
		Title:    "Windowed WDT: kick too early vs too late",
		Category: "embedded",
		Angles:   []string{"open/close window"},
	}
	p := userPrompt(topic)
	if !strings.Contains(p, "Simple English") {
		t.Fatal("should ask for simple English")
	}
	if !strings.Contains(p, "Windowed WDT") {
		t.Fatal("missing title")
	}
}

func TestEmailBodyOmitsWordCountAndModel(t *testing.T) {
	a := &Article{
		Topic:     topics.Topic{Category: "memory", Title: "Buddy"},
		Subject:   "Systems daily: Buddy",
		Body:      "# Buddy\n\nHello.",
		WordCount: 999,
		Model:     "llama-secret",
	}
	body := EmailBody(a, "https://example.com/today/", false)
	if strings.Contains(body, "999") || strings.Contains(body, "words") {
		t.Fatalf("word count leaked: %q", body)
	}
	if strings.Contains(body, "llama-secret") {
		t.Fatalf("model leaked: %q", body)
	}
	if !strings.Contains(body, "https://example.com/today/") {
		t.Fatalf("expected URL: %q", body)
	}
}

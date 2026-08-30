package llm

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIClientChat(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-llm")
	// Reads stdin, checks markers, prints a fixed article.
	body := `#!/bin/sh
set -e
input=$(cat)
echo "$input" | grep -q "### SYSTEM" || exit 2
echo "$input" | grep -q "### USER" || exit 3
test -n "$SYSTEMS_DAILY_SYSTEM" || exit 4
test -n "$SYSTEMS_DAILY_USER" || exit 5
printf '%s\n' '<h1>CLI note</h1><p>ok</p>'
`
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	c := NewCLI(script, nil)
	out, err := c.Chat(context.Background(), "sys prompt", "user prompt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CLI note") {
		t.Fatalf("got %q", out)
	}
	if c.Label() != "cli:"+script {
		t.Fatalf("label %q", c.Label())
	}
}

func TestNewCompleterCLIRequiresCmd(t *testing.T) {
	_, err := NewCompleter(Config{Provider: "cli"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewCompleterCLIIncludesModelProvenance(t *testing.T) {
	c, err := NewCompleter(Config{Provider: "cli", CLICommand: "codex-wrapper", CLIModel: "gpt-5"})
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Label(); got != "cli:codex-wrapper (gpt-5)" {
		t.Fatalf("got %q", got)
	}
}

func TestNewCompleterHTTP(t *testing.T) {
	c, err := NewCompleter(Config{Provider: "http", BaseURL: "http://x", APIKey: "k", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Label() != "m" {
		t.Fatal(c.Label())
	}
}

func TestFormatCLIPrompt(t *testing.T) {
	s := formatCLIPrompt("S", "U")
	if !strings.Contains(s, "### SYSTEM\nS\n") || !strings.Contains(s, "### USER\nU\n") {
		t.Fatal(s)
	}
}

func TestStripCLIPreamble(t *testing.T) {
	in := "I'll check the project first.\n\n<h1>Title</h1><p>body</p>"
	got := stripCLIPreamble(in)
	if !strings.HasPrefix(got, "<h1>") {
		t.Fatalf("got %q", got)
	}
	md := "Thinking...\n\n# Hello\n\npara"
	got = stripCLIPreamble(md)
	if !strings.HasPrefix(got, "# Hello") {
		t.Fatalf("got %q", got)
	}
}

func TestLooksLikeAgentNarration(t *testing.T) {
	if !looksLikeAgentNarration("Checking how systems-daily articles are structured so the note matches the project format.") {
		t.Fatal("expected narration")
	}
	if looksLikeAgentNarration("<h1>WDT</h1><p>A windowed watchdog...</p>") {
		t.Fatal("article should pass")
	}
}

func TestSafeCLIEnvRemovesApplicationSecrets(t *testing.T) {
	got := safeCLIEnv([]string{
		"PATH=/bin",
		"HOME=/home/test",
		"OPENAI_API_KEY=codex-auth",
		"LLM_API_KEY=article-provider-secret",
		"SMTP_PASS=mail-secret",
		"SMTP_USER=user@example.com",
		"SYSTEMS_DAILY_USER=old-prompt",
	})
	joined := strings.Join(got, "\n")
	for _, want := range []string{"PATH=/bin", "HOME=/home/test", "OPENAI_API_KEY=codex-auth"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q from %q", want, joined)
		}
	}
	for _, secret := range []string{"article-provider-secret", "mail-secret", "user@example.com", "old-prompt"} {
		if strings.Contains(joined, secret) {
			t.Errorf("leaked %q in %q", secret, joined)
		}
	}
}

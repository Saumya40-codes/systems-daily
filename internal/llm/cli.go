package llm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type CLIClient struct {
	Command string
	Args    []string
	// Timeout caps a single completion (0 = 10m default).
	Timeout time.Duration
}

// NewCLI builds a CLI completer.
func NewCLI(command string, args []string) *CLIClient {
	return &CLIClient{
		Command: command,
		Args:    append([]string(nil), args...),
		Timeout: 10 * time.Minute,
	}
}

func (c *CLIClient) Label() string {
	base := strings.TrimSpace(c.Command)
	if base == "" {
		return "cli"
	}
	return "cli:" + base
}

// Chat runs the command with system/user on stdin and env; returns stdout.
func (c *CLIClient) Chat(ctx context.Context, system, user string) (string, error) {
	if strings.TrimSpace(c.Command) == "" {
		return "", fmt.Errorf("LLM_CLI_CMD is empty")
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.Command, c.Args...)
	cmd.Env = append(os.Environ(),
		"SYSTEMS_DAILY_SYSTEM="+system,
		"SYSTEMS_DAILY_USER="+user,
	)
	cmd.Stdin = strings.NewReader(formatCLIPrompt(system, user))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errText := strings.TrimSpace(stderr.String())
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("LLM CLI timed out after %s: %s", timeout, c.Command)
	}
	if err != nil {
		if errText != "" {
			return "", fmt.Errorf("LLM CLI %q failed: %w\nstderr: %s", c.Command, err, truncate(errText, 800))
		}
		return "", fmt.Errorf("LLM CLI %q failed: %w", c.Command, err)
	}
	if out == "" {
		if errText != "" {
			return "", fmt.Errorf("LLM CLI %q returned empty stdout (stderr: %s)", c.Command, truncate(errText, 400))
		}
		return "", fmt.Errorf("LLM CLI %q returned empty stdout", c.Command)
	}
	// Agent CLIs sometimes print planning chatter before the article.
	out = stripCLIPreamble(out)
	if out == "" {
		return "", fmt.Errorf("LLM CLI %q: empty article after stripping preamble", c.Command)
	}
	if looksLikeAgentNarration(out) {
		return "", fmt.Errorf("LLM CLI %q returned agent chatter, not an article (got: %s)", c.Command, truncate(out, 120))
	}
	return out, nil
}

// looksLikeAgentNarration detects planning-only replies (no real article body).
func looksLikeAgentNarration(s string) bool {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	// Real article signals
	if strings.Contains(low, "<h1") || strings.Contains(low, "<p") || strings.Contains(low, "<pre") {
		return false
	}
	if strings.HasPrefix(low, "# ") || strings.Contains(s, "\n# ") {
		return false
	}
	// Short plain text that starts with planning verbs
	for _, p := range []string{
		"checking ", "i'll ", "i will ", "let me ", "looking at ", "reading ",
		"i need to ", "first,", "searching ", "exploring ",
	} {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	// Very short with no structure
	if len(s) < 200 && !strings.Contains(s, "\n\n") {
		return true
	}
	return false
}

// stripCLIPreamble drops leading agent narration before HTML or markdown body.
func stripCLIPreamble(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// HTML fragment: first block tag
	for _, tag := range []string{"<h1", "<h2", "<p", "<pre", "<ul", "<ol", "<article", "<div", "<section"} {
		if i := strings.Index(strings.ToLower(s), tag); i >= 0 {
			return strings.TrimSpace(s[i:])
		}
	}
	// Markdown: first ATX heading or fenced block at line start
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "# ") || strings.HasPrefix(t, "## ") || strings.HasPrefix(t, "```") {
			return strings.TrimSpace(strings.Join(lines[i:], "\n"))
		}
	}
	return s
}

func formatCLIPrompt(system, user string) string {
	var b strings.Builder
	b.WriteString("### SYSTEM\n")
	b.WriteString(system)
	if !strings.HasSuffix(system, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("\n### USER\n")
	b.WriteString(user)
	if !strings.HasSuffix(user, "\n") {
		b.WriteByte('\n')
	}
	return b.String()
}

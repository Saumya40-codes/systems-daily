package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime settings. Prefer env vars; optional .env is loaded by main.
type Config struct {
	// LLM
	LLMProvider string // http (default) | cli
	// HTTP (OpenAI-compatible: Ollama, LM Studio, OpenRouter, Groq, xAI, ...)
	LLMBaseURL string
	LLMAPIKey  string
	LLMModel   string
	// CLI (local command; stdout = article). See internal/llm CLI protocol.
	LLMCLICmd  string
	LLMCLIArgs string // space-separated extra args, e.g. "-p"

	// Email
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string
	SMTPTo       string
	SMTPUseTLS   bool // STARTTLS on 587; false for plain (rare)
	SMTPInsecure bool // skip TLS verify (local dev only)

	// Schedule: local wall-clock time, e.g. "09:00"
	SendAt   string
	Timezone string // IANA, e.g. "Asia/Kolkata"; empty = local

	// Content
	TargetWordsMin int
	TargetWordsMax int
	HistoryPath    string
	HistoryWindow  int    // don't reuse a topic within N days
	TopicsPath     string // empty = embedded default catalog

	// Static site (primary reading surface)
	SiteOutDir     string // web root written each run, e.g. site/public
	SiteBaseURL    string // public origin for email links, e.g. https://....vercel.app
	SiteWindowDays int    // dated pages kept (including today); default 7

	// Optional PDF attach (site is primary; PDF off by default)
	AttachPDF bool

	// Behavior
	DryRun bool
}

func Load() (*Config, error) {
	cfg := &Config{
		LLMProvider:    env("LLM_PROVIDER", "http"),
		LLMBaseURL:     env("LLM_BASE_URL", "http://localhost:11434/v1"),
		LLMAPIKey:      env("LLM_API_KEY", "ollama"), // Ollama ignores key but client may require one
		LLMModel:       env("LLM_MODEL", "llama3.2"),
		LLMCLICmd:      env("LLM_CLI_CMD", ""),
		LLMCLIArgs:     env("LLM_CLI_ARGS", ""),
		SMTPHost:       env("SMTP_HOST", ""),
		SMTPPort:       envInt("SMTP_PORT", 587),
		SMTPUser:       env("SMTP_USER", ""),
		SMTPPass:       env("SMTP_PASS", ""),
		SMTPFrom:       env("SMTP_FROM", ""),
		SMTPTo:         env("SMTP_TO", ""),
		SMTPUseTLS:     envBool("SMTP_USE_TLS", true),
		SMTPInsecure:   envBool("SMTP_INSECURE", false),
		SendAt:         env("SEND_AT", "09:00"),
		Timezone:       env("TIMEZONE", ""),
		TargetWordsMin: envInt("TARGET_WORDS_MIN", 700),
		TargetWordsMax: envInt("TARGET_WORDS_MAX", 1200),
		HistoryPath:    env("HISTORY_PATH", "data/history.json"),
		HistoryWindow:  envInt("HISTORY_WINDOW_DAYS", 60),
		TopicsPath:     env("TOPICS_PATH", ""),
		SiteOutDir:     env("SITE_OUT_DIR", "site/public"),
		SiteBaseURL:    env("SITE_BASE_URL", ""),
		SiteWindowDays: envInt("SITE_WINDOW_DAYS", 7),
		AttachPDF:      envBool("ATTACH_PDF", false),
		DryRun:         envBool("DRY_RUN", false),
	}

	if _, err := time.Parse("15:04", cfg.SendAt); err != nil {
		return nil, fmt.Errorf("SEND_AT must be HH:MM (24h), got %q: %w", cfg.SendAt, err)
	}
	if cfg.Timezone != "" {
		if _, err := time.LoadLocation(cfg.Timezone); err != nil {
			return nil, fmt.Errorf("invalid TIMEZONE %q: %w", cfg.Timezone, err)
		}
	}
	if cfg.SiteWindowDays < 1 {
		return nil, fmt.Errorf("SITE_WINDOW_DAYS must be >= 1, got %d", cfg.SiteWindowDays)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.LLMProvider)) {
	case "", "http", "openai", "api", "cli", "command", "exec":
	default:
		return nil, fmt.Errorf("LLM_PROVIDER must be http or cli, got %q", cfg.LLMProvider)
	}
	return cfg, nil
}

// Location returns the schedule timezone (local if unset).
func (c *Config) Location() *time.Location {
	if c.Timezone == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return time.Local
	}
	return loc
}

// ValidateSMTP returns an error if required mail settings are missing.
func (c *Config) ValidateSMTP() error {
	missing := []string{}
	if c.SMTPHost == "" {
		missing = append(missing, "SMTP_HOST")
	}
	if c.SMTPFrom == "" {
		missing = append(missing, "SMTP_FROM")
	}
	if c.SMTPTo == "" {
		missing = append(missing, "SMTP_TO")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required email config: %s", strings.Join(missing, ", "))
	}
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

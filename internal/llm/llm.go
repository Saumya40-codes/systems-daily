package llm

import (
	"context"
	"fmt"
	"strings"
)

// Completer produces assistant text from a system + user prompt pair.
type Completer interface {
	Chat(ctx context.Context, system, user string) (string, error)
	// Label is stored on the article / logs (model id or "cli:...").
	Label() string
}

// Provider names for LLM_PROVIDER.
const (
	ProviderHTTP = "http"
	ProviderCLI  = "cli"
)

// Config selects and configures a Completer.
type Config struct {
	Provider string // http (default) | cli

	// HTTP (OpenAI-compatible)
	BaseURL string
	APIKey  string
	Model   string

	CLICommand string   // required if provider=cli (e.g. "claude" or path to a script)
	CLIArgs    []string // extra args before the command reads stdin (e.g. "-p")
}

// NewCompleter builds an HTTP or CLI completer from Config.
func NewCompleter(cfg Config) (Completer, error) {
	p := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if p == "" {
		p = ProviderHTTP
	}
	switch p {
	case ProviderHTTP, "openai", "api":
		if cfg.Model == "" {
			return nil, fmt.Errorf("LLM_MODEL is empty")
		}
		return NewHTTP(cfg.BaseURL, cfg.APIKey, cfg.Model), nil
	case ProviderCLI, "command", "exec":
		if strings.TrimSpace(cfg.CLICommand) == "" {
			return nil, fmt.Errorf("LLM_PROVIDER=cli requires LLM_CLI_CMD")
		}
		return NewCLI(cfg.CLICommand, cfg.CLIArgs), nil
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER %q (use http or cli)", cfg.Provider)
	}
}

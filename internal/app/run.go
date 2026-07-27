package app

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Saumya40-codes/systems-daily/internal/config"
	"github.com/Saumya40-codes/systems-daily/internal/content"
	"github.com/Saumya40-codes/systems-daily/internal/email"
	"github.com/Saumya40-codes/systems-daily/internal/history"
	"github.com/Saumya40-codes/systems-daily/internal/llm"
	"github.com/Saumya40-codes/systems-daily/internal/topics"
)

// Options control a single generate/send cycle.
type Options struct {
	TopicID string // optional forced topic
	Preview bool   // print only, still records history unless SkipHistory
	SkipHistory bool
}

// RunOnce picks a topic, generates a write-up, and emails (or prints) it.
func RunOnce(ctx context.Context, cfg *config.Config, opt Options) error {
	hist, err := history.Open(cfg.HistoryPath)
	if err != nil {
		return fmt.Errorf("history: %w", err)
	}

	var topic topics.Topic
	if opt.TopicID != "" {
		topic, err = topics.ByID(opt.TopicID)
		if err != nil {
			return err
		}
	} else {
		recent := hist.RecentIDs(cfg.HistoryWindow)
		topic, err = topics.Pick(recent, nil)
		if err != nil {
			return err
		}
	}

	log.Printf("topic: %s [%s] (%s)", topic.Title, topic.Category, topic.ID)

	client := llm.New(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)
	gen := &content.Generator{
		LLM:            client,
		TargetWordsMin: cfg.TargetWordsMin,
		TargetWordsMax: cfg.TargetWordsMax,
	}

	article, err := gen.Generate(ctx, topic)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	log.Printf("generated ~%d words · subject: %s", article.WordCount, article.Subject)

	body := content.PlainEmail(article)

	if opt.Preview || cfg.DryRun {
		fmt.Println(body)
		if opt.Preview {
			log.Printf("preview only - not sending email")
		}
		if cfg.DryRun {
			log.Printf("DRY_RUN=1 - not sending email")
		}
		if !opt.SkipHistory && !opt.Preview {
			_ = hist.Append(history.Entry{
				TopicID:   topic.ID,
				Title:     topic.Title,
				Category:  topic.Category,
				WordCount: article.WordCount,
				Subject:   article.Subject,
			})
		}
		return nil
	}

	if err := cfg.ValidateSMTP(); err != nil {
		return err
	}

	to := splitAddrs(cfg.SMTPTo)
	err = email.Send(email.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		User:     cfg.SMTPUser,
		Pass:     cfg.SMTPPass,
		UseTLS:   cfg.SMTPUseTLS,
		Insecure: cfg.SMTPInsecure,
	}, email.Message{
		From:    cfg.SMTPFrom,
		To:      to,
		Subject: article.Subject,
		Body:    body,
	})
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	log.Printf("email sent to %s", strings.Join(to, ", "))

	if !opt.SkipHistory {
		if err := hist.Append(history.Entry{
			TopicID:   topic.ID,
			Title:     topic.Title,
			Category:  topic.Category,
			WordCount: article.WordCount,
			Subject:   article.Subject,
		}); err != nil {
			return fmt.Errorf("record history: %w", err)
		}
	}
	return nil
}

func splitAddrs(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

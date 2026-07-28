package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Saumya40-codes/systems-daily/internal/config"
	"github.com/Saumya40-codes/systems-daily/internal/content"
	"github.com/Saumya40-codes/systems-daily/internal/diagrams"
	"github.com/Saumya40-codes/systems-daily/internal/email"
	"github.com/Saumya40-codes/systems-daily/internal/history"
	"github.com/Saumya40-codes/systems-daily/internal/llm"
	"github.com/Saumya40-codes/systems-daily/internal/pdfdoc"
	"github.com/Saumya40-codes/systems-daily/internal/site"
	"github.com/Saumya40-codes/systems-daily/internal/topics"
)

// Options control a single generate/send cycle.
type Options struct {
	TopicID     string // optional forced topic
	Preview     bool   // generate + publish site (+ optional PDF file); never records history / no mail
	SkipHistory bool   // skip history even when sending (or dry-run without preview)
}

// RunOnce picks a topic, generates a write-up, publishes the static site, and emails a link.
func RunOnce(ctx context.Context, cfg *config.Config, opt Options) error {
	catalog, err := topics.Load(cfg.TopicsPath)
	if err != nil {
		return fmt.Errorf("topics: %w", err)
	}

	hist, err := history.Open(cfg.HistoryPath)
	if err != nil {
		return fmt.Errorf("history: %w", err)
	}

	var topic topics.Topic
	if opt.TopicID != "" {
		topic, err = catalog.ByID(opt.TopicID)
		if err != nil {
			return err
		}
	} else {
		recent := hist.RecentIDs(cfg.HistoryWindow)
		topic, err = catalog.Pick(recent, nil)
		if err != nil {
			return err
		}
	}

	log.Printf("topic: %s [%s] (%s)", topic.Title, topic.Category, topic.ID)

	completer, err := llm.NewCompleter(llm.Config{
		Provider:   cfg.LLMProvider,
		BaseURL:    cfg.LLMBaseURL,
		APIKey:     cfg.LLMAPIKey,
		Model:      cfg.LLMModel,
		CLICommand: cfg.LLMCLICmd,
		CLIArgs:    splitCLIArgs(cfg.LLMCLIArgs),
	})
	if err != nil {
		return fmt.Errorf("llm: %w", err)
	}
	log.Printf("llm: %s", completer.Label())
	gen := &content.Generator{
		LLM:            completer,
		TargetWordsMin: cfg.TargetWordsMin,
		TargetWordsMax: cfg.TargetWordsMax,
	}

	article, err := gen.Generate(ctx, topic)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	log.Printf("generated ~%d words · subject: %s", article.WordCount, article.Subject)

	// Use schedule timezone for the page date when set.
	now := article.Generated.In(cfg.Location())
	pageTitle := site.TitleFromBody(article.Body, topic.Title)

	pub, err := site.Publish(site.Page{
		Title:        pageTitle,
		Category:     topic.Category,
		Date:         now,
		BodyMarkdown: article.Body,
		Subject:      article.Subject,
	}, site.PublishOptions{
		OutDir:     cfg.SiteOutDir,
		WindowDays: cfg.SiteWindowDays,
		Now:        now,
	})
	if err != nil {
		return fmt.Errorf("site: %w", err)
	}
	log.Printf("site published under %s (today=%s date=%s)", cfg.SiteOutDir, pub.TodayPath, pub.DatePath)
	if len(pub.Pruned) > 0 {
		log.Printf("site pruned %d old day(s): %s", len(pub.Pruned), strings.Join(pub.Pruned, ", "))
	}

	readURL := site.PublicURL(cfg.SiteBaseURL, pub.TodayPath)
	// Prefer absolute /today for mail when base is set; also mention dated path in logs.
	if cfg.SiteBaseURL != "" {
		log.Printf("public URL: %s (dated %s)", readURL, site.PublicURL(cfg.SiteBaseURL, pub.DatePath))
	}

	var pdfBytes []byte
	var pdfName string
	if cfg.AttachPDF {
		diag, err := diagrams.Process(ctx, article.Body)
		if err != nil {
			return fmt.Errorf("diagrams: %w", err)
		}
		pdfBytes, err = pdfdoc.Build(pdfdoc.Input{
			Title:    pageTitle,
			Category: topic.Category,
			Body:     diag.Markdown,
			Figures:  diag.Figures,
			Date:     now,
		})
		if err != nil {
			return fmt.Errorf("pdf: %w", err)
		}
		pdfName = fmt.Sprintf("systems-daily-%s.pdf", now.Format("2006-01-02"))
		log.Printf("pdf size: %d bytes", len(pdfBytes))
	}

	mailBody := content.EmailBody(article, readURL, cfg.AttachPDF && len(pdfBytes) > 0)

	if opt.Preview || cfg.DryRun {
		fmt.Print(mailBody)
		fmt.Printf("\nSite root: %s\n", cfg.SiteOutDir)
		fmt.Printf("Open local: %s\n", filepath.Join(cfg.SiteOutDir, "today", "index.html"))
		if cfg.AttachPDF && len(pdfBytes) > 0 {
			outPath := previewPDFPath(cfg, pdfName)
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return fmt.Errorf("preview dir: %w", err)
			}
			if err := os.WriteFile(outPath, pdfBytes, 0o644); err != nil {
				return fmt.Errorf("write preview pdf: %w", err)
			}
			fmt.Printf("PDF written to %s\n", outPath)
		}
		if opt.Preview {
			log.Printf("preview only - not sending email")
		}
		if cfg.DryRun {
			log.Printf("DRY_RUN=1 - not sending email")
		}
		if !opt.SkipHistory && !opt.Preview {
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

	if err := cfg.ValidateSMTP(); err != nil {
		return err
	}

	msg := email.Message{
		From:    cfg.SMTPFrom,
		To:      splitAddrs(cfg.SMTPTo),
		Subject: article.Subject,
		Body:    mailBody,
	}
	if cfg.AttachPDF && len(pdfBytes) > 0 {
		msg.Attachments = []email.Attachment{{
			Filename:    pdfName,
			ContentType: "application/pdf",
			Data:        pdfBytes,
		}}
	}

	err = email.Send(email.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		User:     cfg.SMTPUser,
		Pass:     cfg.SMTPPass,
		UseTLS:   cfg.SMTPUseTLS,
		Insecure: cfg.SMTPInsecure,
	}, msg)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	log.Printf("email sent to %s", strings.Join(msg.To, ", "))

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

func previewPDFPath(cfg *config.Config, pdfName string) string {
	dir := "data"
	if cfg.HistoryPath != "" {
		if d := filepath.Dir(cfg.HistoryPath); d != "" && d != "." {
			dir = d
		}
	}
	base := strings.TrimSuffix(pdfName, ".pdf")
	return filepath.Join(dir, fmt.Sprintf("%s-preview-%d.pdf", base, time.Now().Unix()))
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

func splitCLIArgs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Simple split on spaces; quote-aware parsing not required for common "-p" flags.
	return strings.Fields(s)
}

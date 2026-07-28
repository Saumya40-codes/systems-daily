package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Saumya40-codes/systems-daily/internal/app"
	"github.com/Saumya40-codes/systems-daily/internal/config"
	"github.com/Saumya40-codes/systems-daily/internal/schedule"
	"github.com/Saumya40-codes/systems-daily/internal/topics"
	"github.com/spf13/cobra"
)

func execute() error {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("systems-daily: ")

	root := &cobra.Command{
		Use:   "systems-daily",
		Short: "Daily systems deep-dives to your inbox",
		Long: `Generate a systems write-up each day, publish a minimal static site,
and email a short link notify (optional PDF attach).`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(onceCmd())
	root.AddCommand(previewCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(topicsCmd())

	return root.Execute()
}

func onceCmd() *cobra.Command {
	var topicID string
	cmd := &cobra.Command{
		Use:   "once",
		Short: "Generate site + notify email now",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return app.RunOnce(ctx, cfg, app.Options{TopicID: topicID})
		},
	}
	cmd.Flags().StringVar(&topicID, "topic", "", "force a topic id (see: systems-daily topics)")
	return cmd
}

func previewCmd() *cobra.Command {
	var topicID string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Generate site (and optional PDF); no email",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return app.RunOnce(ctx, cfg, app.Options{
				TopicID:     topicID,
				Preview:     true,
				SkipHistory: true,
			})
		},
	}
	cmd.Flags().StringVar(&topicID, "topic", "", "force a topic id")
	return cmd
}

func serveCmd() *cobra.Command {
	var now bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run forever; publish + notify daily at SEND_AT",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			loc := cfg.Location()
			log.Printf("serving daily at %s (%s) via model %s @ %s",
				cfg.SendAt, loc, cfg.LLMModel, cfg.LLMBaseURL)

			err = schedule.Daily(ctx, cfg.SendAt, loc, now, func(ctx context.Context) error {
				return app.RunOnce(ctx, cfg, app.Options{})
			})
			if err == context.Canceled {
				return nil
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&now, "now", false, "also fire once immediately, then wait for schedule")
	return cmd
}

func topicsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "topics",
		Short: "List curated topic catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			catalog, err := topics.Load(cfg.TopicsPath)
			if err != nil {
				return err
			}
			fmt.Printf("%-28s %-12s %s\n", "ID", "CATEGORY", "TITLE")
			fmt.Println("----------------------------------------------------------------------")
			for _, t := range catalog {
				fmt.Printf("%-28s %-12s %s\n", t.ID, t.Category, t.Title)
			}
			src := "embedded default"
			if cfg.TopicsPath != "" {
				src = cfg.TopicsPath
			}
			fmt.Printf("\n%d topics (%s)\n", len(catalog), src)
			return nil
		},
	}
}

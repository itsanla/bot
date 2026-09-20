package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/itsanla/bot/internal/config"
	"github.com/itsanla/bot/internal/db"
	"github.com/itsanla/bot/internal/service"
	"github.com/itsanla/bot/internal/service/activecollab"
	"github.com/itsanla/bot/internal/telegram"
)

func main() {
	log.Println("Starting notification bot daemon...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	tgClient := telegram.NewClient(cfg.TelegramBotToken, cfg.TelegramChatID)

	manager := service.NewManager(database)

	// Register ActiveCollab notification service
	if cfg.ActiveCollabToken != "" {
		acService := activecollab.NewService(
			cfg.ActiveCollabURL,
			cfg.ActiveCollabToken,
			database,
			tgClient,
			cfg.ActiveCollabPollInterval,
		)
		manager.Register(acService)
		log.Printf("Registered service: %s (interval: %v)", acService.Name(), acService.Interval())
	} else {
		log.Println("Warning: ACTIVECOLLAB_TOKEN is empty, ActiveCollab service skipped.")
	}

	// Graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start all registered services
	manager.Start(ctx)
	log.Println("All background services started. Listening for signals...")

	sig := <-sigChan
	log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)
	cancel()

	manager.Wait()
	log.Println("Daemon shutdown completed cleanly.")
}

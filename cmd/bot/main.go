package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

	// Start lightweight HTTP status & healthcheck server (port 5005)
	startTime := time.Now()
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"service": "itsanla/bot",
			"version": "v1.0.0",
		})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		lastID, _ := database.GetLastEventID("activecollab")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":           "running",
			"service":          "itsanla/bot",
			"version":          "v1.0.0",
			"uptime_seconds":   int64(time.Since(startTime).Seconds()),
			"active_services":  []string{"activecollab"},
			"ac_last_event_id": lastID,
		})
	})

	port := strings.TrimPrefix(cfg.Port, ":")
	addr := fmt.Sprintf(":%s", port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("HTTP status server listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start all registered background services
	manager.Start(ctx)
	log.Println("All background services started. Listening for signals...")

	sig := <-sigChan
	log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)
	cancel()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)

	manager.Wait()
	log.Println("Daemon shutdown completed cleanly.")
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                     string
	DBPath                   string
	TelegramBotToken         string
	TelegramChatID           int64
	ActiveCollabURL          string
	ActiveCollabToken        string
	ActiveCollabPollInterval time.Duration
	QuietHoursStart          int
	QuietHoursEnd            int
	Timezone                 string
}

func Load() (*Config, error) {
	// Try loading .env if present (ignore error if running in docker where env vars are injected directly)
	_ = godotenv.Load()

	port := getEnv("PORT", "5005")
	dbPath := getEnv("DB_PATH", "./data/bot.db")
	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tgToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	tgChatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if tgChatIDStr == "" {
		return nil, fmt.Errorf("TELEGRAM_CHAT_ID is required")
	}
	tgChatID, err := strconv.ParseInt(tgChatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid TELEGRAM_CHAT_ID: %w", err)
	}

	acURL := getEnv("ACTIVECOLLAB_URL", "https://collab.javan.co.id")
	acToken := os.Getenv("ACTIVECOLLAB_TOKEN")

	pollIntervalStr := getEnv("ACTIVECOLLAB_POLL_INTERVAL", "30s")
	pollInterval, err := time.ParseDuration(pollIntervalStr)
	if err != nil {
		pollInterval = 30 * time.Second
	}

	quietStartStr := getEnv("QUIET_HOURS_START", "23")
	quietStart, err := strconv.Atoi(quietStartStr)
	if err != nil {
		quietStart = 23
	}

	quietEndStr := getEnv("QUIET_HOURS_END", "6")
	quietEnd, err := strconv.Atoi(quietEndStr)
	if err != nil {
		quietEnd = 6
	}

	timezone := getEnv("TIMEZONE", "Asia/Jakarta")

	return &Config{
		Port:                     port,
		DBPath:                   dbPath,
		TelegramBotToken:         tgToken,
		TelegramChatID:           tgChatID,
		ActiveCollabURL:          acURL,
		ActiveCollabToken:        acToken,
		ActiveCollabPollInterval: pollInterval,
		QuietHoursStart:          quietStart,
		QuietHoursEnd:            quietEnd,
		Timezone:                 timezone,
	}, nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

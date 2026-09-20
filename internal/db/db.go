package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory %s: %w", dir, err)
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db at %s: %w", dbPath, err)
	}

	// Recommended SQLite pragmas for high reliability and concurrency
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA busy_timeout = 5000;",
	}
	for _, pragma := range pragmas {
		if _, err := conn.Exec(pragma); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("failed to execute pragma '%s': %w", pragma, err)
		}
	}

	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return d, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS service_states (
		service_name TEXT PRIMARY KEY,
		last_event_id INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS notification_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service_name TEXT NOT NULL,
		event_id INTEGER NOT NULL,
		event_type TEXT NOT NULL,
		title TEXT,
		content TEXT,
		sent_at DATETIME NOT NULL,
		UNIQUE(service_name, event_id)
	);

	CREATE TABLE IF NOT EXISTS service_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service_name TEXT NOT NULL,
		status TEXT NOT NULL,
		items_processed INTEGER NOT NULL DEFAULT 0,
		error_message TEXT,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL
	);
	`
	_, err := d.conn.Exec(query)
	return err
}

// GetLastEventID returns the last recorded event ID for a given service. Returns 0 if none exists.
func (d *DB) GetLastEventID(serviceName string) (int64, error) {
	var lastID int64
	err := d.conn.QueryRow("SELECT last_event_id FROM service_states WHERE service_name = ?", serviceName).Scan(&lastID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return lastID, nil
}

// SetLastEventID updates or inserts the last recorded event ID for a given service.
func (d *DB) SetLastEventID(serviceName string, lastID int64) error {
	query := `
	INSERT INTO service_states (service_name, last_event_id, updated_at)
	VALUES (?, ?, ?)
	ON CONFLICT(service_name) DO UPDATE SET
		last_event_id = excluded.last_event_id,
		updated_at = excluded.updated_at;
	`
	_, err := d.conn.Exec(query, serviceName, lastID, time.Now())
	return err
}

// IsNotificationSent checks whether an event has already been sent to Telegram.
func (d *DB) IsNotificationSent(serviceName string, eventID int64) (bool, error) {
	var count int
	err := d.conn.QueryRow(
		"SELECT COUNT(*) FROM notification_logs WHERE service_name = ? AND event_id = ?",
		serviceName, eventID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RecordNotification records a sent notification in the log.
func (d *DB) RecordNotification(serviceName string, eventID int64, eventType, title, content string) error {
	query := `
	INSERT OR IGNORE INTO notification_logs (service_name, event_id, event_type, title, content, sent_at)
	VALUES (?, ?, ?, ?, ?, ?);
	`
	_, err := d.conn.Exec(query, serviceName, eventID, eventType, title, content, time.Now())
	return err
}

// RecordRun logs an execution of a service run.
func (d *DB) RecordRun(serviceName, status string, itemsProcessed int, errorMsg string, durationMs int64) error {
	query := `
	INSERT INTO service_runs (service_name, status, items_processed, error_message, duration_ms, created_at)
	VALUES (?, ?, ?, ?, ?, ?);
	`
	_, err := d.conn.Exec(query, serviceName, status, itemsProcessed, errorMsg, durationMs, time.Now())
	return err
}

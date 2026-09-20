package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/itsanla/bot/internal/db"
)

type Manager struct {
	db       *db.DB
	services []Service
	wg       sync.WaitGroup
}

func NewManager(database *db.DB) *Manager {
	return &Manager{
		db:       database,
		services: make([]Service, 0),
	}
}

// Register registers a new background service.
func (m *Manager) Register(s Service) {
	m.services = append(m.services, s)
}

// Start launches all registered services concurrently in their own goroutines.
func (m *Manager) Start(ctx context.Context) {
	for _, s := range m.services {
		m.wg.Add(1)
		go m.runService(ctx, s)
	}
}

// Wait blocks until all service goroutines complete.
func (m *Manager) Wait() {
	m.wg.Wait()
}

func (m *Manager) runService(ctx context.Context, s Service) {
	defer m.wg.Done()

	log.Printf("[%s] Starting worker with interval %v", s.Name(), s.Interval())

	// Run once immediately on start
	m.execute(ctx, s)

	ticker := time.NewTicker(s.Interval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Stopping worker...", s.Name())
			return
		case <-ticker.C:
			m.execute(ctx, s)
		}
	}
}

func (m *Manager) execute(ctx context.Context, s Service) {
	start := time.Now()
	err := s.Run(ctx)
	duration := time.Since(start).Milliseconds()

	status := "success"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
		log.Printf("[%s] Run failed after %dms: %v", s.Name(), duration, err)
	} else {
		log.Printf("[%s] Run completed in %dms", s.Name(), duration)
	}

	if m.db != nil {
		_ = m.db.RecordRun(s.Name(), status, 0, errMsg, duration)
	}
}

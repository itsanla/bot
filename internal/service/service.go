package service

import (
	"context"
	"time"
)

// Service defines a modular background job that runs periodically.
type Service interface {
	// Name returns the unique identifier for this service.
	Name() string
	// Interval returns how often this service should be executed.
	Interval() time.Duration
	// Run executes one cycle of the service.
	Run(ctx context.Context) error
}

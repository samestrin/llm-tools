package testdata

import (
	"context"
	"fmt"
)

// Config holds configuration values
type Config struct {
	Name    string
	Value   int
	Enabled bool
}

// Service provides a business service
type Service struct {
	config Config
	ctx    context.Context
}

// NewService creates a new service
func NewService(ctx context.Context, cfg Config) *Service {
	return &Service{
		config: cfg,
		ctx:    ctx,
	}
}

// Run starts the service
func (s *Service) Run() error {
	if !s.config.Enabled {
		return fmt.Errorf("service disabled")
	}
	return nil
}

// Stop stops the service
func (s *Service) Stop() {
	// cleanup
}

// ServiceInterface defines service behavior
type ServiceInterface interface {
	Run() error
	Stop()
}

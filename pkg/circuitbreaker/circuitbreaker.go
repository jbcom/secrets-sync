// Package circuitbreaker provides circuit breaker pattern implementation for external API calls.
//
// This package wraps external API calls to Vault, AWS services, and other external systems
// with circuit breaker pattern to:
//   - Prevent cascade failures when downstream services are degraded
//   - Fail fast when services are unavailable
//   - Provide graceful degradation
//   - Auto-recover when services become healthy
package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/sony/gobreaker/v2"
)

// Config holds circuit breaker configuration
type Config struct {
	// Name is the circuit breaker name (for logging and metrics)
	Name string

	// MaxRequests is the maximum number of requests allowed to pass through
	// when the circuit breaker is half-open. Default: 1
	MaxRequests uint32

	// Interval is the cyclic period of the closed state for the circuit breaker
	// to clear the internal counts. If Interval is 0, the circuit breaker doesn't
	// clear internal counts during the closed state. Default: 60 seconds
	Interval time.Duration

	// Timeout is the period of the open state after which the state becomes half-open.
	// If Timeout is 0, the timeout value is set to 60 seconds. Default: 30 seconds
	Timeout time.Duration

	// ReadyToTrip is called with a copy of Counts whenever a request fails in the closed state.
	// If ReadyToTrip returns true, the circuit breaker will be placed into the open state.
	// If ReadyToTrip is nil, default ReadyToTrip is used.
	// Default ReadyToTrip returns true when the number of consecutive failures is more than 5.
	ReadyToTrip func(counts gobreaker.Counts) bool
}

// DefaultConfig returns a circuit breaker config with sensible defaults
func DefaultConfig(name string) *Config {
	return &Config{
		Name:        name,
		MaxRequests: 1,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Open circuit after 5 consecutive failures
			return counts.ConsecutiveFailures >= 5
		},
	}
}

// CircuitBreaker wraps gobreaker with observability
type CircuitBreaker struct {
	cb     *gobreaker.CircuitBreaker[any]
	config *Config
}

// New creates a new circuit breaker with the given configuration
func New(cfg *Config) *CircuitBreaker {
	if cfg == nil {
		cfg = DefaultConfig("default")
	}

	// Fill in defaults if not provided
	if cfg.MaxRequests == 0 {
		cfg.MaxRequests = 1
	}
	if cfg.Interval == 0 {
		cfg.Interval = 60 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.ReadyToTrip == nil {
		cfg.ReadyToTrip = func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		}
	}

	settings := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: cfg.ReadyToTrip,
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			l := log.WithFields(log.Fields{
				"circuitBreaker": name,
				"fromState":      from.String(),
				"toState":        to.String(),
			})

			switch to {
			case gobreaker.StateOpen:
				l.Warn("Circuit breaker opened - service degraded, failing fast")
			case gobreaker.StateHalfOpen:
				l.Info("Circuit breaker half-open - testing service recovery")
			case gobreaker.StateClosed:
				if from == gobreaker.StateHalfOpen {
					l.Info("Circuit breaker closed - service recovered")
				}
			}
		},
	}

	cb := &CircuitBreaker{
		cb:     gobreaker.NewCircuitBreaker[any](settings),
		config: cfg,
	}

	return cb
}

// Execute wraps a function call with circuit breaker pattern
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) (any, error)) (any, error) {
	// Check context before executing
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result, err := cb.cb.Execute(func() (any, error) {
		return fn(ctx)
	})

	if err != nil {
		// Log errors for observability
		l := log.WithFields(log.Fields{
			"circuitBreaker": cb.config.Name,
			"state":          cb.cb.State().String(),
		})

		switch {
		case errors.Is(err, gobreaker.ErrOpenState):
			l.Debug("Circuit breaker is open - request rejected")
		case errors.Is(err, gobreaker.ErrTooManyRequests):
			l.Debug("Circuit breaker is half-open - too many requests")
		default:
			l.WithError(err).Debug("Circuit breaker tracked failure")
		}
	}

	return result, err
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() gobreaker.State {
	return cb.cb.State()
}

// Counts returns counters of the circuit breaker
func (cb *CircuitBreaker) Counts() gobreaker.Counts {
	return cb.cb.Counts()
}

// Name returns the name of the circuit breaker
func (cb *CircuitBreaker) Name() string {
	return cb.config.Name
}

// ExecuteTyped is a type-safe wrapper around Execute
func ExecuteTyped[T any](cb *CircuitBreaker, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	result, err := cb.Execute(ctx, func(ctx context.Context) (any, error) {
		return fn(ctx)
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result.(T), nil
}

// WrapError wraps an error with circuit breaker information
// Uses errors.Is to properly detect wrapped circuit breaker errors
func WrapError(err error, cbName string, state gobreaker.State) error {
	if err == nil {
		return nil
	}

	// Use switch with errors.Is checks to satisfy staticcheck QF1003
	switch {
	case errors.Is(err, gobreaker.ErrOpenState):
		return fmt.Errorf("circuit breaker %q is open (service degraded): %w", cbName, err)
	case errors.Is(err, gobreaker.ErrTooManyRequests):
		return fmt.Errorf("circuit breaker %q rejected request (too many requests in half-open state): %w", cbName, err)
	default:
		return err
	}
}

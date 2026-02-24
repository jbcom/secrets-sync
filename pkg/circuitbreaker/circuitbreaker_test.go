package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		wantName string
	}{
		{
			name:     "nil config uses defaults",
			config:   nil,
			wantName: "default",
		},
		{
			name: "custom config",
			config: &Config{
				Name:        "test-breaker",
				MaxRequests: 2,
				Interval:    30 * time.Second,
				Timeout:     15 * time.Second,
			},
			wantName: "test-breaker",
		},
		{
			name: "partial config fills defaults",
			config: &Config{
				Name: "partial-breaker",
			},
			wantName: "partial-breaker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := New(tt.config)
			require.NotNil(t, cb)
			assert.Equal(t, tt.wantName, cb.Name())
			assert.Equal(t, gobreaker.StateClosed, cb.State())
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("test")
	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, uint32(1), cfg.MaxRequests)
	assert.Equal(t, 60*time.Second, cfg.Interval)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.NotNil(t, cfg.ReadyToTrip)
}

func TestExecute_Success(t *testing.T) {
	cb := New(DefaultConfig("test-success"))
	ctx := context.Background()

	result, err := cb.Execute(ctx, func(ctx context.Context) (any, error) {
		return "success", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, gobreaker.StateClosed, cb.State())
}

func TestExecute_Failure(t *testing.T) {
	cb := New(DefaultConfig("test-failure"))
	ctx := context.Background()
	testErr := errors.New("test error")

	result, err := cb.Execute(ctx, func(ctx context.Context) (any, error) {
		return nil, testErr
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, testErr, err)
	assert.Equal(t, gobreaker.StateClosed, cb.State())
}

func TestExecute_CircuitOpens(t *testing.T) {
	cfg := &Config{
		Name:        "test-open",
		MaxRequests: 1,
		Interval:    10 * time.Millisecond,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Open after 3 consecutive failures
			return counts.ConsecutiveFailures >= 3
		},
	}
	cb := New(cfg)
	ctx := context.Background()
	testErr := errors.New("test error")

	// First 3 failures should open the circuit
	for i := 0; i < 3; i++ {
		_, err := cb.Execute(ctx, func(ctx context.Context) (any, error) {
			return nil, testErr
		})
		require.Error(t, err)
		assert.Equal(t, testErr, err)
	}

	// Circuit should be open now
	assert.Equal(t, gobreaker.StateOpen, cb.State())

	// Next call should fail fast with ErrOpenState
	_, err := cb.Execute(ctx, func(ctx context.Context) (any, error) {
		return "should not be called", nil
	})
	require.Error(t, err)
	assert.Equal(t, gobreaker.ErrOpenState, err)
}

func TestExecute_CircuitRecovers(t *testing.T) {
	cfg := &Config{
		Name:        "test-recovery",
		MaxRequests: 1,
		Interval:    10 * time.Millisecond,
		Timeout:     50 * time.Millisecond, // Short timeout for test
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}
	cb := New(cfg)
	ctx := context.Background()
	testErr := errors.New("test error")

	// Trigger circuit to open
	for i := 0; i < 2; i++ {
		_, err := cb.Execute(ctx, func(ctx context.Context) (any, error) {
			return nil, testErr
		})
		require.Error(t, err)
	}
	assert.Equal(t, gobreaker.StateOpen, cb.State())

	// Wait for timeout to enter half-open state
	time.Sleep(60 * time.Millisecond)

	// Next successful call should close the circuit
	result, err := cb.Execute(ctx, func(ctx context.Context) (any, error) {
		return "recovered", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "recovered", result)
	assert.Equal(t, gobreaker.StateClosed, cb.State())
}

func TestExecute_ContextCancellation(t *testing.T) {
	cb := New(DefaultConfig("test-context"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := cb.Execute(ctx, func(ctx context.Context) (any, error) {
		return "should not be called", nil
	})

	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Nil(t, result)
}

func TestExecuteTyped(t *testing.T) {
	cb := New(DefaultConfig("test-typed"))
	ctx := context.Background()

	// Test successful typed execution
	result, err := ExecuteTyped(cb, ctx, func(ctx context.Context) (string, error) {
		return "typed result", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "typed result", result)

	// Test failed typed execution
	testErr := errors.New("typed error")
	result, err = ExecuteTyped(cb, ctx, func(ctx context.Context) (string, error) {
		return "", testErr
	})
	require.Error(t, err)
	assert.Equal(t, testErr, err)
	assert.Empty(t, result)
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		cbName    string
		state     gobreaker.State
		wantError bool
		wantMsg   string
	}{
		{
			name:      "nil error",
			err:       nil,
			cbName:    "test",
			state:     gobreaker.StateClosed,
			wantError: false,
		},
		{
			name:      "open state error",
			err:       gobreaker.ErrOpenState,
			cbName:    "test",
			state:     gobreaker.StateOpen,
			wantError: true,
			wantMsg:   "circuit breaker \"test\" is open (service degraded)",
		},
		{
			name:      "too many requests error",
			err:       gobreaker.ErrTooManyRequests,
			cbName:    "test",
			state:     gobreaker.StateHalfOpen,
			wantError: true,
			wantMsg:   "circuit breaker \"test\" rejected request (too many requests in half-open state)",
		},
		{
			name:      "other error",
			err:       errors.New("some error"),
			cbName:    "test",
			state:     gobreaker.StateClosed,
			wantError: true,
			wantMsg:   "some error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WrapError(tt.err, tt.cbName, tt.state)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCounts(t *testing.T) {
	cb := New(DefaultConfig("test-counts"))
	ctx := context.Background()

	// Initial counts should be zero
	counts := cb.Counts()
	assert.Equal(t, uint32(0), counts.Requests)
	assert.Equal(t, uint32(0), counts.TotalSuccesses)
	assert.Equal(t, uint32(0), counts.TotalFailures)

	// Execute some successful requests
	for i := 0; i < 3; i++ {
		_, err := cb.Execute(ctx, func(ctx context.Context) (any, error) {
			return "success", nil
		})
		require.NoError(t, err)
	}

	counts = cb.Counts()
	assert.Equal(t, uint32(3), counts.TotalSuccesses)

	// Execute some failed requests
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		_, err := cb.Execute(ctx, func(ctx context.Context) (any, error) {
			return nil, testErr
		})
		require.Error(t, err)
	}

	counts = cb.Counts()
	assert.Equal(t, uint32(3), counts.TotalSuccesses)
	assert.Equal(t, uint32(2), counts.TotalFailures)
	assert.Equal(t, uint32(2), counts.ConsecutiveFailures)
}

package context

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestErrorBuilder(t *testing.T) {
	reqCtx := NewRequestContext()
	ctx := WithRequestContext(context.Background(), reqCtx)

	builder := NewErrorBuilder(ctx, "vault.list")
	if builder == nil {
		t.Fatal("NewErrorBuilder returned nil")
	}

	if builder.operation != "vault.list" {
		t.Errorf("operation mismatch: got %s, want vault.list", builder.operation)
	}
}

func TestErrorBuilder_WithPath(t *testing.T) {
	ctx := WithRequestContext(context.Background(), NewRequestContext())
	builder := NewErrorBuilder(ctx, "vault.list").WithPath("secret/data/myapp")

	err := builder.Build("failed to list secrets", nil)

	errMsg := err.Error()
	if !strings.Contains(errMsg, "path=\"secret/data/myapp\"") {
		t.Errorf("error message missing path: %s", errMsg)
	}
}

func TestErrorBuilder_WithSecretName(t *testing.T) {
	ctx := WithRequestContext(context.Background(), NewRequestContext())
	builder := NewErrorBuilder(ctx, "aws.create_secret").WithSecretName("my-secret")

	err := builder.Build("failed to create secret", nil)

	errMsg := err.Error()
	if !strings.Contains(errMsg, "secret=\"my-secret\"") {
		t.Errorf("error message missing secret name: %s", errMsg)
	}
}

func TestErrorBuilder_WithRetryCount(t *testing.T) {
	ctx := WithRequestContext(context.Background(), NewRequestContext())
	builder := NewErrorBuilder(ctx, "vault.read").WithRetryCount(3)

	err := builder.Build("failed after retries", nil)

	errMsg := err.Error()
	if !strings.Contains(errMsg, "retries=3") {
		t.Errorf("error message missing retry count: %s", errMsg)
	}
}

func TestErrorBuilder_Build(t *testing.T) {
	reqCtx := NewRequestContext()
	ctx := WithRequestContext(context.Background(), reqCtx)

	builder := NewErrorBuilder(ctx, "vault.list").
		WithPath("secret/data/myapp").
		WithRetryCount(2)

	// Sleep to get measurable duration
	time.Sleep(10 * time.Millisecond)

	baseErr := errors.New("connection refused")
	err := builder.Build("failed to list secrets", baseErr)

	errMsg := err.Error()

	// Check all components
	if !strings.Contains(errMsg, reqCtx.RequestID) {
		t.Errorf("error message missing request ID: %s", errMsg)
	}
	if !strings.Contains(errMsg, "operation=vault.list") {
		t.Errorf("error message missing operation: %s", errMsg)
	}
	if !strings.Contains(errMsg, "path=\"secret/data/myapp\"") {
		t.Errorf("error message missing path: %s", errMsg)
	}
	if !strings.Contains(errMsg, "retries=2") {
		t.Errorf("error message missing retry count: %s", errMsg)
	}
	if !strings.Contains(errMsg, "duration=") {
		t.Errorf("error message missing duration: %s", errMsg)
	}
	if !strings.Contains(errMsg, "failed to list secrets") {
		t.Errorf("error message missing custom message: %s", errMsg)
	}
	if !strings.Contains(errMsg, "connection refused") {
		t.Errorf("error message missing wrapped error: %s", errMsg)
	}
}

func TestErrorBuilder_Errorf(t *testing.T) {
	ctx := WithRequestContext(context.Background(), NewRequestContext())
	builder := NewErrorBuilder(ctx, "vault.list")

	err := builder.Errorf("failed to list %d secrets", 42)

	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to list 42 secrets") {
		t.Errorf("error message missing formatted message: %s", errMsg)
	}
}

func TestErrorBuilder_Wrap(t *testing.T) {
	ctx := WithRequestContext(context.Background(), NewRequestContext())
	builder := NewErrorBuilder(ctx, "vault.list")

	baseErr := errors.New("connection timeout")
	err := builder.Wrap(baseErr, "failed to connect")

	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to connect") {
		t.Errorf("error message missing message: %s", errMsg)
	}
	if !strings.Contains(errMsg, "connection timeout") {
		t.Errorf("error message missing wrapped error: %s", errMsg)
	}

	// Verify error unwrapping works
	if !errors.Is(err, baseErr) {
		t.Error("wrapped error should be unwrappable")
	}
}

func TestErrorBuilder_GetContext(t *testing.T) {
	reqCtx := NewRequestContext()
	ctx := WithRequestContext(context.Background(), reqCtx)

	builder := NewErrorBuilder(ctx, "vault.list").
		WithPath("secret/data/myapp").
		WithSecretName("my-secret").
		WithRetryCount(3)

	time.Sleep(10 * time.Millisecond)

	errCtx := builder.GetContext()

	if errCtx.RequestID != reqCtx.RequestID {
		t.Errorf("RequestID mismatch: got %s, want %s", errCtx.RequestID, reqCtx.RequestID)
	}
	if errCtx.Operation != "vault.list" {
		t.Errorf("Operation mismatch: got %s, want vault.list", errCtx.Operation)
	}
	if errCtx.Path != "secret/data/myapp" {
		t.Errorf("Path mismatch: got %s, want secret/data/myapp", errCtx.Path)
	}
	if errCtx.SecretName != "my-secret" {
		t.Errorf("SecretName mismatch: got %s, want my-secret", errCtx.SecretName)
	}
	if errCtx.RetryCount != 3 {
		t.Errorf("RetryCount mismatch: got %d, want 3", errCtx.RetryCount)
	}
	if errCtx.DurationMs < 10 {
		t.Errorf("DurationMs too small: got %d", errCtx.DurationMs)
	}
}

func TestErrorBuilder_NoRequestContext(t *testing.T) {
	// Test with plain context (no request context)
	ctx := context.Background()
	builder := NewErrorBuilder(ctx, "vault.list")

	err := builder.Build("test error", nil)

	errMsg := err.Error()
	// Should still work, just without request ID
	if !strings.Contains(errMsg, "operation=vault.list") {
		t.Errorf("error message missing operation: %s", errMsg)
	}
	if !strings.Contains(errMsg, "duration=") {
		t.Errorf("error message missing duration: %s", errMsg)
	}
}

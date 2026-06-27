package context

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RequestContextKey is the key for storing RequestContext in context.Context
type contextKey string

const requestContextKey contextKey = "secrets_sync_request_context"

// RequestContext contains metadata for tracking operations across the pipeline
type RequestContext struct {
	RequestID string
	StartTime time.Time
}

// NewRequestContext creates a new request context with a unique request ID
func NewRequestContext() *RequestContext {
	return &RequestContext{
		RequestID: uuid.New().String(),
		StartTime: time.Now(),
	}
}

// WithRequestContext adds request context to a context.Context
func WithRequestContext(ctx context.Context, reqCtx *RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey, reqCtx)
}

// FromContext extracts the request context from context.Context
// Returns nil if no request context is found
func FromContext(ctx context.Context) *RequestContext {
	reqCtx, ok := ctx.Value(requestContextKey).(*RequestContext)
	if !ok || reqCtx == nil {
		return nil
	}
	return reqCtx
}

// GetRequestID safely extracts the request ID from context
// Returns empty string if no request context is found
func GetRequestID(ctx context.Context) string {
	reqCtx := FromContext(ctx)
	if reqCtx == nil {
		return ""
	}
	return reqCtx.RequestID
}

// SafeRequestID returns the request ID with any control characters (CR, LF,
// other non-printable bytes) stripped, so it is safe to embed in a log line.
// Request IDs are normally generated UUIDs, but a caller embedding the pipeline
// (e.g. the serverless entrypoints) could supply an arbitrary one, so it is
// sanitized at the logging boundary to prevent log injection.
func SafeRequestID(ctx context.Context) string {
	return sanitizeForLog(GetRequestID(ctx))
}

// sanitizeForLog removes control characters from a string destined for a log.
func sanitizeForLog(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// GetElapsedTime calculates elapsed time from request start
// Returns 0 if no request context is found
func GetElapsedTime(ctx context.Context) time.Duration {
	reqCtx := FromContext(ctx)
	if reqCtx == nil {
		return 0
	}
	return time.Since(reqCtx.StartTime)
}

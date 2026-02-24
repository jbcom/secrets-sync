# Enhanced Error Context

This document describes the enhanced error context feature in SecretSync, which adds request IDs, timing information, and structured metadata to error messages for easier debugging.

## Overview

All operations in SecretSync now include:
- **Request ID**: Unique identifier for correlating errors across services
- **Operation name**: The specific operation being performed (e.g., "vault.list", "aws.create_secret")
- **Path/Secret name**: The resource being accessed
- **Duration**: Time elapsed from operation start
- **Retry count**: Number of retry attempts (when applicable)

## Request Context

The request context is automatically generated at the pipeline start and propagated through all operations.

### Example

```go
import (
    "context"
    reqctx "github.com/extended-data-library/secretssync/pkg/context"
)

// Generate request context
rctx := reqctx.NewRequestContext()
ctx := reqctx.WithRequestContext(context.Background(), rctx)

// Request ID is now available in logs
requestID := reqctx.GetRequestID(ctx)
```

## Error Messages

Error messages now include structured context:

### Before
```
failed to list secrets: connection refused
```

### After
```
[req=e0958539-fae2-4567-9227-592a5b36983a] operation=vault.list path="secret/data/myapp" duration=150ms: failed to list secrets: connection refused
```

## Error Builder

The `ErrorBuilder` provides a fluent API for constructing errors with context:

```go
import reqctx "github.com/extended-data-library/secretssync/pkg/context"

// Create error builder
errBuilder := reqctx.NewErrorBuilder(ctx, "vault.write").
    WithPath("secret/data/myapp").
    WithRetryCount(3)

// Build error with message
err := errBuilder.Build("failed to write secret", baseErr)

// Or use formatted message
err := errBuilder.Errorf("failed to write %d secrets", count)

// Or wrap existing error
err := errBuilder.Wrap(baseErr, "operation failed")
```

## Structured Logging

All operations now include request IDs in log entries:

```go
log.WithFields(log.Fields{
    "action":     "syncTarget",
    "target":     targetName,
    "request_id": reqctx.GetRequestID(ctx),
}).Info("Starting sync")
```

## Pipeline Integration

The pipeline automatically:
1. Generates a unique request ID at the start of each Run
2. Propagates it through all merge and sync operations
3. Includes it in start/end log messages with duration

Example log output:
```
time="2025-12-09T17:00:00Z" level=info msg="Starting pipeline execution" action=Pipeline.Run operation=pipeline request_id=e0958539-fae2-4567-9227-592a5b36983a
time="2025-12-09T17:00:05Z" level=info msg="Starting merge phase" action=Pipeline.runMerge request_id=e0958539-fae2-4567-9227-592a5b36983a
time="2025-12-09T17:00:10Z" level=info msg="Pipeline execution completed successfully" request_id=e0958539-fae2-4567-9227-592a5b36983a duration_ms=10234
```

## Backward Compatibility

The enhanced error context is fully backward compatible:
- Operations without request context still work (request_id will be empty)
- Error messages maintain the same error wrapping chain
- Existing error handling code continues to work unchanged

## Benefits

1. **Debugging**: Request IDs allow correlating errors across distributed systems
2. **Performance**: Duration information helps identify slow operations
3. **Observability**: Structured fields enable better log aggregation and analysis
4. **Troubleshooting**: Context-rich errors reduce time to diagnosis
5. **Monitoring**: Easier to track error patterns and retry behavior

## Related

- Issue #47: Enhanced Error Messages
- PR #29 code review feedback
- Issue #46: Observability

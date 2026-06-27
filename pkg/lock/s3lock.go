// Package lock provides distributed coordination for multi-replica deployments:
// an S3-backed mutual-exclusion lock (via conditional writes) and a leader-
// election loop built on top of it, plus deterministic work partitioning.
package lock

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// ErrLockHeld is returned when the lock is already held by another holder.
var ErrLockHeld = errors.New("lock: already held")

// s3LockAPI is the subset of the S3 client the lock uses, abstracted for tests.
type s3LockAPI interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// S3Lock is a best-effort distributed lock using S3 conditional writes
// (If-None-Match: *), which atomically create the lock object only when it does
// not already exist. Acquire writes the holder identity; Release deletes it.
type S3Lock struct {
	api    s3LockAPI
	bucket string
	key    string
	holder string
}

// NewS3Lock builds a lock at s3://bucket/key identified by holder (e.g. a pod
// name).
func NewS3Lock(api s3LockAPI, bucket, key, holder string) *S3Lock {
	return &S3Lock{api: api, bucket: bucket, key: key, holder: holder}
}

// Acquire attempts to take the lock. It returns ErrLockHeld when another holder
// already owns it, nil on success, or a wrapped error on transport failure.
func (l *S3Lock) Acquire(ctx context.Context) error {
	body := fmt.Sprintf("%s\n%s", l.holder, time.Now().UTC().Format(time.RFC3339Nano))
	_, err := l.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(l.bucket),
		Key:         aws.String(l.key),
		Body:        bytes.NewReader([]byte(body)),
		IfNoneMatch: aws.String("*"),
	})
	if err != nil {
		if isPreconditionFailed(err) {
			return ErrLockHeld
		}
		return fmt.Errorf("lock: acquire: %w", err)
	}
	return nil
}

// Release deletes the lock object. It is idempotent: deleting a missing lock is
// not an error.
func (l *S3Lock) Release(ctx context.Context) error {
	_, err := l.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(l.bucket),
		Key:    aws.String(l.key),
	})
	if err != nil {
		return fmt.Errorf("lock: release: %w", err)
	}
	return nil
}

// isPreconditionFailed reports whether the error is S3's response to a failed
// If-None-Match condition (the object already exists), i.e. PreconditionFailed
// or 412.
func isPreconditionFailed(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		return code == "PreconditionFailed" || code == "412"
	}
	return false
}

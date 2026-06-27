// Package lock provides distributed coordination for multi-replica deployments:
// an S3-backed mutual-exclusion lock (via conditional writes, with lease expiry)
// and a leader-election loop built on top of it, plus deterministic work
// partitioning.
package lock

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// ErrLockHeld is returned when the lock is already held (and unexpired) by
// another holder.
var ErrLockHeld = errors.New("lock: already held")

// s3LockAPI is the subset of the S3 client the lock uses, abstracted for tests.
type s3LockAPI interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// leaseRecord is the lock object body: who holds it and when the lease expires.
type leaseRecord struct {
	Holder    string    `json:"holder"`
	ExpiresAt time.Time `json:"expires_at"`
}

// S3Lock is a distributed lease lock using S3 conditional writes
// (If-None-Match: *), which atomically create the lock object only when it does
// not already exist. The lock carries a TTL so that a holder which crashes
// without releasing is reclaimable: once the lease expires, another replica may
// take it over. Hold the lease alive with Refresh (or use RunAsLeader, which
// heartbeats automatically).
type S3Lock struct {
	api    s3LockAPI
	bucket string
	key    string
	holder string
	ttl    time.Duration
	now    func() time.Time
}

// NewS3Lock builds a lease lock at s3://bucket/key identified by holder (e.g. a
// pod name) with the given lease TTL (defaulting to 60s).
func NewS3Lock(api s3LockAPI, bucket, key, holder string, ttl time.Duration) *S3Lock {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &S3Lock{api: api, bucket: bucket, key: key, holder: holder, ttl: ttl, now: time.Now}
}

// Acquire attempts to take the lock. It first tries an atomic conditional
// create. If that fails because the object exists, it reads the existing lease
// and, if expired, takes it over with an unconditional overwrite. It returns
// ErrLockHeld when a live lease is held by someone else.
func (l *S3Lock) Acquire(ctx context.Context) error {
	if err := l.conditionalPut(ctx); err == nil {
		return nil
	} else if !isLockContended(err) {
		return fmt.Errorf("lock: acquire: %w", err)
	}

	// The object exists; inspect the lease for staleness.
	rec, err := l.readLease(ctx)
	if err != nil {
		// Couldn't read the lease (e.g. it was just deleted) — treat as contended
		// so the caller retries rather than assuming ownership.
		return ErrLockHeld
	}
	if l.now().Before(rec.ExpiresAt) {
		return ErrLockHeld
	}
	// Lease is expired; take it over with an unconditional overwrite.
	if err := l.put(ctx, false); err != nil {
		return fmt.Errorf("lock: takeover: %w", err)
	}
	return nil
}

// Refresh extends the lease. It overwrites the lock object unconditionally with
// a new expiry; the caller must already hold the lock.
func (l *S3Lock) Refresh(ctx context.Context) error {
	if err := l.put(ctx, false); err != nil {
		return fmt.Errorf("lock: refresh: %w", err)
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

func (l *S3Lock) conditionalPut(ctx context.Context) error { return l.put(ctx, true) }

// put writes the lease record. When conditional is true it uses If-None-Match:*
// (atomic create-if-absent); otherwise it overwrites unconditionally.
func (l *S3Lock) put(ctx context.Context, conditional bool) error {
	body, _ := json.Marshal(leaseRecord{Holder: l.holder, ExpiresAt: l.now().Add(l.ttl)})
	in := &s3.PutObjectInput{
		Bucket: aws.String(l.bucket),
		Key:    aws.String(l.key),
		Body:   bytes.NewReader(body),
	}
	if conditional {
		in.IfNoneMatch = aws.String("*")
	}
	_, err := l.api.PutObject(ctx, in)
	return err
}

func (l *S3Lock) readLease(ctx context.Context) (leaseRecord, error) {
	out, err := l.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(l.bucket),
		Key:    aws.String(l.key),
	})
	if err != nil {
		return leaseRecord{}, err
	}
	defer func() { _ = out.Body.Close() }()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return leaseRecord{}, err
	}
	var rec leaseRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return leaseRecord{}, err
	}
	return rec, nil
}

// isLockContended reports whether a PutObject error is S3's response to a failed
// If-None-Match condition — the object already exists. S3 returns
// PreconditionFailed (HTTP 412) for the classic conditional-header path and
// ConditionalRequestConflict when two conditional writes race on the same key;
// both mean "someone else got there first".
func isLockContended(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "PreconditionFailed", "ConditionalRequestConflict":
			return true
		}
	}
	return false
}

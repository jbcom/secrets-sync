package audit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// FileSink appends each entry as a JSON line to a file. It is the simplest
// tamper-evident destination: the hash chain lets a verifier detect edits even
// though the file itself is plain.
type FileSink struct {
	mu sync.Mutex
	f  *os.File
	w  *bufio.Writer
}

// NewFileSink opens (creating, append) the given path for audit output.
func NewFileSink(path string) (*FileSink, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit: open file sink: %w", err)
	}
	return &FileSink{f: f, w: bufio.NewWriter(f)}, nil
}

// Write appends one JSON-encoded entry followed by a newline and flushes, so an
// audit record is durable as soon as Log returns.
func (s *FileSink) Write(e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := s.w.Write(append(b, '\n')); err != nil {
		return err
	}
	return s.w.Flush()
}

// Close flushes and closes the file.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.w.Flush(); err != nil {
		return err
	}
	return s.f.Close()
}

// s3PutAPI is the subset of the S3 client the sink uses, abstracted for tests.
type s3PutAPI interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3Sink writes each entry as an individual immutable object under a prefix,
// keyed by zero-padded sequence so listing preserves chain order. Writing one
// object per entry keeps records append-only and individually retained.
type S3Sink struct {
	api    s3PutAPI
	bucket string
	prefix string
}

// NewS3Sink builds an S3 audit sink for the given bucket/prefix using the
// provided client.
func NewS3Sink(api s3PutAPI, bucket, prefix string) *S3Sink {
	return &S3Sink{api: api, bucket: bucket, prefix: prefix}
}

// Write puts the entry as a JSON object at <prefix>/<sequence>.json.
func (s *S3Sink) Write(e Entry) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s/%020d.json", s.prefix, e.Sequence)
	_, err = s.api.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(b),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("audit: s3 put %s: %w", key, err)
	}
	return nil
}

// Close is a no-op; the S3 client is owned by the caller.
func (s *S3Sink) Close() error { return nil }

// cwAPI is the subset of the CloudWatch Logs client the sink uses.
type cwAPI interface {
	PutLogEvents(ctx context.Context, in *cloudwatchlogs.PutLogEventsInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
}

// CloudWatchSink writes each entry as a log event to a CloudWatch Logs stream.
// Modern PutLogEvents no longer requires sequence tokens, so each entry is a
// single self-contained put.
type CloudWatchSink struct {
	api       cwAPI
	group     string
	stream    string
	nowMillis func() int64
}

// NewCloudWatchSink builds a CloudWatch Logs audit sink for the given log group
// and stream.
func NewCloudWatchSink(api cwAPI, group, stream string) *CloudWatchSink {
	return &CloudWatchSink{
		api: api, group: group, stream: stream,
		nowMillis: func() int64 { return time.Now().UnixMilli() },
	}
}

// Write emits the entry as a JSON log event.
func (s *CloudWatchSink) Write(e Entry) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = s.api.PutLogEvents(context.Background(), &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(s.group),
		LogStreamName: aws.String(s.stream),
		LogEvents: []cwtypes.InputLogEvent{
			{Message: aws.String(string(b)), Timestamp: aws.Int64(s.nowMillis())},
		},
	})
	if err != nil {
		return fmt.Errorf("audit: cloudwatch put: %w", err)
	}
	return nil
}

// Close is a no-op; the CloudWatch client is owned by the caller.
func (s *CloudWatchSink) Close() error { return nil }

// MultiSink fans each entry out to several sinks. A write fails if any
// underlying sink fails, so audit coverage is all-or-nothing.
type MultiSink struct {
	sinks []Sink
}

// NewMultiSink combines sinks.
func NewMultiSink(sinks ...Sink) *MultiSink { return &MultiSink{sinks: sinks} }

func (m *MultiSink) Write(e Entry) error {
	for _, s := range m.sinks {
		if err := s.Write(e); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiSink) Close() error {
	var firstErr error
	for _, s := range m.sinks {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

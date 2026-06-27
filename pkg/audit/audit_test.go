package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// memSink captures entries in memory.
type memSink struct {
	entries []Entry
	failOn  int // fail Write when len(entries)==failOn (-1 = never)
}

func (m *memSink) Write(e Entry) error {
	if m.failOn >= 0 && len(m.entries) == m.failOn {
		return fmt.Errorf("injected failure")
	}
	m.entries = append(m.entries, e)
	return nil
}
func (m *memSink) Close() error { return nil }

func TestChainIntegrity(t *testing.T) {
	sink := &memSink{failOn: -1}
	l := NewLogger(sink)
	for i := 0; i < 5; i++ {
		if err := l.Log(Record{Operation: OpWrite, Driver: "aws", Secret: fmt.Sprintf("s%d", i), Success: true}); err != nil {
			t.Fatalf("log %d: %v", i, err)
		}
	}
	if len(sink.entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(sink.entries))
	}
	if idx, err := Verify(sink.entries); idx != -1 || err != nil {
		t.Fatalf("valid chain should verify: idx=%d err=%v", idx, err)
	}
	// Sequences are monotonic starting at 1.
	for i, e := range sink.entries {
		if e.Sequence != uint64(i+1) {
			t.Fatalf("entry %d has sequence %d", i, e.Sequence)
		}
	}
}

func TestTamperDetection(t *testing.T) {
	sink := &memSink{failOn: -1}
	l := NewLogger(sink)
	for i := 0; i < 3; i++ {
		_ = l.Log(Record{Operation: OpRead, Driver: "vault", Secret: fmt.Sprintf("s%d", i), Success: true})
	}

	// Tamper with a middle entry's content without recomputing the chain.
	tampered := append([]Entry(nil), sink.entries...)
	tampered[1].Secret = "evil"
	if idx, err := Verify(tampered); idx != 1 || err == nil {
		t.Fatalf("tampered content should be detected at index 1: idx=%d err=%v", idx, err)
	}

	// Deleting an entry breaks the prev_hash linkage.
	deleted := []Entry{sink.entries[0], sink.entries[2]}
	if idx, err := Verify(deleted); idx != 1 || err == nil {
		t.Fatalf("deleted entry should break chain at index 1: idx=%d err=%v", idx, err)
	}
}

func TestWriteFailureRollsBackSequence(t *testing.T) {
	sink := &memSink{failOn: 1} // fail the 2nd write
	l := NewLogger(sink)
	if err := l.Log(Record{Operation: OpWrite, Secret: "a", Success: true}); err != nil {
		t.Fatalf("first log: %v", err)
	}
	if err := l.Log(Record{Operation: OpWrite, Secret: "b", Success: true}); err == nil {
		t.Fatal("expected second log to fail")
	}
	// After the failure, the next successful log must continue the chain from
	// the last good entry with sequence 2 (not 3).
	sink.failOn = -1
	if err := l.Log(Record{Operation: OpWrite, Secret: "c", Success: true}); err != nil {
		t.Fatalf("third log: %v", err)
	}
	if idx, err := Verify(sink.entries); idx != -1 || err != nil {
		t.Fatalf("chain should remain valid after rollback: idx=%d err=%v", idx, err)
	}
	if sink.entries[len(sink.entries)-1].Sequence != 2 {
		t.Fatalf("sequence should be 2 after rollback, got %d", sink.entries[len(sink.entries)-1].Sequence)
	}
}

func TestFileSinkRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("new file sink: %v", err)
	}
	l := NewLogger(sink)
	for i := 0; i < 3; i++ {
		_ = l.Log(Record{Operation: OpDelete, Driver: "gcp", Secret: fmt.Sprintf("s%d", i), Success: true})
	}
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	f, _ := os.Open(path)
	defer f.Close()
	var entries []Entry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("decode line: %v", err)
		}
		entries = append(entries, e)
	}
	if idx, err := Verify(entries); idx != -1 || err != nil {
		t.Fatalf("file chain should verify: idx=%d err=%v", idx, err)
	}
}

// fakeS3 records PutObject calls.
type fakeS3 struct{ keys []string }

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.keys = append(f.keys, *in.Key)
	return &s3.PutObjectOutput{}, nil
}

func TestS3SinkKeysAreOrdered(t *testing.T) {
	fake := &fakeS3{}
	l := NewLogger(NewS3Sink(fake, "bucket", "audit"))
	for i := 0; i < 3; i++ {
		_ = l.Log(Record{Operation: OpWrite, Secret: fmt.Sprintf("s%d", i), Success: true})
	}
	if len(fake.keys) != 3 {
		t.Fatalf("expected 3 puts, got %d", len(fake.keys))
	}
	if fake.keys[0] != "audit/00000000000000000001.json" {
		t.Fatalf("unexpected key: %s", fake.keys[0])
	}
}

// fakeCW records PutLogEvents calls.
type fakeCW struct{ count int }

func (f *fakeCW) PutLogEvents(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	f.count++
	return &cloudwatchlogs.PutLogEventsOutput{}, nil
}

func TestCloudWatchSink(t *testing.T) {
	fake := &fakeCW{}
	sink := NewCloudWatchSink(fake, "grp", "stream")
	sink.nowMillis = func() int64 { return 1000 }
	l := NewLogger(sink)
	_ = l.Log(Record{Operation: OpRead, Secret: "x", Success: true})
	if fake.count != 1 {
		t.Fatalf("expected 1 put log event, got %d", fake.count)
	}
}

func TestMultiSink(t *testing.T) {
	a, b := &memSink{failOn: -1}, &memSink{failOn: -1}
	l := NewLogger(NewMultiSink(a, b))
	_ = l.Log(Record{Operation: OpWrite, Secret: "x", Success: true})
	if len(a.entries) != 1 || len(b.entries) != 1 {
		t.Fatalf("both sinks should receive the entry: a=%d b=%d", len(a.entries), len(b.entries))
	}
}

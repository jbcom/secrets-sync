// Package audit provides tamper-evident structured audit logging for every
// secret read, write, and delete. Entries are linked into a hash chain so any
// retroactive modification or deletion of a record is detectable.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Operation is the audited action.
type Operation string

const (
	OpRead   Operation = "read"
	OpWrite  Operation = "write"
	OpDelete Operation = "delete"
)

// Entry is a single audit record. PrevHash links to the previous entry and Hash
// is this entry's digest over its content plus PrevHash, forming a tamper-
// evident chain. Secret values are never recorded — only identifiers.
type Entry struct {
	Sequence  uint64    `json:"sequence"`
	Timestamp string    `json:"timestamp"`
	Operation Operation `json:"operation"`
	Driver    string    `json:"driver"`
	Target    string    `json:"target,omitempty"`
	Source    string    `json:"source,omitempty"`
	Secret    string    `json:"secret"`
	Actor     string    `json:"actor,omitempty"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	PrevHash  string    `json:"prev_hash"`
	Hash      string    `json:"hash"`
}

// Sink persists audit entries. Implementations write to a file, S3, CloudWatch,
// etc. Write must be safe for the single-goroutine calls the Logger makes under
// its lock.
type Sink interface {
	Write(e Entry) error
	Close() error
}

// Record is the caller-supplied content of an audit event, before sequencing
// and hashing.
type Record struct {
	Operation Operation
	Driver    string
	Target    string
	Source    string
	Secret    string
	Actor     string
	Success   bool
	Error     string
}

// Logger appends hash-chained entries to a Sink. It is safe for concurrent use;
// entries are sequenced and chained under a mutex so the chain is well-ordered.
type Logger struct {
	mu       sync.Mutex
	sink     Sink
	seq      uint64
	lastHash string
	now      func() time.Time
}

// NewLogger returns a Logger writing to sink. The genesis previous-hash is the
// empty-string sentinel "0".
func NewLogger(sink Sink) *Logger {
	return &Logger{sink: sink, lastHash: genesisHash, now: time.Now}
}

const genesisHash = "0"

// computeHash returns the hex sha256 over the entry's content and its PrevHash.
// It is deterministic and excludes the Hash field itself.
func computeHash(e Entry) string {
	// Serialize the content fields in a fixed order via a struct without Hash.
	payload := struct {
		Sequence  uint64    `json:"sequence"`
		Timestamp string    `json:"timestamp"`
		Operation Operation `json:"operation"`
		Driver    string    `json:"driver"`
		Target    string    `json:"target"`
		Source    string    `json:"source"`
		Secret    string    `json:"secret"`
		Actor     string    `json:"actor"`
		Success   bool      `json:"success"`
		Error     string    `json:"error"`
		PrevHash  string    `json:"prev_hash"`
	}{
		Sequence: e.Sequence, Timestamp: e.Timestamp, Operation: e.Operation,
		Driver: e.Driver, Target: e.Target, Source: e.Source, Secret: e.Secret,
		Actor: e.Actor, Success: e.Success, Error: e.Error, PrevHash: e.PrevHash,
	}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Log appends a record to the chain and persists it.
func (l *Logger) Log(r Record) error {
	if l == nil || l.sink == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	l.seq++
	e := Entry{
		Sequence:  l.seq,
		Timestamp: l.now().UTC().Format(time.RFC3339Nano),
		Operation: r.Operation,
		Driver:    r.Driver,
		Target:    r.Target,
		Source:    r.Source,
		Secret:    r.Secret,
		Actor:     r.Actor,
		Success:   r.Success,
		Error:     r.Error,
		PrevHash:  l.lastHash,
	}
	e.Hash = computeHash(e)
	if err := l.sink.Write(e); err != nil {
		// Roll back sequence/hash so a failed write doesn't break the chain.
		l.seq--
		return fmt.Errorf("audit: write entry: %w", err)
	}
	l.lastHash = e.Hash
	return nil
}

// Close flushes and closes the underlying sink.
func (l *Logger) Close() error {
	if l == nil || l.sink == nil {
		return nil
	}
	return l.sink.Close()
}

// Verify checks that a sequence of entries forms an unbroken hash chain: each
// entry's Hash recomputes correctly and its PrevHash equals the previous
// entry's Hash. It returns the index of the first broken entry, or -1 if valid.
func Verify(entries []Entry) (int, error) {
	prev := genesisHash
	for i, e := range entries {
		if e.PrevHash != prev {
			return i, fmt.Errorf("audit: entry %d prev_hash mismatch (chain broken)", e.Sequence)
		}
		if computeHash(e) != e.Hash {
			return i, fmt.Errorf("audit: entry %d hash mismatch (tampered)", e.Sequence)
		}
		prev = e.Hash
	}
	return -1, nil
}

// Package calllog writes one structured JSONL record per LLM call so cache
// behavior (and failures) can be reconstructed after the fact. Each record
// carries the wire-body hash — the byte-level proof of prefix stability —
// plus provider-reported cache usage and local prefix diagnostics. On a
// severe cache miss the full request body is dumped for post-mortem diffing.
package calllog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"nekocode/util/fs"
)

const (
	logFileName       = "llm-calls.jsonl"
	forensicsDir      = "cache-forensics"
	maxSize           = 10 << 20
	maxForensicsFiles = 20
	// severeMissMinTokens is the miss size above which a low hit ratio is
	// treated as a cache collapse worth a full body dump.
	severeMissMinTokens = 50_000
)

// PrefixDiag is the local view of the request's cache-relevant shape: which
// parts changed since the previous call and the fingerprints of the stable
// prefix. Hashes are 12 hex chars — enough to eyeball equality in a log.
type PrefixDiag struct {
	ChangedParts []string `json:"changed_parts,omitempty"`
	SystemHash   string   `json:"system_hash,omitempty"`
	ToolsHash    string   `json:"tools_hash,omitempty"`
	HistoryCount int      `json:"history_count,omitempty"`
	HistoryHash  string   `json:"history_hash,omitempty"`
}

// Record is one LLM call. Usage fields are provider-reported; zero means the
// provider did not report them (or the call failed before usage arrived).
type Record struct {
	TS               time.Time `json:"ts"`
	Seq              uint64    `json:"seq"`
	Source           string    `json:"source"`
	Model            string    `json:"model,omitempty"`
	Protocol         string    `json:"protocol,omitempty"`
	BaseURL          string    `json:"base_url,omitempty"`
	BodySHA256       string    `json:"body_sha256,omitempty"`
	BodyBytes        int       `json:"body_bytes,omitempty"`
	DurMs            int64     `json:"dur_ms"`
	TTFTMs           int64     `json:"ttft_ms,omitempty"`
	PromptTokens     int       `json:"prompt_tokens,omitempty"`
	CacheHitTokens   int       `json:"cache_hit_tokens,omitempty"`
	CacheMissTokens  int       `json:"cache_miss_tokens,omitempty"`
	CompletionTokens int       `json:"completion_tokens,omitempty"`
	PrefixDiag       `json:"prefix,omitempty"`
	Err              string `json:"err,omitempty"`
	ForensicsFile    string `json:"forensics_file,omitempty"`
}

var (
	seq         atomic.Uint64
	defaultSink = &sink{path: filepath.Join(fs.NekocodeLogDir(), logFileName)}
)

type sink struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// Write appends one record to the JSONL call log. Best-effort: logging must
// never break an LLM call.
func Write(rec Record) {
	rec.Seq = seq.Add(1)
	if rec.TS.IsZero() {
		rec.TS = time.Now()
	}
	defaultSink.write(rec)
}

// SevereMiss reports whether a hit/miss pair looks like a cache collapse:
// a large prompt almost entirely uncached.
func SevereMiss(hit, miss int) bool {
	return miss >= severeMissMinTokens && hit*10 < miss
}

// DumpBodyOnSevereMiss persists the raw wire body for post-mortem diffing
// when the call was a cache collapse, and returns the file name ("" when no
// dump happened). Two collapse dumps with identical body_sha256 prove the
// provider dropped a byte-identical prompt. Cold-start calls always miss the
// whole prompt — that is expected, not a collapse — so they never dump.
func DumpBodyOnSevereMiss(rec *Record, body []byte) {
	if !SevereMiss(rec.CacheHitTokens, rec.CacheMissTokens) || len(body) == 0 {
		return
	}
	for _, part := range rec.ChangedParts {
		if part == "cold-start" {
			return
		}
	}
	name := rec.TS.Format("20060102T150405") + "_" + shortHash(rec.BodySHA256) + ".json"
	dir := filepath.Join(fs.NekocodeLogDir(), forensicsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
		return
	}
	pruneForensics(dir)
	rec.ForensicsFile = filepath.Join(forensicsDir, name)
}

func (s *sink) write(rec Record) {
	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.logFile()
	if f == nil {
		return
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		s.file = nil
	}
}

func (s *sink) logFile() *os.File {
	if s.file != nil {
		return s.file
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return nil
	}
	if fi, err := os.Stat(s.path); err == nil && fi.Size() > maxSize {
		rotated := s.path + ".1"
		_ = os.Remove(rotated)
		if err := os.Rename(s.path, rotated); err == nil {
			_ = os.Chmod(rotated, 0o600)
		}
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil
	}
	s.file = f
	return s.file
}

func pruneForensics(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) <= maxForensicsFiles {
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	// Timestamp-prefixed names sort chronologically.
	sort.Strings(names)
	for _, name := range names[:len(names)-maxForensicsFiles] {
		_ = os.Remove(filepath.Join(dir, name))
	}
}

// BodyHash returns the canonical hex digest recorded for a wire body.
func BodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func shortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

// ShortDigest renders a 32-byte digest as 12 hex chars for PrefixDiag.
func ShortDigest(digest [32]byte) string {
	return hex.EncodeToString(digest[:6])
}

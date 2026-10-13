// Package activity is the log a local tool keeps of what it did, for tools
// without a database: one JSON line per entry, read newest first.
package activity

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry is one thing the tool did.
type Entry struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"`
	Target string    `json:"target,omitempty"`
	Detail string    `json:"detail"`
	OK     bool      `json:"ok"`
}

// Log is an append-only JSONL file, safe for concurrent use.
type Log struct {
	Path string
	mu   sync.Mutex
}

// DataDir is $XDG_DATA_HOME/<app>, or ~/.local/share/<app>.
func DataDir(app string) string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, app)
}

// Append writes one entry. The file is 0600: it names what was changed where.
func (l *Log) Append(e Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(raw, '\n'))
	return errors.Join(err, f.Close())
}

// Recent returns up to limit entries, newest first; a missing file is empty,
// and a line that does not parse is skipped.
func (l *Log) Recent(limit int) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.Open(l.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var all []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			all = append(all, e)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

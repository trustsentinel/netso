// Package audit records brokered access sessions for monitoring: how many are
// live, and a bounded history of recent ones (who reached which peer, when, for
// how long). The hub relays ciphertext, so it records connection metadata only,
// never session content.
package audit

import (
	"encoding/json"
	"sync"
	"time"
)

// Session is one brokered access session.
type Session struct {
	Network string    `json:"network"`
	Peer    string    `json:"peer"`
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended,omitempty"` // zero while active
}

// Active reports whether the session is still open.
func (s Session) Active() bool { return s.Ended.IsZero() }

// MarshalJSON omits "ended" while a session is active and adds an "active" flag,
// so the status endpoint doesn't emit a zero timestamp.
func (s Session) MarshalJSON() ([]byte, error) {
	out := struct {
		Network string     `json:"network"`
		Peer    string     `json:"peer"`
		Started time.Time  `json:"started"`
		Ended   *time.Time `json:"ended,omitempty"`
		Active  bool       `json:"active"`
	}{Network: s.Network, Peer: s.Peer, Started: s.Started, Active: s.Active()}
	if !s.Ended.IsZero() {
		e := s.Ended
		out.Ended = &e
	}
	return json.Marshal(out)
}

// Log is a concurrency-safe, bounded history of sessions plus a live count.
type Log struct {
	mu      sync.Mutex
	max     int
	entries []*Session
	active  int
	now     func() time.Time // injectable for tests
}

// NewLog returns a log that keeps at most max recent sessions.
func NewLog(max int) *Log {
	if max < 1 {
		max = 1
	}
	return &Log{max: max, now: time.Now}
}

// Start records the beginning of a session and returns a handle to End later.
func (l *Log) Start(network, peer string) *Session {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := &Session{Network: network, Peer: peer, Started: l.now()}
	l.entries = append(l.entries, s)
	if len(l.entries) > l.max {
		l.entries = l.entries[len(l.entries)-l.max:]
	}
	l.active++
	return s
}

// End marks a session finished.
func (l *Log) End(s *Session) {
	if s == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if s.Ended.IsZero() {
		s.Ended = l.now()
		if l.active > 0 {
			l.active--
		}
	}
}

// Active returns the number of currently-open sessions.
func (l *Log) Active() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active
}

// Recent returns up to n most-recent sessions, newest first (as copies).
func (l *Log) Recent(n int) []Session {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n > len(l.entries) {
		n = len(l.entries)
	}
	out := make([]Session, 0, n)
	for i := len(l.entries) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, *l.entries[i])
	}
	return out
}

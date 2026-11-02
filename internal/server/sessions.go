package server

import "sync"

// sessions tracks running annotation sessions so that shutdown can stop
// accepting new ones, ask the running ones to quit, and wait until each has
// closed its log.
type sessions struct {
	// mu guards closed; begin adds to wg under mu, so wait never races a
	// session that starts after close.
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
	// closing is closed once by close; running sessions quit when it is.
	closing chan struct{}
}

func newSessions() *sessions {
	return &sessions{closing: make(chan struct{})}
}

// begin registers a new session; it reports false once shutdown has begun.
func (s *sessions) begin() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.wg.Add(1)
	return true
}

// end marks a session begun with begin as finished.
func (s *sessions) end() { s.wg.Done() }

// close refuses new sessions and tells running ones to quit.
func (s *sessions) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.closing)
	}
}

// wait blocks until every begun session has ended.
func (s *sessions) wait() { s.wg.Wait() }

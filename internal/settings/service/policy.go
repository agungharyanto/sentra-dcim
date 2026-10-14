package service

import (
	"context"
	"sync"
	"time"
)

// Policy is the effective session and password policy (v0.3.7, D-069/D-070/D-079).
type Policy struct {
	Idle       time.Duration
	Absolute   time.Duration
	ExpiryDays int // 0 = passwords never expire
	WarnDays   int
	History    int // previous passwords that may not be re-used
}

// policyTTL bounds how long a changed policy takes to apply (every request reads it).
const policyTTL = 10 * time.Second

type policyCache struct {
	mu  sync.Mutex
	val Policy
	at  time.Time
}

// Policy returns the session/password policy, cached for a few seconds (it is read on every authenticated request).
func (s *Service) Policy(ctx context.Context) (Policy, error) {
	s.pc.mu.Lock()
	if !s.pc.at.IsZero() && time.Since(s.pc.at) < policyTTL {
		v := s.pc.val
		s.pc.mu.Unlock()
		return v, nil
	}
	s.pc.mu.Unlock()
	sess, err := s.Group(ctx, "session")
	if err != nil {
		return Policy{}, err
	}
	pw, err := s.Group(ctx, "password")
	if err != nil {
		return Policy{}, err
	}
	get := func(items []Item, key string) int64 {
		for _, it := range items {
			if it.Key == key {
				n, _ := it.Value.(int64)
				return n
			}
		}
		return 0
	}
	p := Policy{
		Idle:       time.Duration(get(sess, KeySessionIdleMinutes)) * time.Minute,
		Absolute:   time.Duration(get(sess, KeySessionAbsoluteHours)) * time.Hour,
		ExpiryDays: int(get(pw, KeyPasswordExpiryDays)),
		WarnDays:   int(get(pw, KeyPasswordWarnDays)),
		History:    int(get(pw, KeyPasswordHistory)),
	}
	s.pc.mu.Lock()
	s.pc.val, s.pc.at = p, time.Now()
	s.pc.mu.Unlock()
	return p, nil
}

// invalidatePolicy makes the next Policy call read the database (after a change in this process).
func (s *Service) invalidatePolicy() {
	s.pc.mu.Lock()
	s.pc.at = time.Time{}
	s.pc.mu.Unlock()
}

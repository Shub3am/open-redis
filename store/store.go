// Package store is the in-memory keyspace. It owns value types, key expiry and
// the locking that makes the keyspace safe to share between connections.
//
// It must not know about RESP or command names. Errors it returns carry the
// exact text Redis replies with, so callers can forward them unchanged.
package store

import (
	"container/list"
	"errors"
	"math"
	"strconv"
	"sync"
	"time"
)

var (
	ErrWrongType  = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")
	ErrNotInteger = errors.New("ERR value is not an integer or out of range")
	ErrOverflow   = errors.New("ERR increment or decrement would overflow")
)

// Kind is the value type of a key, spelled the way the TYPE command reports it.
type Kind string

const (
	KindNone   Kind = "none"
	KindString Kind = "string"
	KindList   Kind = "list"
)

type entry struct {
	kind Kind
	text string
	// items is only set for KindList. A linked list keeps pushes and pops at
	// either end O(1), which LPUSH and LPOP depend on.
	items *list.List
}

// Store is a keyspace guarded by one mutex. Reads take the same lock as
// writes because a read can delete an expired key.
type Store struct {
	mu      sync.Mutex
	entries map[string]*entry
	// expiresAt holds only keys that have a TTL, so the active sweep samples
	// from volatile keys instead of the whole keyspace.
	expiresAt map[string]time.Time
	now       func() time.Time
}

func New() *Store {
	return &Store{
		entries:   make(map[string]*entry),
		expiresAt: make(map[string]time.Time),
		now:       time.Now,
	}
}

// SetOptions mirrors the SET flags. A zero TTL means the key never expires.
type SetOptions struct {
	TTL           time.Duration
	OnlyIfMissing bool
	OnlyIfExists  bool
}

// Get returns the string at key. found is false when the key does not exist.
func (s *Store) Get(key string) (value string, found bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.lookup(key)
	if current == nil {
		return "", false, nil
	}
	if current.kind != KindString {
		return "", false, ErrWrongType
	}
	return current.text, true, nil
}

// Set stores a string, replacing any value of any type and clearing any old
// TTL. It reports false when NX or XX prevented the write.
func (s *Store) Set(key, value string, options SetOptions) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exists := s.lookup(key) != nil
	if (options.OnlyIfMissing && exists) || (options.OnlyIfExists && !exists) {
		return false
	}
	s.entries[key] = &entry{kind: KindString, text: value}
	if options.TTL > 0 {
		s.expiresAt[key] = s.now().Add(options.TTL)
	} else {
		delete(s.expiresAt, key)
	}
	return true
}

// IncrBy adds delta to the integer stored at key, treating a missing key as 0,
// and keeps any TTL the key already has.
func (s *Store) IncrBy(key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var current int64
	if existing := s.lookup(key); existing != nil {
		if existing.kind != KindString {
			return 0, ErrWrongType
		}
		parsed, ok := ParseInteger(existing.text)
		if !ok {
			return 0, ErrNotInteger
		}
		current = parsed
	}
	if (delta > 0 && current > math.MaxInt64-delta) || (delta < 0 && current < math.MinInt64-delta) {
		return 0, ErrOverflow
	}
	next := current + delta
	s.entries[key] = &entry{kind: KindString, text: strconv.FormatInt(next, 10)}
	return next, nil
}

// Delete removes keys and returns how many existed.
func (s *Store) Delete(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := 0
	for _, key := range keys {
		if s.lookup(key) != nil {
			s.remove(key)
			deleted++
		}
	}
	return deleted
}

// Exists counts how many of keys exist. A key named twice counts twice.
func (s *Store) Exists(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := 0
	for _, key := range keys {
		if s.lookup(key) != nil {
			found++
		}
	}
	return found
}

func (s *Store) Type(key string) Kind {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.lookup(key)
	if current == nil {
		return KindNone
	}
	return current.kind
}

// Keys returns every live key matching a Redis glob pattern, in no order.
func (s *Store) Keys(pattern string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := []string{}
	for key := range s.entries {
		if s.lookup(key) != nil && matchGlob(pattern, key) {
			matched = append(matched, key)
		}
	}
	return matched
}

func (s *Store) FlushAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.entries)
	clear(s.expiresAt)
}

// Expire sets a TTL on an existing key and reports whether the key existed.
// A TTL that is zero or negative deletes the key, as in Redis.
func (s *Store) Expire(key string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lookup(key) == nil {
		return false
	}
	if ttl <= 0 {
		s.remove(key)
		return true
	}
	s.expiresAt[key] = s.now().Add(ttl)
	return true
}

// TTL reports the time left on key. exists is false for a missing key and
// hasExpiry is false for a key that never expires.
func (s *Store) TTL(key string) (remaining time.Duration, hasExpiry bool, exists bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lookup(key) == nil {
		return 0, false, false
	}
	deadline, hasExpiry := s.expiresAt[key]
	if !hasExpiry {
		return 0, false, true
	}
	return deadline.Sub(s.now()), true, true
}

// lookup returns the live entry for key, deleting it first if it has expired.
// This is the lazy half of expiry. The caller must hold s.mu.
func (s *Store) lookup(key string) *entry {
	current, ok := s.entries[key]
	if !ok {
		return nil
	}
	if deadline, hasExpiry := s.expiresAt[key]; hasExpiry && !s.now().Before(deadline) {
		s.remove(key)
		return nil
	}
	return current
}

// remove deletes key and its TTL. The caller must hold s.mu.
func (s *Store) remove(key string) {
	delete(s.entries, key)
	delete(s.expiresAt, key)
}

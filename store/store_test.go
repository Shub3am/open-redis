package store

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
)

// fakeClock lets expiry tests move time forward without sleeping.
type fakeClock struct {
	current time.Time
}

func (c *fakeClock) now() time.Time              { return c.current }
func (c *fakeClock) advance(delta time.Duration) { c.current = c.current.Add(delta) }

func newStoreWithClock() (*Store, *fakeClock) {
	clock := &fakeClock{current: time.Unix(1_700_000_000, 0)}
	keyspace := New()
	keyspace.now = clock.now
	return keyspace, clock
}

func TestSetThenGet(t *testing.T) {
	keyspace := New()
	if _, found, _ := keyspace.Get("missing"); found {
		t.Fatal("missing key reported as found")
	}
	keyspace.Set("greeting", "hello", SetOptions{})
	value, found, err := keyspace.Get("greeting")
	if err != nil || !found || value != "hello" {
		t.Fatalf("got (%q, %v, %v), want (hello, true, nil)", value, found, err)
	}
}

func TestSetOnlyIfMissingAndOnlyIfExists(t *testing.T) {
	keyspace := New()
	if keyspace.Set("k", "v1", SetOptions{OnlyIfExists: true}) {
		t.Fatal("XX wrote a missing key")
	}
	if !keyspace.Set("k", "v1", SetOptions{OnlyIfMissing: true}) {
		t.Fatal("NX refused a missing key")
	}
	if keyspace.Set("k", "v2", SetOptions{OnlyIfMissing: true}) {
		t.Fatal("NX overwrote an existing key")
	}
	if !keyspace.Set("k", "v3", SetOptions{OnlyIfExists: true}) {
		t.Fatal("XX refused an existing key")
	}
	if value, _, _ := keyspace.Get("k"); value != "v3" {
		t.Fatalf("got %q, want v3", value)
	}
}

func TestLazyExpiryHidesAndDeletesKey(t *testing.T) {
	keyspace, clock := newStoreWithClock()
	keyspace.Set("session", "abc", SetOptions{TTL: time.Second})

	clock.advance(999 * time.Millisecond)
	if _, found, _ := keyspace.Get("session"); !found {
		t.Fatal("key expired early")
	}
	clock.advance(time.Millisecond)
	if _, found, _ := keyspace.Get("session"); found {
		t.Fatal("key readable after its deadline")
	}
	if _, stillStored := keyspace.entries["session"]; stillStored {
		t.Fatal("expired key was not deleted on access")
	}
}

func TestSetWithoutTTLClearsOldTTL(t *testing.T) {
	keyspace, clock := newStoreWithClock()
	keyspace.Set("k", "v", SetOptions{TTL: time.Second})
	keyspace.Set("k", "v", SetOptions{})
	clock.advance(time.Hour)
	if _, found, _ := keyspace.Get("k"); !found {
		t.Fatal("plain SET did not clear the previous TTL")
	}
}

func TestExpireAndTTL(t *testing.T) {
	keyspace, clock := newStoreWithClock()
	if keyspace.Expire("missing", time.Second) {
		t.Fatal("Expire reported success on a missing key")
	}
	if _, _, exists := keyspace.TTL("missing"); exists {
		t.Fatal("TTL reported a missing key as existing")
	}

	keyspace.Set("k", "v", SetOptions{})
	if _, hasExpiry, exists := keyspace.TTL("k"); !exists || hasExpiry {
		t.Fatal("persistent key should exist without expiry")
	}
	if !keyspace.Expire("k", 10*time.Second) {
		t.Fatal("Expire failed on an existing key")
	}
	clock.advance(4 * time.Second)
	remaining, hasExpiry, _ := keyspace.TTL("k")
	if !hasExpiry || remaining != 6*time.Second {
		t.Fatalf("got remaining %v, want 6s", remaining)
	}

	if !keyspace.Expire("k", -1) {
		t.Fatal("negative Expire should report the key existed")
	}
	if keyspace.Exists("k") != 0 {
		t.Fatal("negative Expire should delete the key")
	}
}

func TestDeleteExistsTypeFlushAll(t *testing.T) {
	keyspace := New()
	keyspace.Set("a", "1", SetOptions{})
	keyspace.Set("b", "2", SetOptions{})

	if got := keyspace.Exists("a", "a", "missing"); got != 2 {
		t.Fatalf("Exists counted %d, want 2", got)
	}
	if got := keyspace.Type("a"); got != KindString {
		t.Fatalf("Type = %q, want string", got)
	}
	if got := keyspace.Type("missing"); got != KindNone {
		t.Fatalf("Type = %q, want none", got)
	}
	if got := keyspace.Delete("a", "missing"); got != 1 {
		t.Fatalf("Delete removed %d, want 1", got)
	}
	keyspace.FlushAll()
	if keyspace.Exists("b") != 0 {
		t.Fatal("FlushAll left a key behind")
	}
}

func TestKeysSkipsExpiredKeys(t *testing.T) {
	keyspace, clock := newStoreWithClock()
	keyspace.Set("user:1", "a", SetOptions{})
	keyspace.Set("user:2", "b", SetOptions{TTL: time.Second})
	keyspace.Set("order:1", "c", SetOptions{})
	clock.advance(time.Second)

	got := keyspace.Keys("user:*")
	slices.Sort(got)
	if !slices.Equal(got, []string{"user:1"}) {
		t.Fatalf("got %q, want [user:1]", got)
	}
}

func TestActiveSweepRemovesUnreadExpiredKeys(t *testing.T) {
	keyspace, clock := newStoreWithClock()
	for i := range 1000 {
		keyspace.Set(fmt.Sprintf("temp:%d", i), "v", SetOptions{TTL: time.Second})
	}
	keyspace.Set("durable", "v", SetOptions{TTL: time.Hour})
	clock.advance(time.Second)

	keyspace.sweepExpired()

	// With one live key among the volatile ones, every round finds more than
	// a quarter expired until none are left, so the sweep drains them all.
	if len(keyspace.entries) != 1 {
		t.Fatalf("%d keys left after sweep, want 1", len(keyspace.entries))
	}
	if _, found := keyspace.entries["durable"]; !found {
		t.Fatal("sweep removed a key that had not expired")
	}
}

func TestRunActiveExpiryStopsOnCancel(t *testing.T) {
	keyspace := New()
	keyspace.Set("k", "v", SetOptions{TTL: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		keyspace.RunActiveExpiry(ctx, time.Millisecond)
		close(stopped)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		keyspace.mu.Lock()
		remaining := len(keyspace.entries)
		keyspace.mu.Unlock()
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("active expiry never removed the key")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-stopped
}

func TestConcurrentAccessIsSafe(t *testing.T) {
	keyspace := New()
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range 1000 {
				key := fmt.Sprintf("k%d", i%50)
				keyspace.Set(key, fmt.Sprint(worker), SetOptions{TTL: time.Millisecond})
				keyspace.Get(key)
				keyspace.Keys("k*")
				keyspace.Delete(key)
			}
		}()
	}
	workers.Wait()
}

func BenchmarkSet(b *testing.B) {
	keyspace := New()
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("key:%d", i)
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		keyspace.Set(keys[i%len(keys)], "value", SetOptions{})
	}
}

func BenchmarkGet(b *testing.B) {
	keyspace := New()
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("key:%d", i)
		keyspace.Set(keys[i], "value", SetOptions{})
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		keyspace.Get(keys[i%len(keys)])
	}
}

func BenchmarkGetParallel(b *testing.B) {
	keyspace := New()
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("key:%d", i)
		keyspace.Set(keys[i], "value", SetOptions{})
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			keyspace.Get(keys[i%len(keys)])
			i++
		}
	})
}

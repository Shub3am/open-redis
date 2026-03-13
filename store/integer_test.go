package store

import (
	"errors"
	"math"
	"strconv"
	"testing"
)

func TestParseIntegerAcceptsOnlyCanonicalForm(t *testing.T) {
	accepted := map[string]int64{"0": 0, "42": 42, "-7": -7, "9223372036854775807": math.MaxInt64}
	for text, want := range accepted {
		if got, ok := ParseInteger(text); !ok || got != want {
			t.Errorf("ParseInteger(%q) = (%d, %v), want (%d, true)", text, got, ok, want)
		}
	}
	for _, text := range []string{"", "+5", "05", "-0", " 5", "5 ", "1.5", "abc", "9223372036854775808"} {
		if _, ok := ParseInteger(text); ok {
			t.Errorf("ParseInteger(%q) accepted a non-canonical integer", text)
		}
	}
}

func TestIncrBy(t *testing.T) {
	keyspace := New()
	if got, err := keyspace.IncrBy("counter", 5); err != nil || got != 5 {
		t.Fatalf("missing key: got (%d, %v), want (5, nil)", got, err)
	}
	if got, err := keyspace.IncrBy("counter", -8); err != nil || got != -3 {
		t.Fatalf("got (%d, %v), want (-3, nil)", got, err)
	}

	keyspace.Set("text", "hello", SetOptions{})
	if _, err := keyspace.IncrBy("text", 1); !errors.Is(err, ErrNotInteger) {
		t.Fatalf("non-integer value: got %v, want ErrNotInteger", err)
	}

	keyspace.Set("max", strconv.FormatInt(math.MaxInt64, 10), SetOptions{})
	if _, err := keyspace.IncrBy("max", 1); !errors.Is(err, ErrOverflow) {
		t.Fatalf("overflow: got %v, want ErrOverflow", err)
	}
	keyspace.Set("min", strconv.FormatInt(math.MinInt64, 10), SetOptions{})
	if _, err := keyspace.IncrBy("min", -1); !errors.Is(err, ErrOverflow) {
		t.Fatalf("underflow: got %v, want ErrOverflow", err)
	}
}

func TestIncrByKeepsTTL(t *testing.T) {
	keyspace, _ := newStoreWithClock()
	keyspace.Set("counter", "1", SetOptions{TTL: 10})
	keyspace.IncrBy("counter", 1)
	if _, hasExpiry, _ := keyspace.TTL("counter"); !hasExpiry {
		t.Fatal("IncrBy dropped the key's TTL")
	}
}

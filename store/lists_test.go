package store

import (
	"errors"
	"slices"
	"testing"
)

func TestPushOrderAndLength(t *testing.T) {
	keyspace := New()
	if length, _ := keyspace.PushBack("l", "b", "c"); length != 2 {
		t.Fatalf("PushBack length = %d, want 2", length)
	}
	if length, _ := keyspace.PushFront("l", "a", "z"); length != 4 {
		t.Fatalf("PushFront length = %d, want 4", length)
	}
	got, _ := keyspace.Range("l", 0, -1)
	if want := []string{"z", "a", "b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if length, _ := keyspace.Len("l"); length != 4 {
		t.Fatalf("Len = %d, want 4", length)
	}
	if keyspace.Type("l") != KindList {
		t.Fatal("Type did not report list")
	}
}

func TestRangeIndexes(t *testing.T) {
	keyspace := New()
	keyspace.PushBack("l", "a", "b", "c", "d", "e")
	tests := []struct {
		start, stop int64
		want        []string
	}{
		{0, 0, []string{"a"}},
		{1, 3, []string{"b", "c", "d"}},
		{-2, -1, []string{"d", "e"}},
		{-100, 1, []string{"a", "b"}},
		{3, 100, []string{"d", "e"}},
		{4, 2, []string{}},
		{10, 20, []string{}},
		{-1, -3, []string{}},
	}
	for _, test := range tests {
		got, err := keyspace.Range("l", test.start, test.stop)
		if err != nil || !slices.Equal(got, test.want) {
			t.Errorf("Range(%d, %d) = %q, %v; want %q", test.start, test.stop, got, err, test.want)
		}
	}
	if got, _ := keyspace.Range("missing", 0, -1); len(got) != 0 {
		t.Fatalf("missing key returned %q", got)
	}
}

func TestPopDeletesEmptiedList(t *testing.T) {
	keyspace := New()
	keyspace.PushBack("l", "a", "b")
	if value, found, _ := keyspace.PopFront("l"); !found || value != "a" {
		t.Fatalf("PopFront = %q, %v", value, found)
	}
	if value, found, _ := keyspace.PopBack("l"); !found || value != "b" {
		t.Fatalf("PopBack = %q, %v", value, found)
	}
	if keyspace.Exists("l") != 0 {
		t.Fatal("empty list key still exists")
	}
	if _, found, _ := keyspace.PopFront("l"); found {
		t.Fatal("pop from missing key reported a value")
	}
}

func TestListAndStringTypesDoNotMix(t *testing.T) {
	keyspace := New()
	keyspace.Set("s", "v", SetOptions{})
	keyspace.PushBack("l", "a")

	if _, err := keyspace.PushBack("s", "x"); !errors.Is(err, ErrWrongType) {
		t.Errorf("PushBack on string: %v", err)
	}
	if _, _, err := keyspace.PopFront("s"); !errors.Is(err, ErrWrongType) {
		t.Errorf("PopFront on string: %v", err)
	}
	if _, err := keyspace.Range("s", 0, -1); !errors.Is(err, ErrWrongType) {
		t.Errorf("Range on string: %v", err)
	}
	if _, err := keyspace.Len("s"); !errors.Is(err, ErrWrongType) {
		t.Errorf("Len on string: %v", err)
	}
	if _, _, err := keyspace.Get("l"); !errors.Is(err, ErrWrongType) {
		t.Errorf("Get on list: %v", err)
	}
	if _, err := keyspace.IncrBy("l", 1); !errors.Is(err, ErrWrongType) {
		t.Errorf("IncrBy on list: %v", err)
	}
}

func BenchmarkPushFront(b *testing.B) {
	keyspace := New()
	b.ReportAllocs()
	for b.Loop() {
		keyspace.PushFront("list", "value")
	}
}

// This file holds the list operations of the keyspace. An emptied list is
// deleted, so a list key never exists without elements. It must not touch
// string values beyond reporting WRONGTYPE.
package store

import "container/list"

// PushFront inserts values at the head one at a time, so the last value ends
// up first, and returns the new length.
func (s *Store) PushFront(key string, values ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.listForWrite(key)
	if err != nil {
		return 0, err
	}
	for _, value := range values {
		items.PushFront(value)
	}
	return items.Len(), nil
}

// PushBack appends values at the tail and returns the new length.
func (s *Store) PushBack(key string, values ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.listForWrite(key)
	if err != nil {
		return 0, err
	}
	for _, value := range values {
		items.PushBack(value)
	}
	return items.Len(), nil
}

// PopFront removes and returns the head. found is false for a missing key.
func (s *Store) PopFront(key string) (value string, found bool, err error) {
	return s.pop(key, (*list.List).Front)
}

// PopBack removes and returns the tail. found is false for a missing key.
func (s *Store) PopBack(key string) (value string, found bool, err error) {
	return s.pop(key, (*list.List).Back)
}

// Range returns the elements from start to stop inclusive. Negative indexes
// count from the tail and out-of-range indexes are clamped, as in LRANGE.
func (s *Store) Range(key string, start, stop int64) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.listForRead(key)
	if err != nil || items == nil {
		return []string{}, err
	}

	length := int64(items.Len())
	if start < 0 {
		start += length
	}
	if stop < 0 {
		stop += length
	}
	start = max(start, 0)
	stop = min(stop, length-1)
	if start > stop {
		return []string{}, nil
	}

	selected := make([]string, 0, stop-start+1)
	element := items.Front()
	for range start {
		element = element.Next()
	}
	for index := start; index <= stop; index++ {
		selected = append(selected, element.Value.(string))
		element = element.Next()
	}
	return selected, nil
}

// Len returns the list length, 0 for a missing key.
func (s *Store) Len(key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.listForRead(key)
	if err != nil || items == nil {
		return 0, err
	}
	return items.Len(), nil
}

func (s *Store) pop(key string, end func(*list.List) *list.Element) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.listForRead(key)
	if err != nil || items == nil {
		return "", false, err
	}
	value := items.Remove(end(items)).(string)
	if items.Len() == 0 {
		s.remove(key)
	}
	return value, true, nil
}

// listForRead returns the list at key, or nil if the key is missing. The
// caller must hold s.mu.
func (s *Store) listForRead(key string) (*list.List, error) {
	current := s.lookup(key)
	if current == nil {
		return nil, nil
	}
	if current.kind != KindList {
		return nil, ErrWrongType
	}
	return current.items, nil
}

// listForWrite returns the list at key, creating an empty one if the key is
// missing. The caller must hold s.mu and must add at least one element.
func (s *Store) listForWrite(key string) (*list.List, error) {
	items, err := s.listForRead(key)
	if err != nil || items != nil {
		return items, err
	}
	items = list.New()
	s.entries[key] = &entry{kind: KindList, items: items}
	return items, nil
}

package session

import (
	"container/list"
	"sync"
	"time"
)

type entry struct {
	key   string
	value *Session
	elem  *list.Element
}

type MemoryStore struct {
	mu       sync.RWMutex
	data     map[string]*entry
	lru      *list.List
	maxSize  int
	ttl      time.Duration
}

func NewMemoryStore(maxSize int, ttl time.Duration) *MemoryStore {
	return &MemoryStore{
		data:    make(map[string]*entry),
		lru:     list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func (s *MemoryStore) Get(sessionID string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[sessionID]
	if !ok {
		return nil, nil
	}
	if time.Since(e.value.LastAccess) > e.value.TTL {
		delete(s.data, sessionID)
		s.lru.Remove(e.elem)
		return nil, nil
	}
	e.value.LastAccess = time.Now()
	s.lru.MoveToFront(e.elem)
	return e.value, nil
}

func (s *MemoryStore) Set(sessionID string, sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess.TTL == 0 {
		sess.TTL = s.ttl
	}
	sess.LastAccess = time.Now()

	if e, ok := s.data[sessionID]; ok {
		e.value = sess
		s.lru.MoveToFront(e.elem)
		return nil
	}

	e := &entry{key: sessionID, value: sess}
	e.elem = s.lru.PushFront(e)
	s.data[sessionID] = e

	for s.lru.Len() > s.maxSize {
		oldest := s.lru.Back()
		if oldest != nil {
			oldEntry := oldest.Value.(*entry)
			delete(s.data, oldEntry.key)
			s.lru.Remove(oldest)
		}
	}
	return nil
}

func (s *MemoryStore) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.data[sessionID]; ok {
		delete(s.data, sessionID)
		s.lru.Remove(e.elem)
	}
	return nil
}

func (s *MemoryStore) Prune() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var toRemove []*list.Element
	for e := s.lru.Back(); e != nil; e = e.Prev() {
		entry := e.Value.(*entry)
		if time.Since(entry.value.LastAccess) > entry.value.TTL {
			toRemove = append(toRemove, e)
		}
	}
	for _, e := range toRemove {
		entry := e.Value.(*entry)
		delete(s.data, entry.key)
		s.lru.Remove(e)
	}
}

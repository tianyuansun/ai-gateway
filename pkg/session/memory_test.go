package session

import (
	"testing"
	"time"
)

func TestMemoryStore_SetGet(t *testing.T) {
	s := NewMemoryStore(10, 1*time.Hour)
	sess := &Session{ID: "s1", ModelName: "gpt-4"}
	if err := s.Set("s1", sess); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := s.Get("s1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if got.ID != "s1" {
		t.Errorf("expected ID 's1', got '%s'", got.ID)
	}
}

func TestMemoryStore_GetNonexistent(t *testing.T) {
	s := NewMemoryStore(10, 1*time.Hour)
	got, err := s.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent key")
	}
}

func TestMemoryStore_Overwrite(t *testing.T) {
	s := NewMemoryStore(10, 1*time.Hour)

	s.Set("s1", &Session{ID: "s1", ModelName: "gpt-4"})
	s.Set("s1", &Session{ID: "s1", ModelName: "gpt-4o"})

	got, err := s.Get("s1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ModelName != "gpt-4o" {
		t.Errorf("expected ModelName 'gpt-4o', got '%s'", got.ModelName)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	s := NewMemoryStore(10, 1*time.Hour)

	s.Set("s1", &Session{ID: "s1"})
	s.Delete("s1")

	got, err := s.Get("s1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestMemoryStore_TTLExpiry(t *testing.T) {
	s := NewMemoryStore(10, 10*time.Millisecond)

	s.Set("s1", &Session{ID: "s1"})

	// Immediately after, should be present
	got, err := s.Get("s1")
	if err != nil || got == nil {
		t.Fatal("expected session immediately after Set")
	}

	// Sleep past TTL
	time.Sleep(15 * time.Millisecond)

	got, err = s.Get("s1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Error("expected nil after TTL expiry")
	}
}

func TestMemoryStore_LRUEviction(t *testing.T) {
	s := NewMemoryStore(3, 1*time.Hour)

	s.Set("a", &Session{ID: "a"})
	s.Set("b", &Session{ID: "b"})
	s.Set("c", &Session{ID: "c"})
	// This should evict "a"
	s.Set("d", &Session{ID: "d"})

	got, err := s.Get("a")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Error("expected 'a' to be evicted")
	}

	// b, c, d should still exist
	for _, id := range []string{"b", "c", "d"} {
		g, err := s.Get(id)
		if err != nil {
			t.Fatalf("Get(%s) failed: %v", id, err)
		}
		if g == nil {
			t.Errorf("expected '%s' to exist", id)
		}
	}
}

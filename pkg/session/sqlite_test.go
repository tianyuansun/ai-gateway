package session

import (
	"os"
	"testing"
	"time"
)

func TestSQLiteStore_SetGet(t *testing.T) {
	path := "/tmp/test_sqlite_setget.db"
	os.Remove(path)
	defer os.Remove(path)

	store, err := NewSQLiteStore(path, time.Hour)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	sess := &Session{
		ID:        "sess-1",
		ModelName: "test-model",
		ProviderID: "p1",
		Messages: []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
		},
		ReasoningRecords: []Reasoning{
			{Content: "let me think..."},
		},
	}

	if err := store.Set("sess-1", sess); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := store.Get("sess-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if got.ID != "sess-1" || got.ModelName != "test-model" {
		t.Errorf("ID/Model mismatch: %+v", got)
	}
}

func TestSQLiteStore_GetNonexistent(t *testing.T) {
	path := "/tmp/test_sqlite_nonexistent.db"
	os.Remove(path)
	defer os.Remove(path)
	store, _ := NewSQLiteStore(path, time.Hour)
	sess, _ := store.Get("no-such-key")
	if sess != nil {
		t.Error("expected nil for nonexistent key")
	}
}

func TestSQLiteStore_Delete(t *testing.T) {
	path := "/tmp/test_sqlite_delete.db"
	os.Remove(path)
	defer os.Remove(path)
	store, _ := NewSQLiteStore(path, time.Hour)
	store.Set("sess-1", &Session{ID: "sess-1"})
	store.Delete("sess-1")
	sess, _ := store.Get("sess-1")
	if sess != nil {
		t.Error("expected nil after delete")
	}
}

func TestSQLiteStore_TTLAndPrune(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TTL test in short mode")
	}
	path := "/tmp/test_sqlite_ttl.db"
	os.Remove(path)
	defer os.Remove(path)

	store, _ := NewSQLiteStore(path, 2*time.Second)
	store.Set("sess-1", &Session{ID: "sess-1"})

	sess, _ := store.Get("sess-1")
	if sess == nil {
		t.Fatal("expected session before TTL")
	}

	time.Sleep(3 * time.Second)
	sess, _ = store.Get("sess-1")
	if sess != nil {
		t.Error("expected nil after TTL expiry")
	}

	store.Prune()
	var count int
	store.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 rows after prune, got %d", count)
	}
}

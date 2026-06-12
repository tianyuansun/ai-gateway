package router

import (
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/provider"
	"github.com/tianyuansun/ai-gateway/pkg/session"
)

func TestSelectWeightedSkipsUnhealthy(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Chat: "http://p1"}},
			"p2": {Endpoints: config.ProviderEndpoints{Chat: "http://p2"}},
		},
	}

	checker := provider.NewHealthChecker(30)
	checker.SetHealth("p1", false)
	checker.SetHealth("p2", true)

	selector := NewProviderSelector(cfg, nil, checker)

	model := &config.Model{
		Routing: &config.RoutingConfig{Strategy: "weighted"},
		Providers: []config.ModelProvider{
			{Provider: "p1", Priority: 5},
			{Provider: "p2", Priority: 1},
		},
	}

	// Run multiple times — unhealthy provider should never be selected.
	for i := 0; i < 50; i++ {
		_, provID, err := selector.Select(model, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provID == "p1" {
			t.Errorf("weighted routing selected unhealthy provider p1")
		}
		if provID != "p2" {
			t.Errorf("expected p2 (only healthy), got %s", provID)
		}
	}
}

func TestSelectPrioritySkipsUnhealthy(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Chat: "http://p1"}},
			"p2": {Endpoints: config.ProviderEndpoints{Chat: "http://p2"}},
		},
	}

	checker := provider.NewHealthChecker(30)
	checker.SetHealth("p1", false)
	checker.SetHealth("p2", true)

	selector := NewProviderSelector(cfg, nil, checker)

	model := &config.Model{
		Routing: &config.RoutingConfig{Strategy: "priority"},
		Providers: []config.ModelProvider{
			{Provider: "p1", Priority: 1},
			{Provider: "p2", Priority: 2},
		},
	}

	_, provID, err := selector.Select(model, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provID != "p2" {
		t.Errorf("expected p2 (healthy), got %s", provID)
	}
}

func TestSelectRandomDistribution(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Chat: "http://p1"}},
			"p2": {Endpoints: config.ProviderEndpoints{Chat: "http://p2"}},
			"p3": {Endpoints: config.ProviderEndpoints{Chat: "http://p3"}},
		},
	}

	checker := provider.NewHealthChecker(30)
	checker.SetHealth("p1", true)
	checker.SetHealth("p2", true)
	checker.SetHealth("p3", true)

	selector := NewProviderSelector(cfg, nil, checker)

	model := &config.Model{
		Routing: &config.RoutingConfig{Strategy: "random"},
		Providers: []config.ModelProvider{
			{Provider: "p1", Priority: 1},
			{Provider: "p2", Priority: 1},
			{Provider: "p3", Priority: 1},
		},
	}

	counts := map[string]int{"p1": 0, "p2": 0, "p3": 0}
	const iterations = 300
	for range iterations {
		_, provID, err := selector.Select(model, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[provID]++
	}

	// With 300 iterations and 3 providers, each should get roughly 100 selections.
	// Allow ±40% tolerance to avoid flaky tests.
	for id, count := range counts {
		if count < 60 || count > 140 {
			t.Errorf("provider %s got %d/%d selections (expected ~100, range 60-140)", id, count, iterations)
		}
	}
}

func TestSelectRandomAllUnhealthyReturnsError(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Chat: "http://p1"}},
			"p2": {Endpoints: config.ProviderEndpoints{Chat: "http://p2"}},
		},
	}

	checker := provider.NewHealthChecker(30)
	checker.SetHealth("p1", false)
	checker.SetHealth("p2", false)

	selector := NewProviderSelector(cfg, nil, checker)

	model := &config.Model{
		Routing: &config.RoutingConfig{Strategy: "random"},
		Providers: []config.ModelProvider{
			{Provider: "p1", Priority: 1},
			{Provider: "p2", Priority: 1},
		},
	}

	_, _, err := selector.Select(model, "")
	if err == nil {
		t.Fatal("expected error when all providers unhealthy, got nil")
	}
}

func TestSelectAllUnhealthyReturnsError(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Chat: "http://p1"}},
		},
	}

	checker := provider.NewHealthChecker(30)
	checker.SetHealth("p1", false)

	selector := NewProviderSelector(cfg, nil, checker)

	model := &config.Model{
		Routing: &config.RoutingConfig{Strategy: "priority"},
		Providers: []config.ModelProvider{
			{Provider: "p1", Priority: 1},
		},
	}

	_, _, err := selector.Select(model, "")
	if err == nil {
		t.Fatal("expected error when all providers unhealthy, got nil")
	}
}

// mockSessionStore implements session.Store for testing.
type mockSessionStore struct {
	sessions map[string]*session.Session
}

func (m *mockSessionStore) Get(id string) (*session.Session, error) {
	if m.sessions == nil {
		return nil, nil
	}
	s, ok := m.sessions[id]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (m *mockSessionStore) Set(id string, s *session.Session) error {
	if m.sessions == nil {
		m.sessions = make(map[string]*session.Session)
	}
	m.sessions[id] = s
	return nil
}

func (m *mockSessionStore) Delete(id string) error {
	if m.sessions != nil {
		delete(m.sessions, id)
	}
	return nil
}

func (m *mockSessionStore) Prune() {}

func TestSelectEmptyProvidersReturnsError(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.Provider{},
	}

	checker := provider.NewHealthChecker(30)
	selector := NewProviderSelector(cfg, nil, checker)

	model := &config.Model{
		Routing:   &config.RoutingConfig{Strategy: "priority"},
		Providers: []config.ModelProvider{},
	}

	_, _, err := selector.Select(model, "")
	if err == nil {
		t.Fatal("expected error when providers list is empty, got nil")
	}
	if err.Error() == "no healthy provider available" {
		t.Errorf("expected 'no providers configured' error, got 'no healthy provider available'")
	}
}

func TestSelectSessionAffinity(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Chat: "http://p1"}},
			"p2": {Endpoints: config.ProviderEndpoints{Chat: "http://p2"}},
		},
	}

	store := &mockSessionStore{
		sessions: map[string]*session.Session{
			"sess-1": {ProviderID: "p2"},
		},
	}

	checker := provider.NewHealthChecker(30)
	// Make all providers healthy so affinity is the deciding factor.
	checker.SetHealth("p1", true)
	checker.SetHealth("p2", true)

	selector := NewProviderSelector(cfg, store, checker)

	model := &config.Model{
		Routing: &config.RoutingConfig{Strategy: "priority"},
		Providers: []config.ModelProvider{
			{Provider: "p1", Priority: 1},
			{Provider: "p2", Priority: 2},
		},
	}

	prov, provID, err := selector.Select(model, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provID != "p2" {
		t.Errorf("session affinity should select p2, got %s", provID)
	}
	if prov == nil {
		t.Fatal("expected non-nil provider")
	}
}

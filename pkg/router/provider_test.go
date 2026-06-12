package router

import (
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/provider"
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

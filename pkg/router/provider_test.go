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

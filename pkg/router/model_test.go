package router

import (
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
)

func TestModelResolverResolveExactMatch(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]config.Model{
			"gpt-4": {
				DisplayName: "GPT-4",
				Aliases:     []string{"gpt4"},
			},
		},
	}
	resolver := NewModelResolver(cfg)

	model, canonicalName, ok := resolver.Resolve("gpt-4")
	if !ok {
		t.Fatal("expected exact match to succeed")
	}
	if canonicalName != "gpt-4" {
		t.Errorf("expected canonical name 'gpt-4', got %q", canonicalName)
	}
	if model.DisplayName != "GPT-4" {
		t.Errorf("expected DisplayName 'GPT-4', got %q", model.DisplayName)
	}
}

func TestModelResolverResolveAliasMatch(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]config.Model{
			"gpt-4": {
				DisplayName: "GPT-4",
				Aliases:     []string{"gpt4", "gpt-4-turbo"},
			},
		},
	}
	resolver := NewModelResolver(cfg)

	model, canonicalName, ok := resolver.Resolve("gpt4")
	if !ok {
		t.Fatal("expected alias match to succeed")
	}
	if canonicalName != "gpt-4" {
		t.Errorf("expected canonical name 'gpt-4', got %q", canonicalName)
	}
	if model.DisplayName != "GPT-4" {
		t.Errorf("expected DisplayName 'GPT-4', got %q", model.DisplayName)
	}
}

func TestModelResolverResolveNoMatch(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]config.Model{
			"gpt-4": {
				DisplayName: "GPT-4",
				Aliases:     []string{"gpt4"},
			},
		},
	}
	resolver := NewModelResolver(cfg)

	_, _, ok := resolver.Resolve("unknown-model")
	if ok {
		t.Fatal("expected no match for unknown model, but got ok=true")
	}
}

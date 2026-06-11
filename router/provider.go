package router

import (
	"fmt"
	"math/rand"

	"github.com/tianyuansun/ai-gateway/config"
	"github.com/tianyuansun/ai-gateway/session"
)

type ProviderSelector struct {
	cfg           *config.Config
	sessionStore  session.Store
	healthStatus  map[string]bool
}

func NewProviderSelector(cfg *config.Config, store session.Store) *ProviderSelector {
	return &ProviderSelector{
		cfg:          cfg,
		sessionStore: store,
		healthStatus: make(map[string]bool),
	}
}

func (s *ProviderSelector) Select(model *config.Model, sessionID string) (*config.Provider, string, error) {
	if sessionID != "" {
		if sess, _ := s.sessionStore.Get(sessionID); sess != nil {
			if prov, ok := s.cfg.Providers[sess.ProviderID]; ok {
				return &prov, sess.ProviderID, nil
			}
		}
	}

	strategy := "priority"
	if model.Routing != nil && model.Routing.Strategy != "" {
		strategy = model.Routing.Strategy
	}

	if len(model.Providers) == 0 {
		return nil, "", fmt.Errorf("no providers configured for model")
	}

	switch strategy {
	case "weighted":
		return s.selectWeighted(model)
	default:
		return s.selectPriority(model)
	}
}

func (s *ProviderSelector) selectPriority(model *config.Model) (*config.Provider, string, error) {
	for _, mp := range model.Providers {
		if prov, ok := s.cfg.Providers[mp.Provider]; ok {
			if s.isHealthy(mp.Provider) {
				return &prov, mp.Provider, nil
			}
		}
	}
	return nil, "", fmt.Errorf("no healthy provider available")
}

func (s *ProviderSelector) selectWeighted(model *config.Model) (*config.Provider, string, error) {
	type candidate struct {
		provider *config.Provider
		id       string
		weight   int
	}
	var candidates []candidate
	for _, mp := range model.Providers {
		if prov, ok := s.cfg.Providers[mp.Provider]; ok {
			if s.isHealthy(mp.Provider) {
				weight := mp.Priority
				if weight <= 0 {
					weight = 1
				}
				candidates = append(candidates, candidate{&prov, mp.Provider, weight})
			}
		}
	}
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("no healthy provider available")
	}

	totalWeight := 0
	for _, c := range candidates {
		totalWeight += c.weight
	}
	r := rand.Intn(totalWeight)
	for _, c := range candidates {
		r -= c.weight
		if r < 0 {
			return c.provider, c.id, nil
		}
	}
	return candidates[0].provider, candidates[0].id, nil
}

func (s *ProviderSelector) isHealthy(providerID string) bool {
	if status, ok := s.healthStatus[providerID]; ok {
		return status
	}
	return true
}

func (s *ProviderSelector) SetHealth(providerID string, healthy bool) {
	s.healthStatus[providerID] = healthy
}

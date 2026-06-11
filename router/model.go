package router

import "github.com/tianyuansun/ai-gateway/config"

type ModelResolver struct {
	cfg *config.Config
}

func NewModelResolver(cfg *config.Config) *ModelResolver {
	return &ModelResolver{cfg: cfg}
}

func (r *ModelResolver) Resolve(modelName string) (*config.Model, string, bool) {
	return r.cfg.ResolveModel(modelName)
}

package service

import (
	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/infra"
)

type Service struct {
	// ponytail: keep infra concrete, single implementation; add interfaces when tests need isolation.
	config *config.Config
	infra  *infra.Infra
}

func New(cfg *config.Config, infra *infra.Infra) *Service {
	return &Service{config: cfg, infra: infra}
}

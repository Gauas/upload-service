package service

import (
	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/infra"
)

type Service struct {
	Config *config.Config
	Infra  *infra.Infra
}

func New(cfg *config.Config, infra *infra.Infra) *Service {
	return &Service{Config: cfg, Infra: infra}
}

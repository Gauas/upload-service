package service

import (
	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/infra"
)

type Service struct {
	cfg   config.Config
	infra *infra.Infra
}

func New(cfg config.Config, i *infra.Infra) *Service {
	return &Service{cfg: cfg, infra: i}
}

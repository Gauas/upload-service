package controller

import (
	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/infra"
	"github.com/gauas/upload-service/provider"
)

type Controller struct {
	Infrastructure *infra.Infra
	Config         *config.Config
	Provider       *provider.Provider
}

func NewController(cfg *config.Config, infraInstance *infra.Infra) *Controller {
	provide := provider.InitProvider(cfg)
	return &Controller{
		Infrastructure: infraInstance,
		Config:         cfg,
		Provider:       provide,
	}
}

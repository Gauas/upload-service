package main

import (
	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/controller"
	"github.com/gauas/upload-service/infra"
	"github.com/gauas/upload-service/kernel"
	"github.com/gauas/upload-service/middlewares"
	"github.com/gauas/upload-service/service"
)

func main() {
	cfg := config.New()

	infraInstance := infra.New(cfg)

	serviceInstance := service.New(cfg, infraInstance)

	controllerInstance := controller.New(serviceInstance)

	middlewareInstance := middlewares.New(cfg)

	kernel.New(controllerInstance, middlewareInstance, cfg).Start()
}

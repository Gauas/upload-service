package main

import (
	"log"

	"github.com/gauas/upload-service/bootstrap"
	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/controller"
	"github.com/gauas/upload-service/grpc"
	"github.com/gauas/upload-service/http"
	"github.com/gauas/upload-service/infra"
	"github.com/gauas/upload-service/middlewares"
	"github.com/gauas/upload-service/service"
)

func main() {
	cfg := config.New()

	infraInstance := infra.New(cfg)
	svc := service.New(cfg, infraInstance)
	ctrl := controller.New(svc)
	mw := middlewares.New(cfg)

	httpServer := http.Register(ctrl, mw, cfg)
	grpcServer := grpc.Register(cfg.GRPCPort)

	if err := bootstrap.Start(httpServer, grpcServer); err != nil {
		log.Fatal(err)
	}
}

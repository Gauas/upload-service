package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/gauas/upload-service/app"
	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/controller"
	"github.com/gauas/upload-service/grpc"
	"github.com/gauas/upload-service/http"
	"github.com/gauas/upload-service/infra"
	"github.com/gauas/upload-service/middlewares"
)

func main() {
	err := godotenv.Load("/gau_upload/upload.env")
	if err != nil {
		log.Println("No .env file found, continuing with environment variables")
	}

	cfgValue := config.New()
	cfg := &cfgValue
	infraInstance := infra.InitInfra(cfg)
	ctrl := controller.NewController(cfg, infraInstance)
	mw := middlewares.New(cfg)

	httpServer := http.Register(cfg, ctrl, mw)
	grpcServer := grpc.Register(cfg.GRPCPort)

	// Append consumerServer here when the consumer runs in this process.
	if err := app.Start(httpServer, grpcServer); err != nil {
		log.Fatal(err)
	}
}

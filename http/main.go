package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/controller"
	"github.com/gauas/upload-service/middlewares"
)

type Server struct {
	port       string
	controller *controller.Controller
	middleware *middlewares.Middleware
}

func Register(cfg *config.Config, ctrl *controller.Controller, mw *middlewares.Middleware) *Server {
	return &Server{
		port:       cfg.Port,
		controller: ctrl,
		middleware: mw,
	}
}

func (s *Server) Start(ctx context.Context) error {
	router := gin.Default()

	apiRoutes := router.Group("/api/v2/upload")
	{
		apiRoutes.Use(s.middleware.PrivateMiddleware)
		apiRoutes.POST("/file", s.controller.UploadFile)
		apiRoutes.GET("/file", s.controller.GetFile)
		apiRoutes.DELETE("/file", s.controller.DeleteFile)
		apiRoutes.GET("/files/list", s.controller.ListFiles)
	}
	apiRoutes.GET("/health", s.controller.CheckHealth)

	httpServer := &http.Server{
		Addr:    ":" + s.port,
		Handler: router,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
			log.Printf("upload-service http shutdown error: %v", err)
		}
	}()

	log.Printf("upload-service http listening on :%s", s.port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}

	return nil
}

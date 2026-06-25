package http

import (
	"context"
	"errors"
	"fmt"
	"log"
	nethttp "net/http"
	"time"

	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/controller"
	"github.com/gauas/upload-service/middlewares"
	"github.com/gauas/upload-service/packages/httpresp"
	"github.com/gauas/upload-service/route"
	"github.com/labstack/echo/v4"
)

type Server struct {
	controller *controller.Controller
	middleware *middlewares.Middleware
	config     config.Config
}

func Register(ctrl *controller.Controller, mw *middlewares.Middleware, cfg *config.Config) *Server {
	return &Server{controller: ctrl, middleware: mw, config: *cfg}
}

func (s *Server) Start(ctx context.Context) error {
	server := echo.New()
	server.HideBanner = true
	server.HTTPErrorHandler = func(err error, c echo.Context) {
		var e *httpresp.Error
		if errors.As(err, &e) {
			_ = c.JSON(e.Code, httpresp.Response{Status: e.Code, Error: e.Message})
			return
		}

		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			code := httpErr.Code
			_ = c.JSON(code, httpresp.Response{Status: code, Error: fmt.Sprintf("%v", httpErr.Message)})
			return
		}

		_ = c.JSON(nethttp.StatusInternalServerError, httpresp.Response{Status: nethttp.StatusInternalServerError, Error: "internal server error"})
	}

	s.middleware.RegisterGlobal(server)
	route.New(server, s.controller, s.middleware.Internal()).RegisterRoutes()

	addr := fmt.Sprintf(":%s", s.config.Port)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			log.Printf("upload-service http shutdown error: %v", err)
		}
	}()

	log.Printf("upload-service http listening on %s", addr)
	if err := server.Start(addr); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}

	return nil
}

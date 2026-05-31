package kernel

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/controller"
	"github.com/gauas/upload-service/middlewares"
	response "github.com/gauas/upload-service/packages/httpresp"
	"github.com/gauas/upload-service/route"
)

type Kernel struct {
	controller *controller.Controller
	middleware *middlewares.Middleware
	config     config.Config
}

func New(ctrl *controller.Controller, mw *middlewares.Middleware, cfg config.Config) *Kernel {
	return &Kernel{controller: ctrl, middleware: mw, config: cfg}
}

func (k *Kernel) Start() {
	server := echo.New()
	server.HideBanner = true
	server.HTTPErrorHandler = errorHandler

	k.middleware.RegisterGlobal(server)

	route.New(server, k.controller, k.middleware.Internal()).RegisterRoutes()

	addr := fmt.Sprintf(":%s", k.config.Port)
	log.Printf("upload-service listening on %s", addr)

	if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func errorHandler(err error, c echo.Context) {
	var e *response.Error
	if errors.As(err, &e) {
		_ = c.JSON(e.Code, response.Response{Status: e.Code, Error: e.Message})
		return
	}

	if he, ok := err.(*echo.HTTPError); ok {
		_ = c.JSON(he.Code, response.Response{Status: he.Code, Error: fmt.Sprintf("%v", he.Message)})
		return
	}

	_ = c.JSON(500, response.Response{Status: 500, Error: "internal server error"})
}



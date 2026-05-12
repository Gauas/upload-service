package route

import (
	"github.com/labstack/echo/v4"
	"github.com/gauas/upload-service/controller"
)

type Router struct {
	server     *echo.Echo
	controller *controller.Controller
	internal   echo.MiddlewareFunc
}

func New(server *echo.Echo, ctrl *controller.Controller, internal echo.MiddlewareFunc) *Router {
	return &Router{server: server, controller: ctrl, internal: internal}
}

func (r *Router) RegisterRoutes() {
	r.server.GET("/v1/upload/health", r.controller.Health)

	api := r.server.Group("/v1/upload", r.internal)
	{
		api.POST("", r.controller.UploadFile)
		api.GET("", r.controller.GetFile)
		api.DELETE("", r.controller.DeleteFile)
		api.GET("/list", r.controller.ListFiles)
	}
}

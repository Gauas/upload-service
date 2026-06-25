package middlewares

import (
	"crypto/subtle"
	"net/http"

	"github.com/gauas/upload-service/config"
	response "github.com/gauas/upload-service/packages/httpresp"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

var MuteLog = []string{"/v1/upload/health"}

type Middleware struct {
	Config *config.Config
}

func New(cfg *config.Config) *Middleware {
	return &Middleware{Config: cfg}
}

func (m *Middleware) RegisterGlobal(e *echo.Echo) {
	e.Use(echoMiddleware.Recover())
	e.Use(echoMiddleware.LoggerWithConfig(echoMiddleware.LoggerConfig{
		Skipper: func(c echo.Context) bool {
			for _, path := range MuteLog {
				if c.Path() == path {
					return true
				}
			}

			return false
		},
	}))
	e.Use(echoMiddleware.RequestID())
}

func (m *Middleware) Internal() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := c.Request().Header.Get("Secret-Key")
			if subtle.ConstantTimeCompare([]byte(key), []byte(m.Config.SecretKey)) != 1 {
				return response.NewError(http.StatusUnauthorized, "unauthorized")
			}

			return next(c)
		}
	}
}

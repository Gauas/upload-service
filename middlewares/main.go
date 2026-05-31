package middlewares

import (
	"crypto/subtle"
	"net/http"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/gauas/upload-service/config"
	response "github.com/gauas/upload-service/packages/httpresp"
)

type Middleware struct {
	secretKey string
}

func New(cfg config.Config) *Middleware {
	return &Middleware{secretKey: cfg.SecretKey}
}

func (m *Middleware) RegisterGlobal(e *echo.Echo) {
	e.Use(echoMiddleware.Recover())
	e.Use(echoMiddleware.Logger())
	e.Use(echoMiddleware.RequestID())
}

func (m *Middleware) Internal() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := c.Request().Header.Get("Secret-Key")
			if subtle.ConstantTimeCompare([]byte(key), []byte(m.secretKey)) != 1 {
				return response.NewError(http.StatusUnauthorized, "unauthorized")
			}

			return next(c)
		}
	}
}



package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/gauas/upload-service/config"
)

type Middleware struct {
	PrivateMiddleware gin.HandlerFunc
}

func New(cfg *config.Config) *Middleware {
	return &Middleware{
		PrivateMiddleware: PrivateMiddleware(cfg),
	}
}

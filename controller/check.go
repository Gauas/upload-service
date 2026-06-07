package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/gauas/upload-service/utils"
)

func (ctrl *Controller) CheckHealth(c *gin.Context) {
	utils.JSON200(c, gin.H{"status": "running"})
}

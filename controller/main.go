package controller

import "github.com/gauas/upload-service/service"

type Controller struct {
	Service *service.Service
}

func New(svc *service.Service) *Controller {
	return &Controller{Service: svc}
}

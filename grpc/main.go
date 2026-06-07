package grpc

import "github.com/gauas/upload-service/grpc/runtime"

type Server = runtime.Server

func Register(port string) *Server {
	return runtime.Register(port, "upload-service", func(server runtime.ServiceRegistrar) {})
}

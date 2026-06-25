package runtime

import (
	"context"
	"fmt"
	"log"
	"net"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type ServiceRegistrar = grpcpkg.ServiceRegistrar

type RegisterFunc func(ServiceRegistrar)

type Server struct {
	port        string
	serviceName string
	register    RegisterFunc
}

func Register(port, serviceName string, register RegisterFunc) *Server {
	return &Server{port: port, serviceName: serviceName, register: register}
}

func (s *Server) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
	if err != nil {
		return err
	}

	server := grpcpkg.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(s.serviceName, healthpb.HealthCheckResponse_SERVING)

	healthpb.RegisterHealthServer(server, healthServer)
	s.register(server)
	reflection.Register(server)

	go func() {
		<-ctx.Done()
		log.Printf("%s grpc shutting down", s.serviceName)
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		healthServer.SetServingStatus(s.serviceName, healthpb.HealthCheckResponse_NOT_SERVING)
		server.GracefulStop()
	}()

	log.Printf("%s grpc listening on :%s", s.serviceName, s.port)
	if err := server.Serve(lis); err != nil && ctx.Err() == nil {
		return err
	}

	return nil
}

package main

import (
	"flag"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"

	"github.com/alesr/audiostripper/api"
	apiv1 "github.com/alesr/audiostripper/api/proto/audiostripper/v1"
	"github.com/alesr/audiostripper/internal/app/audiostripper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	grpcPort string = ":50051"
	certPath string = "/etc/ssl/mycerts/cert.pem"
	keyPath  string = "/etc/ssl/mycerts/key.pem"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))

	var useSSL bool
	flag.BoolVar(&useSSL, "ssl", false, "Use SSL for the gRPC server")
	flag.Parse()

	var serverOpts []grpc.ServerOption

	if useSSL {
		creds, err := credentials.NewServerTLSFromFile(certPath, keyPath)
		if err != nil {
			log.Fatalf("Failed to load SSL certificates: %v", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
	}

	grpcServer := grpc.NewServer(serverOpts...)

	grpcServer.RegisterService(
		&apiv1.AudioStripper_ServiceDesc,
		api.NewGRPCServer(logger, audiostripper.NewService(logger)),
	)

	logger.Info("Starting gRPC server", "grpc_port", grpcPort)

	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		logger.Error("Could not listen", "grpc_port", grpcPort, "error", err)
		os.Exit(2)
	}

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed to serve gRPC server", "error", err)
			os.Exit(3)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer signal.Stop(c)

	<-c

	logger.Info("Shutting down gRPC server")
	grpcServer.GracefulStop()
}

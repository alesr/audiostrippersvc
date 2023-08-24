package main

import (
	"log/slog"
	"net"
	"os"
	"os/signal"

	"github.com/alesr/audiostripper/api"
	apiv1 "github.com/alesr/audiostripper/api/proto/audiostripper/v1"
	"github.com/alesr/audiostripper/internal/app/audiostripper"
	"google.golang.org/grpc"
)

const grpcPort string = ":50051"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))

	grpcServer := grpc.NewServer()

	grpcServer.RegisterService(
		&apiv1.AudioStripper_ServiceDesc,
		api.NewGRPCServer(logger, audiostripper.NewService(logger)),
	)

	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		logger.Error("Could not listen", "grpc_port", grpcPort, "error", err)
		os.Exit(1)
	}

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed to serve gRPC server", "error", err)
			os.Exit(2)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer signal.Stop(c)

	<-c

	logger.Info("Shutting down gRPC server")
	grpcServer.GracefulStop()
}

package main

import (
	"flag"
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

var version string

func main() {
	logger := makeLogger()

	logger.Info("Running Audiostripper")

	var useSSL bool
	flag.BoolVar(&useSSL, "ssl", false, "Use SSL for the gRPC server")
	flag.Parse()

	var serverOpts []grpc.ServerOption

	if useSSL {
		creds, err := credentials.NewServerTLSFromFile(certPath, keyPath)
		if err != nil {
			logger.Error("Could not create credentials", slog.String("error", err.Error()))
			os.Exit(1)
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
	}

	grpcServer := grpc.NewServer(serverOpts...)

	grpcServer.RegisterService(
		&apiv1.AudioStripper_ServiceDesc,
		api.NewGRPCServer(logger, audiostripper.NewService(logger)),
	)

	logger.Info("Starting gRPC server")

	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		logger.Error("Could not listen", slog.String("error", err.Error()))
		os.Exit(2)
	}

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed to serve gRPC server", slog.String("error", err.Error()))
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

func makeLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}).WithAttrs(func() []slog.Attr {
		var attributes = []slog.Attr{
			{
				Key:   "grpc_port",
				Value: slog.StringValue(grpcPort),
			},
		}

		if version == "" {
			return attributes
		}

		attributes = append(attributes, slog.Attr{Key: "version", Value: slog.StringValue(version)})
		return attributes
	}()))
}

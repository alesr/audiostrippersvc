package api

import (
	"context"
	"io"
	"os"

	"log/slog"

	"github.com/alesr/audiostripper"
	apiv1 "github.com/alesr/audiostrippersvc/api/proto/audiostrippersvc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	MaxInMemorySize = 5 << 20 // 5MB memory threshold
	chunkSize       = 5 << 20 // 5MB chunk for sending data back to client
)

type audioStripperService interface {
	ExtractAudio(ctx context.Context, in *audiostripper.ExtractAudioInput) (*audiostripper.ExtractAudioOutput, error)
}

type GRPCServer struct {
	apiv1.UnimplementedAudioStripperServer
	logger  *slog.Logger
	service audioStripperService
}

func NewGRPCServer(logger *slog.Logger, service audioStripperService) *GRPCServer {
	return &GRPCServer{
		logger:  logger,
		service: service,
	}
}

func (s *GRPCServer) Register(server *grpc.Server) {
	apiv1.RegisterAudioStripperServer(server, s)
	s.logger.Info("Registered GRPCServer to gRPC server")
}

func (s *GRPCServer) ExtractAudio(stream apiv1.AudioStripper_ExtractAudioServer) error {
	var sampleRate string

	// Create a temp file for the incoming data
	tempFile, err := os.CreateTemp("", "input-*")
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create temp file: %v", err)
	}

	// Loop to receive streamed data and write to temp file
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "failed to receive data: %v", err)
		}

		// Capture sample rate from the first chunk
		if sampleRate == "" {
			sampleRate = chunk.SampleRate
		}

		if _, err = tempFile.Write(chunk.Data); err != nil {
			return status.Errorf(codes.Internal, "failed to write to temp file: %v", err)
		}
	}

	if err := tempFile.Close(); err != nil {
		return status.Errorf(codes.Internal, "failed to close temp file: %v", err)
	}

	// Call the service to extract audio
	output, err := s.service.ExtractAudio(
		stream.Context(),
		&audiostripper.ExtractAudioInput{
			SampleRate: sampleRate,
			FilePath:   tempFile.Name(),
		},
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to extract audio: %v", err)
	}

	outputFile, err := os.Open(output.FilePath)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to open output file: %v", err)
	}
	defer outputFile.Close()

	buffer := make([]byte, chunkSize)

	for {
		bytesRead, err := outputFile.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to read from output file: %s", err)
		}

		// Send the chunk to the client
		if err := stream.Send(&apiv1.AudioData{Data: buffer[:bytesRead]}); err != nil {
			return status.Errorf(codes.Internal, "failed to send chunk to client: %s", err)
		}
	}

	if err := os.Remove(output.FilePath); err != nil {
		s.logger.Error("Failed to remove temp output file", slog.String("file", output.FilePath), slog.String("error", err.Error()))
	}
	return nil
}

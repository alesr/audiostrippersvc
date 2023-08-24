package api

import (
	"bytes"
	"context"
	"io"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"log/slog"

	apiv1 "github.com/alesr/audiostripper/api/proto/audiostripper/v1"
	"github.com/alesr/audiostripper/internal/app/audiostripper"
	"google.golang.org/grpc"
)

const (
	MaxInMemorySize = 5 << 20 // 5MB memory threshold, adjusted as per feedback
	chunkSize       = 2 << 20 // 2MB chunk for sending data back to client
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
	var (
		videoBuffer bytes.Buffer
		tmpFile     *os.File
		err         error
	)

	// Receiving video data from the client in chunks.
	for {
		videoData, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			s.logger.Error("Error receiving video data:", err)
			return status.Errorf(codes.Internal, "error receiving video data: %v", err)
		}

		// Check if accumulated data exceeds the 5MB threshold.
		if videoBuffer.Len()+len(videoData.Data) > MaxInMemorySize {
			// If buffer size exceeds, we save the data to a temporary file.
			if tmpFile == nil {
				tmpFile, err = os.CreateTemp("", "video-data-*.tmp")
				if err != nil {
					s.logger.Error("Error creating temporary file:", err)
					return status.Errorf(codes.Internal, "error creating temporary file: %v", err)
				}
			}

			// Write buffer content to the temporary file.
			if _, err := tmpFile.Write(videoBuffer.Bytes()); err != nil {
				s.logger.Error("Error writing to temporary file:", err)
				return status.Errorf(codes.Internal, "error writing to temporary file: %v", err)
			}

			// Reset the buffer for next chunks of data.
			videoBuffer.Reset()
		}

		// Accumulate video data in the buffer.
		videoBuffer.Write(videoData.Data)
	}

	// Handle data after receiving all chunks.
	var finalData []byte
	if tmpFile != nil {
		// If data was spilled to a temp file, read the file content.
		tmpFile.Write(videoBuffer.Bytes()) // Write remaining buffer content to file.
		videoBuffer.Reset()

		// Read the complete data from the file.
		finalData, err = os.ReadFile(tmpFile.Name())
		if err != nil {
			s.logger.Error("Error reading from temporary file", "error", err)
			return status.Errorf(codes.Internal, "error reading from temporary file: %s", err)
		}

		// Clean up the temporary file.
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()
	} else {
		// If data was not spilled to file, just use buffer content.
		finalData = videoBuffer.Bytes()
	}

	extractAudioInput := audiostripper.ExtractAudioInput{
		Data: finalData,
	}

	extractAudioOutput, err := s.service.ExtractAudio(stream.Context(), &extractAudioInput)
	if err != nil {
		s.logger.Error("Error extracting audio:", err)
		return status.Errorf(codes.Internal, "error extracting audio: %s", err)
	}

	// Send extracted audio data back to the client in chunks.
	for start := 0; start < len(extractAudioOutput.Data); start += chunkSize {
		end := start + chunkSize
		if end > len(extractAudioOutput.Data) {
			end = len(extractAudioOutput.Data)
		}

		resp := apiv1.AudioData{
			Data: extractAudioOutput.Data[start:end],
		}

		if err := stream.Send(&resp); err != nil {
			s.logger.Error("Error sending audio data:", err)
			return status.Errorf(codes.Internal, "error sending audio data: %v", err)
		}
	}

	return nil
}

package audiostripper

import (
	"context"
	"log/slog"
)

type Service struct {
	logger *slog.Logger
}

func NewService(logger *slog.Logger) *Service {
	return &Service{
		logger: logger,
	}
}

type (
	ExtractAudioInput struct {
		Data []byte
	}

	ExtractAudioOutput struct {
		Data []byte
	}
)

func (s *Service) ExtractAudio(ctx context.Context, in *ExtractAudioInput) (*ExtractAudioOutput, error) {
	s.logger.Info("Extracting audio", "input", in)
	return &ExtractAudioOutput{
		Data: in.Data,
	}, nil
}

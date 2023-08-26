package audiostripper

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
)

type (
	// ExtractAudioInput defines the input for the ExtractAudio method.
	ExtractAudioInput struct {
		SampleRate string
		FilePath   string
	}

	// ExtractAudioOutput defines the output for the ExtractAudio method
	ExtractAudioOutput struct {
		FilePath string
	}

	ExtractCmdParams struct {
		InputFile, OutputFile, SampleRate string
		Stderr                            *bytes.Buffer
	}

	// ExtractCmd is a function that runs the extractor command.
	ExtractCmd func(params *ExtractCmdParams) error

	// Service provides methods for extracting audio from a video file.
	Service struct {
		logger *slog.Logger
		cmd    ExtractCmd
	}
)

// New creates a new Service.
func New(logger *slog.Logger, cmd ExtractCmd) *Service {
	return &Service{
		logger: logger,
		cmd:    cmd,
	}
}

// ExtractAudio extracts audio from a video file.
func (s *Service) ExtractAudio(ctx context.Context, in *ExtractAudioInput) (*ExtractAudioOutput, error) {
	cmdParams := ExtractCmdParams{
		InputFile:  in.FilePath,
		OutputFile: outputFilePath(in.FilePath),
		SampleRate: in.SampleRate,
		Stderr:     &bytes.Buffer{},
	}

	s.logger.Debug("Running extractor command", "params", cmdParams)

	if err := s.cmd(&cmdParams); err != nil {
		s.logger.Error("Could not run extractor command",
			slog.String("error", err.Error()),
			slog.String("stderr", cmdParams.Stderr.String()),
		)
		return nil, fmt.Errorf("could not run extractor command: %s", err)
	}

	return &ExtractAudioOutput{
		FilePath: cmdParams.OutputFile,
	}, nil
}

func outputFilePath(in string) string {
	return in[:len(in)-4] + ".wav"
}

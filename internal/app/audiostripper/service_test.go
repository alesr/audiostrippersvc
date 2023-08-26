package audiostripper

import (
	"context"
	"testing"

	"github.com/alesr/audiostripper/pkg/slognoop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractAudio(t *testing.T) {
	var (
		wasCalled  bool
		givenInput = ExtractAudioInput{
			FilePath:   "test.mp4",
			SampleRate: "44000",
		}

		expectedOutputFile = "test.wav"
	)

	cmdMock := func(params *ExtractCmdParams) error {
		wasCalled = true

		assert.Equal(t, givenInput.FilePath, params.InputFile)
		assert.Equal(t, givenInput.SampleRate, params.SampleRate)
		assert.Equal(t, expectedOutputFile, params.OutputFile)
		assert.NotNil(t, params.Stderr)

		return nil
	}

	service := New(slognoop.NoopLogger(), cmdMock)

	got, err := service.ExtractAudio(context.TODO(), &givenInput)
	require.NoError(t, err)

	assert.Equal(t, expectedOutputFile, got.FilePath)
	assert.True(t, wasCalled)
}

func TestOutputFilePath(t *testing.T) {
	given := "/tmp/test.mp4"
	want := "/tmp/test.wav"

	got := outputFilePath(given)

	assert.Equal(t, want, got)
}

package api

import (
	"context"
	"io"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	apiv1 "github.com/alesr/audiostripper/api/proto/audiostripper/v1"
	"github.com/alesr/audiostripper/internal/app/audiostripper"
	"github.com/alesr/audiostripper/pkg/slognoop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

var _ audioStripperService = &mockAudioStripperService{}

type mockAudioStripperService struct {
	ExtractAudioFunc func(ctx context.Context, in *audiostripper.ExtractAudioInput) (*audiostripper.ExtractAudioOutput, error)
}

func (m *mockAudioStripperService) ExtractAudio(ctx context.Context, in *audiostripper.ExtractAudioInput) (*audiostripper.ExtractAudioOutput, error) {
	return m.ExtractAudioFunc(ctx, in)
}

func TestExtractAudio(t *testing.T) {
	mockService := mockAudioStripperService{}

	mockService.ExtractAudioFunc = func(ctx context.Context, in *audiostripper.ExtractAudioInput) (*audiostripper.ExtractAudioOutput, error) {
		require.NotEmpty(t, in.FilePath)
		require.Equal(t, "44100", in.SampleRate)

		// Read the input file
		inputFile, err := os.Open(in.FilePath)
		require.NoError(t, err)

		inputFileData, err := io.ReadAll(inputFile)
		require.NoError(t, err)

		// The input file data should be the video data sent by the client
		require.Equal(t, []byte("videoDataChunk1videoDataChunk2"), inputFileData)

		// At this point ffmpeg will extract the audio from the video data and save it to the temp file.
		// Let's write some random data to the temp file instead,

		outputFile, err := os.Create(in.FilePath)
		require.NoError(t, err)

		_, err = outputFile.Write([]byte("someRandomAudioData"))
		require.NoError(t, err)

		require.NoError(t, outputFile.Close())

		return &audiostripper.ExtractAudioOutput{
			FilePath: inputFile.Name(),
		}, nil
	}

	// Arrange the grpc server and client
	server, lis := makeGRPCServerHelper(t, &mockService)
	defer server.Stop()

	client := makeGRPCClientHelper(t, lis)

	// Client sends video data to the server

	videoDataStream, err := client.ExtractAudio(context.TODO())
	require.NoError(t, err)

	err = videoDataStream.Send(&apiv1.VideoData{
		SampleRate: "44100", // We only care about sample rate once
		Data:       []byte("videoDataChunk1"),
	})
	require.NoError(t, err)

	// Send more video data
	err = videoDataStream.Send(&apiv1.VideoData{Data: []byte("videoDataChunk2")})
	require.NoError(t, err)

	err = videoDataStream.CloseSend()
	require.NoError(t, err)

	// Server receives audio data in chunks and assert

	var allReceivedData = make([]byte, 0)

	for {
		receivedData, err := videoDataStream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		allReceivedData = append(allReceivedData, receivedData.Data...)
	}

	// The allReceivedData is the result of the ffmpeg process.69
	require.Equal(t, []byte("someRandomAudioData"), allReceivedData)
}

const bufSize int = 512 * 1024 // 512 KB should be enough for our tests

func makeGRPCServerHelper(t *testing.T, service *mockAudioStripperService) (*grpc.Server, *bufconn.Listener) {
	t.Helper()

	s := grpc.NewServer()

	apiv1.RegisterAudioStripperServer(s, NewGRPCServer(slognoop.NoopLogger(), service))

	serverErrCh := make(chan error, 1)
	serverStartedCh := make(chan struct{}, 1)

	lis := bufconn.Listen(bufSize)

	go func() {
		close(serverStartedCh)
		if err := s.Serve(lis); err != nil {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	<-serverStartedCh

	select {
	case err, ok := <-serverErrCh:
		if ok {
			t.Fatalf("Server exited with error: %s", err)
		}
	default:
	}
	return s, lis
}

func makeGRPCClientHelper(t *testing.T, lis *bufconn.Listener) apiv1.AudioStripperClient {
	t.Helper()

	bufDialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.DialContext(context.TODO(), "bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	client := apiv1.NewAudioStripperClient(conn)

	return client
}

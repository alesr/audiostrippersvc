package api

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	apiv1 "github.com/alesr/audiostripper/api/proto/audiostripper/v1"
	"github.com/alesr/audiostripper/internal/app/audiostripper"
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
	expectedData := []byte("testAudioData")

	mockService := mockAudioStripperService{}

	mockService.ExtractAudioFunc = func(ctx context.Context, in *audiostripper.ExtractAudioInput) (*audiostripper.ExtractAudioOutput, error) {
		return &audiostripper.ExtractAudioOutput{
			Data: expectedData,
		}, nil
	}

	server, lis := makeGRPCServerHelper(t, &mockService)
	defer server.Stop()

	client := makeGRPCClientHelper(t, lis)

	videoDataStream, err := client.ExtractAudio(context.TODO())
	require.NoError(t, err)

	err = videoDataStream.Send(&apiv1.VideoData{Data: []byte("testVideoData")})
	require.NoError(t, err)

	err = videoDataStream.CloseSend()
	require.NoError(t, err)

	receivedData, err := videoDataStream.Recv()
	require.NoError(t, err)

	require.Equal(t, expectedData, receivedData.Data)
}

const bufSize int = 512 * 1024 // 512 KB should be enough for our tests

func makeGRPCServerHelper(t *testing.T, service *mockAudioStripperService) (*grpc.Server, *bufconn.Listener) {
	t.Helper()

	server := grpc.NewServer()
	apiv1.RegisterAudioStripperServer(server, NewGRPCServer(noopLogger(), service))

	s := grpc.NewServer()
	apiv1.RegisterAudioStripperServer(s, NewGRPCServer(noopLogger(), service))

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

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

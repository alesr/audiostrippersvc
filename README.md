# audiostripper

A Go gRPC application for extracting audio from videos using FFMPEG

## Overview

AudioStripper exposes a gRPC Bi-directional stream API and uses [FFMPEG](https://ffmpeg.org/) for extracting audio from videos.

## Architecture

### Core Components

```bash
.
├── LICENSE
├── README.md
├── Taskfile.yaml
├── api
│   ├── grpcserver.go # Implement gRPC bi-directional stream API fro extracting audio from videos
│   ├── grpcserver_test.go
│   └── proto
│       └── audiostripper
│           └── v1
│               ├── audiostripper.pb.go
│               ├── audiostripper.proto # API spec
│               └── audiostripper_grpc.pb.go
├── cmd
│   └── audiostripper
│       └── main.go # Application entrypoint. Starts the server on port :50051
├── go.mod
├── go.sum
├── internal
│   └── app
│       └── audiostripper
│           ├── service.go # Implements domain logic
│           └── service_test.go
└── pkg
    └── slognoop
        └── slognoop.go

```

## Usage Example

```go

func main() {
	secure := flag.Bool("secure", false, "Use secure connection")
	flag.Parse()

	var conn *grpc.ClientConn
	var err error

	if *secure {
		creds, err := credentials.NewClientTLSFromFile(certPath, "")
		if err != nil {
			log.Fatalf("Failed to load credentials: %v", err)
		}

		conn, err = grpc.Dial(prodHost, grpc.WithTransportCredentials(creds))
		if err != nil {
			log.Fatalf("Failed to dial server: %v", err)
		}
	} else {
		conn, err = grpc.Dial(localHost, grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		))
		if err != nil {
			log.Fatalf("Failed to dial server: %v", err)
		}
	}

	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}

	// Initialize gRPC client
	client := pb.NewAudioStripperClient(conn)

	// Open & read data from test video

	f, err := os.Open(videoFilepath)
	if err != nil {
		log.Fatalf("Failed to open test video: %v", err)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("Failed to read test video: %v", err)
	}
	defer f.Close()

	// Initialize stream and send data

	stream, err := client.ExtractAudio(context.TODO())
	if err != nil {
		log.Fatalf("Error while calling ExtractAudio: %v", err)
	}

	for i := 0; i < 10; i++ {
		if err := stream.Send(&pb.VideoData{
			Data:       data,
			SampleRate: "44100", // The server consumes the sample request only once.
		}); err != nil {
			log.Fatalf("Failed to send data: %v", err)
		}
	}

	// Close the client stream

	if err := stream.CloseSend(); err != nil {
		log.Fatalf("Failed to close stream: %v", err)
	}

	// Read from the server-side stream

	var audioData []byte

	for {
		chunkData, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalf("Failed to receive: %v", err)
		}

		audioData = append(audioData, chunkData.Data...)
	}

	// Write received audio data to wav file

	wavFile, err := os.Create("test_video.wav")
	if err != nil {
		log.Fatalf("Failed to create wav file: %v", err)
	}
	defer wavFile.Close()

	if _, err := wavFile.Write(audioData); err != nil {
		log.Fatalf("Failed to write wav file: %v", err)
	}
}
```

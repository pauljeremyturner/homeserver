package main

import (
	"context"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	cryptopb "homeserver/gen/crypto"
	newspb "homeserver/gen/news"
	weatherpb "homeserver/gen/weather"
)

const streamReconnectDelay = 5 * time.Second

type weatherMsg *weatherpb.WeatherUpdate
type newsMsg *newspb.NewsUpdate
type cryptoMsg *cryptopb.CryptoUpdate

func dialInfoServer(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// streamLoop subscribes to a server-streaming RPC and calls onMsg for every
// message received, reconnecting with a fixed delay if the connection drops
// or fails (the server always resends the latest known value immediately on
// a fresh subscribe, so a reconnect just means a brief gap, not stale data).
// The three generated *ServiceClient.Stream* methods all return the same
// underlying grpc.ServerStreamingClient[T] shape (aliased per-service by
// protoc-gen-go-grpc), so one generic helper covers weather, news, and crypto.
func streamLoop[T any](ctx context.Context, name string, connect func(context.Context) (grpc.ServerStreamingClient[T], error), onMsg func(*T)) {
	for {
		stream, err := connect(ctx)
		if err != nil {
			log.Printf("%s stream: connect failed: %v", name, err)
			if !sleepOrDone(ctx, streamReconnectDelay) {
				return
			}
			continue
		}
		for {
			msg, err := stream.Recv()
			if err != nil {
				log.Printf("%s stream: recv failed: %v", name, err)
				break
			}
			onMsg(msg)
		}
		if !sleepOrDone(ctx, streamReconnectDelay) {
			return
		}
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

// startStreams launches all three streaming subscriptions for the process
// lifetime. tea.Program.Send is safe to call at any point after
// tea.NewProgram, so these goroutines run independently of the bubbletea
// Update loop's own command lifecycle.
func startStreams(ctx context.Context, conn *grpc.ClientConn, p *tea.Program) {
	weatherClient := weatherpb.NewWeatherServiceClient(conn)
	newsClient := newspb.NewNewsServiceClient(conn)
	cryptoClient := cryptopb.NewCryptoServiceClient(conn)

	go streamLoop(ctx, "weather",
		func(ctx context.Context) (grpc.ServerStreamingClient[weatherpb.WeatherUpdate], error) {
			return weatherClient.StreamWeather(ctx, &weatherpb.StreamWeatherRequest{})
		},
		func(u *weatherpb.WeatherUpdate) { p.Send(weatherMsg(u)) },
	)

	go streamLoop(ctx, "news",
		func(ctx context.Context) (grpc.ServerStreamingClient[newspb.NewsUpdate], error) {
			return newsClient.StreamNews(ctx, &newspb.StreamNewsRequest{})
		},
		func(u *newspb.NewsUpdate) { p.Send(newsMsg(u)) },
	)

	go streamLoop(ctx, "crypto",
		func(ctx context.Context) (grpc.ServerStreamingClient[cryptopb.CryptoUpdate], error) {
			return cryptoClient.StreamCrypto(ctx, &cryptopb.StreamCryptoRequest{})
		},
		func(u *cryptopb.CryptoUpdate) { p.Send(cryptoMsg(u)) },
	)
}

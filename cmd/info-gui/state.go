package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	cryptopb "homeserver/gen/crypto"
	newspb "homeserver/gen/news"
	weatherpb "homeserver/gen/weather"
)

const streamReconnectDelay = 5 * time.Second

// state is everything the UI draws apart from the clock. The gRPC streams and
// the board temperature poller write it from their own goroutines; the
// window loop reads a snapshot per frame.
type state struct {
	mu        sync.Mutex
	weather   *weatherpb.WeatherUpdate
	news      *newspb.NewsUpdate
	crypto    map[string]*cryptopb.CryptoUpdate
	wallet    *cryptopb.WalletBalanceUpdate
	boardTemp float64
	hasTemp   bool
}

type snapshot struct {
	weather   *weatherpb.WeatherUpdate
	news      *newspb.NewsUpdate
	btc, eth  *cryptopb.CryptoUpdate
	wallet    *cryptopb.WalletBalanceUpdate
	boardTemp float64
	hasTemp   bool
}

func newState() *state {
	return &state{crypto: make(map[string]*cryptopb.CryptoUpdate)}
}

func (s *state) update(f func(*state)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(s)
}

func (s *state) snapshot() snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return snapshot{
		weather:   s.weather,
		news:      s.news,
		btc:       s.crypto["BTC"],
		eth:       s.crypto["ETH"],
		wallet:    s.wallet,
		boardTemp: s.boardTemp,
		hasTemp:   s.hasTemp,
	}
}

// streamLoop subscribes to a server-streaming RPC and calls onMsg for every
// message, reconnecting after a fixed delay if the stream fails (the server
// resends its latest value on subscribe, so a reconnect is just a gap).
func streamLoop[T any](ctx context.Context, name string, connect func(context.Context) (grpc.ServerStreamingClient[T], error), onMsg func(*T)) {
	for {
		stream, err := connect(ctx)
		if err == nil {
			for {
				msg, err := stream.Recv()
				if err != nil {
					log.Printf("%s stream: recv failed: %v", name, err)
					break
				}
				onMsg(msg)
			}
		} else {
			log.Printf("%s stream: connect failed: %v", name, err)
		}
		select {
		case <-time.After(streamReconnectDelay):
		case <-ctx.Done():
			return
		}
	}
}

// startStreams subscribes to every info-server stream; changed() is called
// after each update so the window can redraw.
func startStreams(ctx context.Context, addr string, s *state, changed func()) error {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	weatherClient := weatherpb.NewWeatherServiceClient(conn)
	newsClient := newspb.NewNewsServiceClient(conn)
	cryptoClient := cryptopb.NewCryptoServiceClient(conn)

	go streamLoop(ctx, "weather",
		func(ctx context.Context) (grpc.ServerStreamingClient[weatherpb.WeatherUpdate], error) {
			return weatherClient.StreamWeather(ctx, &weatherpb.StreamWeatherRequest{})
		},
		func(u *weatherpb.WeatherUpdate) { s.update(func(s *state) { s.weather = u }); changed() },
	)
	go streamLoop(ctx, "news",
		func(ctx context.Context) (grpc.ServerStreamingClient[newspb.NewsUpdate], error) {
			return newsClient.StreamNews(ctx, &newspb.StreamNewsRequest{})
		},
		func(u *newspb.NewsUpdate) { s.update(func(s *state) { s.news = u }); changed() },
	)
	go streamLoop(ctx, "crypto",
		func(ctx context.Context) (grpc.ServerStreamingClient[cryptopb.CryptoUpdate], error) {
			return cryptoClient.StreamCrypto(ctx, &cryptopb.StreamCryptoRequest{})
		},
		func(u *cryptopb.CryptoUpdate) { s.update(func(s *state) { s.crypto[u.Symbol] = u }); changed() },
	)
	go streamLoop(ctx, "wallet",
		func(ctx context.Context) (grpc.ServerStreamingClient[cryptopb.WalletBalanceUpdate], error) {
			return cryptoClient.StreamWalletBalance(ctx, &cryptopb.StreamWalletBalanceRequest{})
		},
		func(u *cryptopb.WalletBalanceUpdate) { s.update(func(s *state) { s.wallet = u }); changed() },
	)
	return nil
}

// thermalZonePaths are tried in order; the first readable one wins.
// thermal_zone0 is cpu-thermal on the Tinker Board (RK3288).
var thermalZonePaths = []string{
	"/sys/class/thermal/thermal_zone0/temp",
	"/sys/class/thermal/thermal_zone1/temp",
}

func readBoardTempC() (float64, bool) {
	for _, p := range thermalZonePaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		milli, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			continue
		}
		return float64(milli) / 1000, true
	}
	return 0, false
}

// pollBoardTemp reads the board's own temperature every 10s — local data,
// not something the server knows about.
func pollBoardTemp(ctx context.Context, s *state, changed func()) {
	for {
		t, ok := readBoardTempC()
		s.update(func(s *state) { s.boardTemp, s.hasTemp = t, ok })
		changed()
		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

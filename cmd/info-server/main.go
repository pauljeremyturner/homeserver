package main

import (
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	"homeserver/internal/broadcast"

	cryptopb "homeserver/gen/crypto"
	newspb "homeserver/gen/news"
	weatherpb "homeserver/gen/weather"
)

const (
	weatherInterval = 15 * time.Minute
	newsInterval    = 15 * time.Minute
	cryptoInterval  = 30 * time.Minute
)

type coinConfig struct {
	id     string // CoinGecko coin id
	symbol string // display symbol
}

var trackedCoins = []coinConfig{
	{"bitcoin", "BTC"},
	{"ethereum", "ETH"},
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	addr := envOr("LISTEN_ADDR", ":9090")
	walletAddr := os.Getenv("WALLET_BTC_ADDRESS")
	walletLabel := envOr("WALLET_BTC_LABEL", "BTC Wallet")

	weatherBC := broadcast.New[*weatherpb.WeatherUpdate]()
	newsBC := broadcast.New[*newspb.NewsUpdate]()
	cryptoBCs := make(map[string]*broadcast.Broadcaster[*cryptopb.CryptoUpdate], len(trackedCoins))
	for _, c := range trackedCoins {
		cryptoBCs[c.symbol] = broadcast.New[*cryptopb.CryptoUpdate]()
	}
	walletBC := broadcast.New[*cryptopb.WalletBalanceUpdate]()

	go pollWeather(weatherBC)
	go pollNews(newsBC)
	for _, c := range trackedCoins {
		go pollCrypto(c, cryptoBCs[c.symbol])
	}
	if walletAddr != "" {
		go pollWallet(walletAddr, walletLabel, walletBC)
	} else {
		log.Print("WALLET_BTC_ADDRESS not set, wallet balance stream will stay empty")
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", addr, err)
	}

	srv := grpc.NewServer()
	weatherpb.RegisterWeatherServiceServer(srv, &weatherServer{bc: weatherBC})
	newspb.RegisterNewsServiceServer(srv, &newsServer{bc: newsBC})
	cryptopb.RegisterCryptoServiceServer(srv, &cryptoServer{bcs: cryptoBCs, walletBC: walletBC})

	log.Printf("info-server listening on %s", addr)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func pollWeather(bc *broadcast.Broadcaster[*weatherpb.WeatherUpdate]) {
	poll := func() {
		update, err := fetchWeather()
		if err != nil {
			log.Printf("weather: fetch failed: %v", err)
			return
		}
		bc.Publish(update)
	}
	poll()
	for range time.Tick(weatherInterval) {
		poll()
	}
}

func pollNews(bc *broadcast.Broadcaster[*newspb.NewsUpdate]) {
	poll := func() {
		update, err := fetchNews()
		if err != nil {
			log.Printf("news: fetch failed: %v", err)
			return
		}
		bc.Publish(update)
	}
	poll()
	for range time.Tick(newsInterval) {
		poll()
	}
}

func pollCrypto(c coinConfig, bc *broadcast.Broadcaster[*cryptopb.CryptoUpdate]) {
	poll := func() {
		update, err := fetchCryptoWeek(c.id, c.symbol)
		if err != nil {
			log.Printf("crypto %s: fetch failed: %v", c.symbol, err)
			return
		}
		bc.Publish(update)
	}
	poll()
	for range time.Tick(cryptoInterval) {
		poll()
	}
}

type weatherServer struct {
	weatherpb.UnimplementedWeatherServiceServer
	bc *broadcast.Broadcaster[*weatherpb.WeatherUpdate]
}

func (s *weatherServer) StreamWeather(_ *weatherpb.StreamWeatherRequest, stream weatherpb.WeatherService_StreamWeatherServer) error {
	ch := s.bc.Subscribe()
	defer s.bc.Unsubscribe(ch)
	for {
		select {
		case v := <-ch:
			if err := stream.Send(v); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

type newsServer struct {
	newspb.UnimplementedNewsServiceServer
	bc *broadcast.Broadcaster[*newspb.NewsUpdate]
}

func (s *newsServer) StreamNews(_ *newspb.StreamNewsRequest, stream newspb.NewsService_StreamNewsServer) error {
	ch := s.bc.Subscribe()
	defer s.bc.Unsubscribe(ch)
	for {
		select {
		case v := <-ch:
			if err := stream.Send(v); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

func pollWallet(address, label string, bc *broadcast.Broadcaster[*cryptopb.WalletBalanceUpdate]) {
	poll := func() {
		update, err := fetchWalletBalance(address, label)
		if err != nil {
			log.Printf("wallet: fetch failed: %v", err)
			return
		}
		bc.Publish(update)
	}
	poll()
	for range time.Tick(cryptoInterval) {
		poll()
	}
}

type cryptoServer struct {
	cryptopb.UnimplementedCryptoServiceServer
	bcs      map[string]*broadcast.Broadcaster[*cryptopb.CryptoUpdate]
	walletBC *broadcast.Broadcaster[*cryptopb.WalletBalanceUpdate]
}

// StreamWalletBalance streams the configured wallet's balance. If no wallet
// was configured (WALLET_BTC_ADDRESS unset), the underlying broadcaster never
// publishes, so this simply never sends anything — not an error.
func (s *cryptoServer) StreamWalletBalance(_ *cryptopb.StreamWalletBalanceRequest, stream cryptopb.CryptoService_StreamWalletBalanceServer) error {
	ch := s.walletBC.Subscribe()
	defer s.walletBC.Unsubscribe(ch)
	for {
		select {
		case v := <-ch:
			if err := stream.Send(v); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

// StreamCrypto fans in every tracked coin's broadcaster into one output
// stream, so a client gets BTC and ETH (and anything else tracked) over a
// single subscription, each last-known value delivered immediately on connect.
func (s *cryptoServer) StreamCrypto(_ *cryptopb.StreamCryptoRequest, stream cryptopb.CryptoService_StreamCryptoServer) error {
	out := make(chan *cryptopb.CryptoUpdate)
	done := make(chan struct{})
	defer close(done)

	for _, bc := range s.bcs {
		ch := bc.Subscribe()
		defer bc.Unsubscribe(ch)
		go func(ch chan *cryptopb.CryptoUpdate) {
			for {
				select {
				case v := <-ch:
					select {
					case out <- v:
					case <-done:
						return
					}
				case <-done:
					return
				}
			}
		}(ch)
	}

	for {
		select {
		case v := <-out:
			if err := stream.Send(v); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

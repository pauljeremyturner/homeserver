package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"homeserver/internal/broadcast"

	cryptopb "homeserver/gen/crypto"
	displaypb "homeserver/gen/display"
	newspb "homeserver/gen/news"
	planetspb "homeserver/gen/planets"
	weatherpb "homeserver/gen/weather"
)

const (
	weatherInterval = 15 * time.Minute
	newsInterval    = 15 * time.Minute
	cryptoInterval  = 30 * time.Minute
	// CoinGecko's keyless API throttles bursts, so its requests go one at a
	// time cryptoGap apart, and a throttled one is retried after
	// cryptoRetry, up to cryptoTries times a round.
	cryptoGap   = 10 * time.Second
	cryptoRetry = 2 * time.Minute
	cryptoTries = 3
)

type coinConfig struct {
	id     string // CoinGecko coin id
	symbol string // display symbol
}

var trackedCoins = []coinConfig{
	{"bitcoin", "BTC"},
	{"ethereum", "ETH"},
	// Gold and silver, per troy ounce: PAX Gold and Kinesis Silver are
	// tokens each backed by an ounce of the metal, so they track spot
	// closely, and CoinGecko has them hourly in GBP like the coins above.
	{"pax-gold", "XAU"},
	{"kinesis-silver", "XAG"},
}

// cryptoWindows are the spans of prices fetched for every tracked coin, in
// days: the week and six months, for the two markets pages.
var cryptoWindows = []int{7, 180}

// version is set at build time (-ldflags "-X main.version=...") and sent to
// clients as "server-version" header metadata on every stream.
var version = "dev"

func versionHeader(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := ss.SetHeader(metadata.Pairs("server-version", version)); err != nil {
		return err
	}
	return handler(srv, ss)
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
	pageSeconds := positiveEnv("PAGE_SECONDS", 60)
	marketsSeconds := positiveEnv("MARKETS_PAGE_SECONDS", 30)
	planetsSeconds := positiveEnv("PLANETS_PAGE_SECONDS", 30)
	// The planets' elements are only good from 1800, and past ~50 years
	// Mercury laps too fast for a display's animation to follow.
	planetsYears, err := strconv.Atoi(envOr("PLANETS_YEARS", "10"))
	if err != nil || planetsYears < 1 || planetsYears > 200 {
		log.Fatalf("PLANETS_YEARS must be a number of years from 1 to 200, got %q", os.Getenv("PLANETS_YEARS"))
	}
	loc, err := parseLocation(os.Getenv("LOCATION"), os.Getenv("LOCATION_NAME"))
	if err != nil {
		log.Fatalf("LOCATION: %v", err)
	}
	if loc != nil {
		log.Printf("weather: location set to %s,%s", loc.lat, loc.lon)
	} else {
		log.Printf("weather: no LOCATION set, locating by IP")
	}

	weatherBC := broadcast.New[*weatherpb.WeatherUpdate]()
	newsBC := broadcast.New[*newspb.NewsUpdate]()
	// cryptoBCs[days][symbol] carries one coin's prices over one window.
	cryptoBCs := make(map[int]map[string]*broadcast.Broadcaster[*cryptopb.CryptoUpdate], len(cryptoWindows))
	for _, days := range cryptoWindows {
		cryptoBCs[days] = make(map[string]*broadcast.Broadcaster[*cryptopb.CryptoUpdate], len(trackedCoins))
		for _, c := range trackedCoins {
			cryptoBCs[days][c.symbol] = broadcast.New[*cryptopb.CryptoUpdate]()
		}
	}
	walletBC := broadcast.New[*cryptopb.WalletBalanceUpdate]()
	planetsBC := broadcast.New[*planetspb.PlanetsUpdate]()
	pageBC := broadcast.New[*displaypb.PageUpdate]()

	go pollWeather(weatherBC, loc)
	go pollNews(newsBC)
	go pollPlanets(planetsBC, planetsYears)
	go cyclePages(pageBC, pageCycle(pageSeconds, marketsSeconds, planetsSeconds))
	go pollCrypto(cryptoBCs)
	if walletAddr != "" {
		go pollWallet(walletAddr, walletLabel, walletBC)
	} else {
		log.Print("WALLET_BTC_ADDRESS not set, wallet balance stream will stay empty")
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", addr, err)
	}

	srv := grpc.NewServer(grpc.StreamInterceptor(versionHeader))
	weatherpb.RegisterWeatherServiceServer(srv, &weatherServer{bc: weatherBC})
	newspb.RegisterNewsServiceServer(srv, &newsServer{bc: newsBC})
	cryptopb.RegisterCryptoServiceServer(srv, &cryptoServer{bcs: cryptoBCs, walletBC: walletBC})
	planetspb.RegisterPlanetServiceServer(srv, &planetServer{bc: planetsBC})
	displaypb.RegisterDisplayServiceServer(srv, &displayServer{bc: pageBC})

	log.Printf("info-server %s listening on %s", version, addr)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func pollWeather(bc *broadcast.Broadcaster[*weatherpb.WeatherUpdate], loc *location) {
	poll := func() {
		update, err := fetchWeather(loc)
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

// positiveEnv reads a positive whole number of seconds from the environment.
func positiveEnv(key string, fallback int) int {
	n, err := strconv.Atoi(envOr(key, strconv.Itoa(fallback)))
	if err != nil || n <= 0 {
		log.Fatalf("%s must be a positive number of seconds, got %q", key, os.Getenv(key))
	}
	return n
}

// pollCrypto fetches every window of every tracked coin's prices, the
// week's first, one request at a time (see cryptoGap), every
// cryptoInterval.
func pollCrypto(bcs map[int]map[string]*broadcast.Broadcaster[*cryptopb.CryptoUpdate]) {
	type fetch struct {
		c    coinConfig
		days int
	}
	var all []fetch
	for _, days := range cryptoWindows {
		for _, c := range trackedCoins {
			all = append(all, fetch{c, days})
		}
	}
	for {
		start := time.Now()
		todo := all
		for try := 1; len(todo) > 0; try++ {
			if try > 1 {
				time.Sleep(cryptoRetry)
			}
			var failed []fetch
			for i, f := range todo {
				if i > 0 {
					time.Sleep(cryptoGap)
				}
				update, err := fetchCrypto(f.c.id, f.c.symbol, f.days)
				if err != nil {
					log.Printf("crypto %s %dd: fetch failed (try %d of %d): %v", f.c.symbol, f.days, try, cryptoTries, err)
					failed = append(failed, f)
					continue
				}
				bcs[f.days][f.c.symbol].Publish(update)
			}
			if try == cryptoTries {
				break
			}
			todo = failed
		}
		time.Sleep(time.Until(start.Add(cryptoInterval)))
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
	bcs      map[int]map[string]*broadcast.Broadcaster[*cryptopb.CryptoUpdate]
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

// StreamCrypto fans in every tracked coin's broadcaster for the requested
// window into one output stream, so a client gets BTC and ETH (and anything
// else tracked) over a single subscription, each last-known value delivered
// immediately on connect.
func (s *cryptoServer) StreamCrypto(req *cryptopb.StreamCryptoRequest, stream cryptopb.CryptoService_StreamCryptoServer) error {
	days := int(req.Days)
	if days == 0 {
		days = 7 // older clients don't say
	}
	bcs, ok := s.bcs[days]
	if !ok {
		return status.Errorf(codes.InvalidArgument, "no %d-day prices; ask for one of %v", days, cryptoWindows)
	}
	out := make(chan *cryptopb.CryptoUpdate)
	done := make(chan struct{})
	defer close(done)

	for _, bc := range bcs {
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

// forward sends everything published on bc to a server stream until the
// client goes away.
func forward[T any](bc *broadcast.Broadcaster[T], stream interface {
	Send(T) error
	Context() context.Context
}) error {
	ch := bc.Subscribe()
	defer bc.Unsubscribe(ch)
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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	cryptopb "homeserver/gen/crypto"
)

type coingeckoResponse struct {
	Prices [][2]float64 `json:"prices"`
}

// fetchCryptoWeek fetches 7 days of daily GBP prices for the given CoinGecko
// coin id (e.g. "bitcoin", "ethereum"), labeling the result with symbol
// (e.g. "BTC", "ETH") for display.
func fetchCryptoWeek(coinID, symbol string) (*cryptopb.CryptoUpdate, error) {
	client := http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s/market_chart?vs_currency=gbp&days=7&interval=daily", coinID)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var cg coingeckoResponse
	if err := json.Unmarshal(body, &cg); err != nil {
		return nil, err
	}
	if len(cg.Prices) == 0 {
		return nil, fmt.Errorf("coingecko %s: empty price series", coinID)
	}

	prices := make([]float64, 0, len(cg.Prices))
	for _, p := range cg.Prices {
		prices = append(prices, p[1])
	}

	latest := prices[len(prices)-1]
	first := prices[0]
	changePct := 0.0
	if first != 0 {
		changePct = (latest - first) / first * 100
	}

	min, max := prices[0], prices[0]
	for _, p := range prices {
		if p < min {
			min = p
		}
		if p > max {
			max = p
		}
	}

	return &cryptopb.CryptoUpdate{
		Symbol:        symbol,
		FetchedAtUnix: time.Now().Unix(),
		Prices:        prices,
		Latest:        latest,
		ChangePct:     changePct,
		Min:           min,
		Max:           max,
	}, nil
}

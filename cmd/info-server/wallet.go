package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	cryptopb "homeserver/gen/crypto"
)

type esploraAddressStats struct {
	FundedTxoSum int64 `json:"funded_txo_sum"`
	SpentTxoSum  int64 `json:"spent_txo_sum"`
}

type esploraAddressResponse struct {
	ChainStats esploraAddressStats `json:"chain_stats"`
}

const satsPerBTC = 100_000_000

// fetchWalletBalance fetches the confirmed on-chain balance for a Bitcoin
// address via Blockstream's public Esplora API (no API key required). The
// address itself never ends up in the returned update — only the resulting
// balance and a display label.
func fetchWalletBalance(address, label string) (*cryptopb.WalletBalanceUpdate, error) {
	client := http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("https://blockstream.info/api/address/%s", address)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("blockstream: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var r esploraAddressResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}

	sats := r.ChainStats.FundedTxoSum - r.ChainStats.SpentTxoSum
	return &cryptopb.WalletBalanceUpdate{
		Label:         label,
		FetchedAtUnix: time.Now().Unix(),
		BalanceBtc:    float64(sats) / satsPerBTC,
		BalanceSats:   sats,
	}, nil
}

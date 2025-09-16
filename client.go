package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HyperliquidClient struct {
	config     *Config
	httpClient *http.Client
	baseURL    string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

type Fill struct {
	Coin      string  `json:"coin"`
	Side      string  `json:"side"`
	Size      float64 `json:"sz,string"`
	Price     float64 `json:"px,string"`
	Time      int64   `json:"time"`
	StartPosition float64 `json:"startPosition,string"`
	Dir       string  `json:"dir"`
	ClosedPnl string  `json:"closedPnl"`
	Hash      string  `json:"hash"`
	Oid       int64   `json:"oid"`
	Crossed   bool    `json:"crossed"`
	Fee       string  `json:"fee"`
}

type Order struct {
	Coin  string  `json:"coin"`
	Side  string  `json:"side"`
	Size  float64 `json:"sz"`
	Price float64 `json:"px"`
	Type  string  `json:"orderType"`
}

func NewHyperliquidClient(config *Config) (*HyperliquidClient, error) {
	var baseURL string
	if config.UseTestnet {
		baseURL = "https://api.hyperliquid-testnet.xyz"
	} else {
		baseURL = "https://api.hyperliquid.xyz"
	}

	privateKeyBytes, err := hex.DecodeString(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %v", err)
	}

	privateKey := ed25519.PrivateKey(privateKeyBytes)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return &HyperliquidClient{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		privateKey: privateKey,
		publicKey:  publicKey,
	}, nil
}

func (c *HyperliquidClient) GetUserFills(user string) ([]*Fill, error) {
	payload := map[string]interface{}{
		"type": "userFills",
		"user": user,
	}

	resp, err := c.makeInfoRequest(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to get user fills for %s: %v", user, err)
	}

	var fills []*Fill
	if err := json.Unmarshal(resp, &fills); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fills response: %v", err)
	}

	return fills, nil
}

func (c *HyperliquidClient) PlaceOrder(order *Order) error {
	payload := map[string]interface{}{
		"type": "order",
		"orders": []map[string]interface{}{
			{
				"a":         order.Coin,
				"b":         order.Side == "buy",
				"p":         fmt.Sprintf("%.6f", order.Price),
				"s":         fmt.Sprintf("%.6f", order.Size),
				"r":         false,
				"t":         map[string]string{"limit": "Limit"}[order.Type],
			},
		},
		"grouping": "na",
	}

	_, err := c.makeExchangeRequest(payload)
	return err
}

func (c *HyperliquidClient) makeInfoRequest(payload map[string]interface{}) ([]byte, error) {
	return c.makeRequest("/info", payload, false)
}

func (c *HyperliquidClient) makeExchangeRequest(payload map[string]interface{}) ([]byte, error) {
	return c.makeRequest("/exchange", payload, true)
}

func (c *HyperliquidClient) makeRequest(endpoint string, payload map[string]interface{}, needsAuth bool) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if needsAuth {
		// For now, skip signing - we'll implement this when we test
		// In a real implementation, you'd need to sign the payload with the private key
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (c *HyperliquidClient) Close() {
	// Cleanup if needed
}
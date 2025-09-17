package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// getDataDir returns the data directory path, using config if available or defaults for tests
func getDataDir() string {
	// For tests or when no config is available, use current directory
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/srv"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data/hype-copy-bot"
	}

	// In test environment, use local paths
	if prefix == "/srv" && os.Getenv("PREFIX") == "" {
		// Check if we're likely in a test (current dir writeable)
		if _, err := os.Stat("."); err == nil {
			return "."
		}
	}

	return filepath.Join(prefix, dataDir)
}

// SaveFill appends a fill record to daily fills file
func (pt *PaperTrader) SaveFill(fill *Fill, action string, realizedPnL, unrealizedPnL float64) {
	// Skip storage during tests
	if pt.VolumeThreshold == 0.0 {
		return
	}
	record := map[string]interface{}{
		"time":           time.Now().UnixMilli(),
		"coin":           fill.Coin,
		"side":           fill.Side,
		"size":           fill.Size,
		"price":          fill.Price,
		"action":         action,
		"realized_pnl":   realizedPnL,
		"unrealized_pnl": unrealizedPnL,
		"volume_usd":     fill.Size * fill.Price,
	}

	filename := fmt.Sprintf("%s/fills/%s.jl", getDataDir(), time.Now().Format("20060102"))
	appendJSON(filename, record)
}

// SaveAccount appends current account state to daily accounts file
func (pt *PaperTrader) SaveAccount() {
	// Skip storage during tests
	if pt.VolumeThreshold == 0.0 {
		return
	}
	// Note: Caller must already hold pt.mu.Lock()

	positions := make(map[string]map[string]float64)
	totalUnrealized := 0.0

	for coin, pos := range pt.Positions {
		if pos.Size == 0 {
			continue // Skip flat positions
		}

		unrealized := pt.calculateUnrealizedPnL(pos)
		totalUnrealized += unrealized

		positions[coin] = map[string]float64{
			"size":       pos.Size,
			"avg_price":  pos.AvgEntryPrice,
			"last_price": pos.LastPrice,
			"realized":   pos.RealizedPnL,
			"unrealized": unrealized,
			"market_val": pos.Size * pos.LastPrice,
		}
	}

	record := map[string]interface{}{
		"time":         time.Now().UnixMilli(),
		"total_pnl":    pt.TotalRealizedPnL + totalUnrealized,
		"realized_pnl": pt.TotalRealizedPnL,
		"positions":    positions,
		"num_trades":   pt.TotalTrades,
	}

	filename := fmt.Sprintf("%s/accounts/%s.jl", getDataDir(), time.Now().Format("20060102"))
	appendJSON(filename, record)
}

// appendJSON appends a JSON record to a file (creates dirs if needed)
func appendJSON(filename string, data interface{}) {
	// Create directory if needed
	dir := filepath.Dir(filename)
	os.MkdirAll(dir, 0755)

	// Marshal to JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return
	}

	// Append to file
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	file.WriteString(string(jsonBytes) + "\n")
}

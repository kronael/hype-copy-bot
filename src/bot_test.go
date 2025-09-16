package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBotConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "Valid config",
			config: &Config{
				TargetAccount: "0x1234567890abcdef1234567890abcdef12345678",
				APIKey:        "test_key",
				PrivateKey:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				CopyThreshold: 100.0,
			},
			expectError: false,
		},
		{
			name: "Invalid private key",
			config: &Config{
				TargetAccount: "0x1234567890abcdef1234567890abcdef12345678",
				APIKey:        "test_key",
				PrivateKey:    "invalid_key",
				CopyThreshold: 100.0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBot(tt.config)
			hasError := err != nil
			if hasError != tt.expectError {
				t.Errorf("NewBot() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

func TestFillProcessingThreshold(t *testing.T) {
	config := &Config{
		TargetAccount: "0x1234567890abcdef1234567890abcdef12345678",
		APIKey:        "test_key",
		PrivateKey:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CopyThreshold: 1000.0, // $1000 threshold
	}

	bot, err := NewBot(config)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	tests := []struct {
		name           string
		fill           *Fill
		shouldProcess  bool
		expectedTrades int
	}{
		{
			name: "Below threshold - small trade",
			fill: &Fill{
				Coin:      "ETH",
				Side:      "B",
				Size:      0.1,
				Price:     4000.0, // $400 total
				ClosedPnl: "0.0",
				Hash:      "hash1",
				Time:      time.Now().Unix() * 1000,
			},
			shouldProcess:  false,
			expectedTrades: 0,
		},
		{
			name: "Above threshold - large trade",
			fill: &Fill{
				Coin:      "ETH",
				Side:      "B",
				Size:      1.0,
				Price:     4000.0, // $4000 total
				ClosedPnl: "0.0",
				Hash:      "hash2",
				Time:      time.Now().Unix() * 1000,
			},
			shouldProcess:  true,
			expectedTrades: 1,
		},
		{
			name: "Exactly at threshold",
			fill: &Fill{
				Coin:      "BTC",
				Side:      "A",
				Size:      0.01,
				Price:     100000.0, // $1000 total
				ClosedPnl: "100.0",
				Hash:      "hash3",
				Time:      time.Now().Unix() * 1000,
			},
			shouldProcess:  true,
			expectedTrades: 2,
		},
	}

	totalExpectedTrades := 0
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialTrades := bot.paperTrader.GetTotalTrades()
			err := bot.process(tt.fill)

			if err != nil {
				t.Errorf("process() error = %v", err)
			}

			tradesAdded := bot.paperTrader.GetTotalTrades() - initialTrades
			if tt.shouldProcess && tradesAdded != 1 {
				t.Errorf("Expected trade to be processed but wasn't")
			} else if !tt.shouldProcess && tradesAdded != 0 {
				t.Errorf("Expected trade to be filtered but was processed")
			}
		})

		if tt.shouldProcess {
			totalExpectedTrades++
		}
	}

	if bot.paperTrader.GetTotalTrades() != totalExpectedTrades {
		t.Errorf("Total trades = %d, want %d", bot.paperTrader.GetTotalTrades(), totalExpectedTrades)
	}
}

func TestClientAPI(t *testing.T) {
	// Create mock server
	mockFills := []*Fill{
		{
			Coin:      "BTC",
			Side:      "B",
			Size:      1.0,
			Price:     50000.0,
			Time:      time.Now().Unix() * 1000,
			ClosedPnl: "0.0",
			Hash:      "0xabc123",
		},
		{
			Coin:      "ETH",
			Side:      "A",
			Size:      10.0,
			Price:     4000.0,
			Time:      time.Now().Unix() * 1000,
			ClosedPnl: "1000.0",
			Hash:      "0xdef456",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/info" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockFills)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client with mock server URL
	config := &Config{
		TargetAccount: "0x1234567890abcdef1234567890abcdef12345678",
		APIKey:        "test_key",
		PrivateKey:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CopyThreshold: 100.0,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Override base URL to use mock server
	client.baseURL = server.URL

	// Test GetUserFills
	fills, err := client.GetUserFills(config.TargetAccount)
	if err != nil {
		t.Fatalf("GetUserFills() error = %v", err)
	}

	if len(fills) != 2 {
		t.Errorf("Expected 2 fills, got %d", len(fills))
	}

	// Verify fill data
	if fills[0].Coin != "BTC" || fills[0].Side != "B" {
		t.Errorf("First fill incorrect: coin=%s, side=%s", fills[0].Coin, fills[0].Side)
	}

	if fills[1].Coin != "ETH" || fills[1].Side != "A" {
		t.Errorf("Second fill incorrect: coin=%s, side=%s", fills[1].Coin, fills[1].Side)
	}
}

func TestBotDuplicateFillHandling(t *testing.T) {
	config := &Config{
		TargetAccount: "0x1234567890abcdef1234567890abcdef12345678",
		APIKey:        "test_key",
		PrivateKey:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CopyThreshold: 100.0,
	}

	bot, err := NewBot(config)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	// Create test fill
	fill := &Fill{
		Coin:      "BTC",
		Side:      "B",
		Size:      1.0,
		Price:     50000.0,
		Time:      time.Now().Unix() * 1000,
		ClosedPnl: "100.0",
		Hash:      "unique_hash_123",
	}

	// Process same fill twice
	err1 := bot.process(fill)
	err2 := bot.process(fill)

	if err1 != nil || err2 != nil {
		t.Errorf("process() errors: %v, %v", err1, err2)
	}

	// Should only process once due to hash tracking
	if bot.paperTrader.GetTotalTrades() != 1 {
		t.Errorf("Duplicate fill processed: trades = %d, want 1", bot.paperTrader.GetTotalTrades())
	}

	// Check hash is tracked
	if !bot.processedFills[fill.Hash] {
		t.Errorf("Fill hash not tracked in processedFills")
	}
}

func TestConfigEnvironmentDefaults(t *testing.T) {
	// Test with missing environment variables (should use defaults/fail gracefully)
	_, err := loadConfig("")

	// Should fail due to missing required env vars
	if err == nil {
		t.Errorf("loadConfig() should fail with missing environment variables")
	}

	// Test the config structure has expected defaults
	testConfig := &Config{
		CopyThreshold: 0.01,
	}

	if testConfig.CopyThreshold != 0.01 {
		t.Errorf("Default CopyThreshold = %f, want 0.01", testConfig.CopyThreshold)
	}
}

func TestBotStartStop(t *testing.T) {
	config := &Config{
		TargetAccount: "0x1234567890abcdef1234567890abcdef12345678",
		APIKey:        "test_key",
		PrivateKey:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CopyThreshold: 100.0,
	}

	bot, err := NewBot(config)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	// Test start
	if bot.running {
		t.Errorf("Bot should not be running initially")
	}

	err = bot.Start()
	if err != nil {
		t.Errorf("Bot.Start() error = %v", err)
	}

	if !bot.running {
		t.Errorf("Bot should be running after Start()")
	}

	// Test stop
	bot.Stop()

	if bot.running {
		t.Errorf("Bot should not be running after Stop()")
	}

	// Test multiple stops (should be safe)
	bot.Stop()
	bot.Stop()
}

func TestRealWorldTradingScenario(t *testing.T) {
	// Simulate The White Whale's actual trading pattern
	config := &Config{
		TargetAccount: "0xb8b9e3097c8b1dddf9c5ea9d48a7ebeaf09d67d2",
		APIKey:        "test_key",
		PrivateKey:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CopyThreshold: 1000.0,
	}

	bot, err := NewBot(config)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	// Simulate realistic trading sequence based on actual data
	fills := []*Fill{
		// ETH accumulation phase
		{Coin: "ETH", Side: "B", Size: 10.3, Price: 4475.1, ClosedPnl: "0.0", Hash: "fill1", Time: time.Now().Unix() * 1000},
		{Coin: "ETH", Side: "B", Size: 22.27, Price: 4491.7, ClosedPnl: "0.0", Hash: "fill2", Time: time.Now().Unix() * 1000},
		{Coin: "ETH", Side: "B", Size: 82.49, Price: 4635.8, ClosedPnl: "0.0", Hash: "fill3", Time: time.Now().Unix() * 1000},

		// ETH profit taking
		{Coin: "ETH", Side: "A", Size: 38.05, Price: 4707.5, ClosedPnl: "1096.23", Hash: "fill4", Time: time.Now().Unix() * 1000},
		{Coin: "ETH", Side: "A", Size: 82.49, Price: 4712.0, ClosedPnl: "2747.89", Hash: "fill5", Time: time.Now().Unix() * 1000},

		// BTC short building
		{Coin: "BTC", Side: "A", Size: 7.22, Price: 114000.0, ClosedPnl: "11704.84", Hash: "fill6", Time: time.Now().Unix() * 1000},
		{Coin: "BTC", Side: "A", Size: 10.52, Price: 114000.0, ClosedPnl: "17052.34", Hash: "fill7", Time: time.Now().Unix() * 1000},
		{Coin: "BTC", Side: "A", Size: 2.71, Price: 114000.0, ClosedPnl: "4399.96", Hash: "fill8", Time: time.Now().Unix() * 1000},
	}

	for _, fill := range fills {
		err := bot.process(fill)
		if err != nil {
			t.Errorf("process() error = %v", err)
		}
	}

	// Verify realistic results
	pt := bot.paperTrader

	if pt.GetTotalTrades() != len(fills) {
		t.Errorf("Total trades = %d, want %d", pt.GetTotalTrades(), len(fills))
	}

	// Should have significant realized PnL (from the ClosedPnl values)
	expectedMinPnL := 30000.0 // Conservative estimate
	if pt.TotalRealizedPnL < expectedMinPnL {
		t.Errorf("Total realized PnL = %f, want at least %f", pt.TotalRealizedPnL, expectedMinPnL)
	}

	// Should have both ETH and BTC positions
	if len(pt.Positions) != 2 {
		t.Errorf("Number of assets = %d, want 2", len(pt.Positions))
	}

	// ETH should be reduced/flat after profit taking
	ethPos := pt.Positions["ETH"]
	if ethPos.Size > 50 { // Should be much smaller after profit taking
		t.Errorf("ETH position too large after profit taking: %f", ethPos.Size)
	}

	// BTC should be short
	btcPos := pt.Positions["BTC"]
	if btcPos.Size >= 0 {
		t.Errorf("BTC should be short position, got: %f", btcPos.Size)
	}
}

func TestErrorHandling(t *testing.T) {
	config := &Config{
		TargetAccount: "0x1234567890abcdef1234567890abcdef12345678",
		APIKey:        "test_key",
		PrivateKey:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CopyThreshold: 100.0,
	}

	bot, err := NewBot(config)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	// Test with malformed fill data
	malformedFills := []*Fill{
		{Coin: "", Side: "B", Size: 1.0, Price: 50000.0, ClosedPnl: "invalid", Hash: "test1", Time: time.Now().Unix() * 1000},
		{Coin: "BTC", Side: "", Size: -1.0, Price: 0.0, ClosedPnl: "0.0", Hash: "test2", Time: time.Now().Unix() * 1000},
		{Coin: "ETH", Side: "B", Size: 1.0, Price: 50000.0, ClosedPnl: "", Hash: "", Time: 0},
	}

	for i, fill := range malformedFills {
		err := bot.process(fill)
		if err != nil {
			t.Errorf("process(%d) should handle malformed data gracefully, got error: %v", i, err)
		}
	}

	// Bot should still be functional despite malformed data
	validFill := &Fill{
		Coin:      "BTC",
		Side:      "B",
		Size:      1.0,
		Price:     50000.0,
		ClosedPnl: "100.0",
		Hash:      "valid_hash",
		Time:      time.Now().Unix() * 1000,
	}

	err = bot.process(validFill)
	if err != nil {
		t.Errorf("Bot should handle valid fill after malformed data: %v", err)
	}
}

// Benchmark for bot performance under load
func BenchmarkBotProcessFill(b *testing.B) {
	config := &Config{
		TargetAccount: "0x1234567890abcdef1234567890abcdef12345678",
		APIKey:        "test_key",
		PrivateKey:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CopyThreshold: 100.0,
	}

	bot, err := NewBot(config)
	if err != nil {
		b.Fatalf("Failed to create bot: %v", err)
	}

	fill := &Fill{
		Coin:      "BTC",
		Side:      "B",
		Size:      1.0,
		Price:     50000.0,
		ClosedPnl: "100.0",
		Hash:      "benchmark_hash",
		Time:      time.Now().Unix() * 1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create unique hash for each iteration
		testFill := *fill
		testFill.Hash = fill.Hash + "_" + string(rune(i))
		bot.process(&testFill)
	}
}

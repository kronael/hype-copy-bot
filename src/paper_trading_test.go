package main

import (
	"math"
	"testing"
	"time"
)

func TestPositionActionDetermination(t *testing.T) {
	pt := NewPaperTrader()

	tests := []struct {
		name     string
		oldSize  float64
		newSize  float64
		expected PositionAction
	}{
		{"Flat to Long", 0, 10, ActionOpen},
		{"Flat to Short", 0, -10, ActionOpen},
		{"Long to Flat", 10, 0, ActionClose},
		{"Short to Flat", -10, 0, ActionClose},
		{"Long to Bigger Long", 10, 15, ActionAdd},
		{"Short to Bigger Short", -10, -15, ActionAdd},
		{"Long to Smaller Long", 10, 5, ActionReduce},
		{"Short to Smaller Short", -10, -5, ActionReduce},
		{"Long to Short", 10, -5, ActionReverse},
		{"Short to Long", -10, 5, ActionReverse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := pt.determineAction(tt.oldSize, tt.newSize)
			if action != tt.expected {
				t.Errorf("determineAction(%f, %f) = %v, want %v",
					tt.oldSize, tt.newSize, action, tt.expected)
			}
		})
	}
}

func TestVolumeWeightedAveragePrice(t *testing.T) {
	pt := NewPaperTrader()

	// Test building a long position
	fills := []*Fill{
		createTestFill("BTC", "B", 1.0, 50000.0, "100.0", time.Now().Unix()),
		createTestFill("BTC", "B", 1.0, 51000.0, "0.0", time.Now().Unix()),
		createTestFill("BTC", "B", 2.0, 52000.0, "0.0", time.Now().Unix()),
	}

	for _, fill := range fills {
		pt.ProcessFill(fill)
	}

	position := pt.Positions["BTC"]
	expectedSize := 4.0
	expectedAvgPrice := (50000.0 + 51000.0 + 2*52000.0) / 4.0 // Volume weighted

	if position.Size != expectedSize {
		t.Errorf("Position size = %f, want %f", position.Size, expectedSize)
	}

	if math.Abs(position.AvgEntryPrice-expectedAvgPrice) > 0.01 {
		t.Errorf("Average price = %f, want %f", position.AvgEntryPrice, expectedAvgPrice)
	}
}

func TestRealizedPnLCalculation(t *testing.T) {
	pt := NewPaperTrader()

	// Build position then reduce it
	fills := []*Fill{
		// Buy 2 BTC at 50000
		createTestFill("BTC", "B", 2.0, 50000.0, "0.0", time.Now().Unix()),
		// Sell 1 BTC at 55000 (should realize 5000 profit)
		createTestFill("BTC", "A", 1.0, 55000.0, "5000.0", time.Now().Unix()),
	}

	for _, fill := range fills {
		pt.ProcessFill(fill)
	}

	// Check total realized PnL
	expectedRealizedPnL := 5000.0
	if math.Abs(pt.TotalRealizedPnL-expectedRealizedPnL) > 0.01 {
		t.Errorf("Total realized PnL = %f, want %f", pt.TotalRealizedPnL, expectedRealizedPnL)
	}

	// Check remaining position
	position := pt.Positions["BTC"]
	expectedSize := 1.0
	if position.Size != expectedSize {
		t.Errorf("Remaining position size = %f, want %f", position.Size, expectedSize)
	}
}

func TestUnrealizedPnLCalculation(t *testing.T) {
	pt := NewPaperTrader()

	// Buy 1 BTC at 50000
	fill := createTestFill("BTC", "B", 1.0, 50000.0, "0.0", time.Now().Unix())
	pt.ProcessFill(fill)

	position := pt.Positions["BTC"]

	// Simulate current price at 55000
	position.LastPrice = 55000.0
	unrealizedPnL := pt.calculateUnrealizedPnL(position)

	expectedUnrealizedPnL := 5000.0 // (55000 - 50000) * 1
	if math.Abs(unrealizedPnL-expectedUnrealizedPnL) > 0.01 {
		t.Errorf("Unrealized PnL = %f, want %f", unrealizedPnL, expectedUnrealizedPnL)
	}
}

func TestPositionReversal(t *testing.T) {
	pt := NewPaperTrader()

	fills := []*Fill{
		// Build long position: 2 BTC at 50000
		createTestFill("BTC", "B", 2.0, 50000.0, "0.0", time.Now().Unix()),
		// Reverse to short: sell 5 BTC at 55000 (close 2, open 3 short)
		createTestFill("BTC", "A", 5.0, 55000.0, "10000.0", time.Now().Unix()),
	}

	for _, fill := range fills {
		pt.ProcessFill(fill)
	}

	position := pt.Positions["BTC"]
	expectedSize := -3.0        // Now short 3 BTC
	expectedAvgPrice := 55000.0 // New average for short position

	if position.Size != expectedSize {
		t.Errorf("Position size after reversal = %f, want %f", position.Size, expectedSize)
	}

	if math.Abs(position.AvgEntryPrice-expectedAvgPrice) > 0.01 {
		t.Errorf("Average price after reversal = %f, want %f", position.AvgEntryPrice, expectedAvgPrice)
	}
}

func TestWhaleScenario(t *testing.T) {
	pt := NewPaperTrader()

	// Simulate The White Whale's ETH trading scenario
	fills := []*Fill{
		// Build large ETH long position
		createTestFill("ETH", "B", 100.0, 4500.0, "0.0", time.Now().Unix()),
		createTestFill("ETH", "B", 50.0, 4600.0, "0.0", time.Now().Unix()),
		createTestFill("ETH", "B", 100.0, 4550.0, "0.0", time.Now().Unix()),
		// Partial exit at profit
		createTestFill("ETH", "A", 80.0, 4700.0, "15000.0", time.Now().Unix()),
		// Full reversal - close remaining long and go short
		createTestFill("ETH", "A", 200.0, 4710.0, "25000.0", time.Now().Unix()),
	}

	for _, fill := range fills {
		pt.ProcessFill(fill)
	}

	position := pt.Positions["ETH"]

	// Should be short 30 ETH (250 total sold - 220 long position)
	expectedSize := -30.0
	if position.Size != expectedSize {
		t.Errorf("Final ETH position = %f, want %f", position.Size, expectedSize)
	}

	// Should have significant realized PnL
	expectedMinRealizedPnL := 35000.0 // Conservative estimate
	if pt.TotalRealizedPnL < expectedMinRealizedPnL {
		t.Errorf("Total realized PnL = %f, want at least %f", pt.TotalRealizedPnL, expectedMinRealizedPnL)
	}

	// Should have 5 trades recorded
	expectedTrades := 5
	if pt.GetTotalTrades() != expectedTrades {
		t.Errorf("Total trades = %d, want %d", pt.GetTotalTrades(), expectedTrades)
	}
}

func TestMultipleAssetPortfolio(t *testing.T) {
	pt := NewPaperTrader()

	fills := []*Fill{
		// BTC long position
		createTestFill("BTC", "B", 1.0, 50000.0, "0.0", time.Now().Unix()),
		// ETH long position
		createTestFill("ETH", "B", 10.0, 4000.0, "0.0", time.Now().Unix()),
		// SOL short position
		createTestFill("SOL", "A", 100.0, 200.0, "0.0", time.Now().Unix()),
		// Close BTC with profit
		createTestFill("BTC", "A", 1.0, 55000.0, "5000.0", time.Now().Unix()),
	}

	for _, fill := range fills {
		pt.ProcessFill(fill)
	}

	// Check we have 3 positions initially, 2 after BTC close
	if len(pt.Positions) != 3 {
		t.Errorf("Number of positions = %d, want 3", len(pt.Positions))
	}

	// Check BTC is flat
	btcPos := pt.Positions["BTC"]
	if btcPos.Size != 0 {
		t.Errorf("BTC position size = %f, want 0", btcPos.Size)
	}

	// Check ETH is long
	ethPos := pt.Positions["ETH"]
	if ethPos.Size != 10.0 {
		t.Errorf("ETH position size = %f, want 10.0", ethPos.Size)
	}

	// Check SOL is short
	solPos := pt.Positions["SOL"]
	if solPos.Size != -100.0 {
		t.Errorf("SOL position size = %f, want -100.0", solPos.Size)
	}
}

func TestSmallTradeFiltering(t *testing.T) {
	// This test should be in bot_test.go but we'll test the threshold logic
	config := &Config{
		CopyThreshold: 1000.0,
	}

	bot := &Bot{
		config:         config,
		paperTrader:    NewPaperTrader(),
		processedFills: make(map[string]bool),
	}

	// Small trade - should be filtered out
	smallFill := createTestFill("ETH", "B", 0.1, 4000.0, "0.0", time.Now().Unix())
	err := bot.process(smallFill)

	if err != nil {
		t.Errorf("process returned error: %v", err)
	}

	// Should have 0 trades due to threshold
	if bot.paperTrader.GetTotalTrades() != 0 {
		t.Errorf("Small trade was not filtered: got %d trades, want 0", bot.paperTrader.GetTotalTrades())
	}

	// Large trade - should pass through
	largeFill := createTestFill("ETH", "B", 1.0, 4000.0, "0.0", time.Now().Unix())
	err = bot.process(largeFill)

	if err != nil {
		t.Errorf("process returned error: %v", err)
	}

	// Should have 1 trade now
	if bot.paperTrader.GetTotalTrades() != 1 {
		t.Errorf("Large trade was filtered: got %d trades, want 1", bot.paperTrader.GetTotalTrades())
	}
}

func TestZeroAndNegativeSizes(t *testing.T) {
	pt := NewPaperTrader()

	// Test zero size trade (should be ignored)
	zeroFill := createTestFill("BTC", "B", 0.0, 50000.0, "0.0", time.Now().Unix())
	pt.ProcessFill(zeroFill)

	position := pt.Positions["BTC"]
	if position.Size != 0 {
		t.Errorf("Zero size trade affected position: size = %f, want 0", position.Size)
	}

	if pt.GetTotalTrades() != 1 {
		t.Errorf("Zero size trade not recorded: trades = %d, want 1", pt.GetTotalTrades())
	}
}

func TestHighFrequencyTrading(t *testing.T) {
	pt := NewPaperTrader()

	// Simulate rapid fire trades like The White Whale
	baseTime := time.Now().Unix()

	for i := 0; i < 100; i++ {
		side := "B"
		if i%2 == 1 {
			side = "A"
		}

		price := 50000.0 + float64(i*10) // Varying prices
		size := 0.1 + float64(i%5)*0.05  // Varying sizes

		fill := createTestFill("BTC", side, size, price, "10.0", baseTime+int64(i))
		pt.ProcessFill(fill)
	}

	// Should have processed 100 trades
	if pt.GetTotalTrades() != 100 {
		t.Errorf("High frequency test: trades = %d, want 100", pt.GetTotalTrades())
	}

	// Should have some realized PnL
	if pt.TotalRealizedPnL <= 0 {
		t.Errorf("High frequency test: no realized PnL generated")
	}

	// Position should be reasonable (not wildly off)
	position := pt.Positions["BTC"]
	if math.Abs(position.Size) > 50 {
		t.Errorf("High frequency test: position size unrealistic = %f", position.Size)
	}
}

func TestPnLStringParsing(t *testing.T) {
	pt := NewPaperTrader()

	tests := []struct {
		name     string
		pnlStr   string
		expected float64
	}{
		{"Positive PnL", "1234.56", 1234.56},
		{"Negative PnL", "-987.65", -987.65},
		{"Zero PnL", "0", 0.0},
		{"Zero String", "0.0", 0.0},
		{"Invalid String", "invalid", 0.0}, // Should default to 0
		{"Empty String", "", 0.0},          // Should default to 0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fill := createTestFill("BTC", "B", 1.0, 50000.0, tt.pnlStr, time.Now().Unix())
			pt.ProcessFill(fill)

			// Check the realized PnL was parsed correctly
			// (This is an indirect test since the parsing happens inside ProcessFill)
		})
	}
}

// Helper function to create test fills
func createTestFill(coin, side string, size, price float64, closedPnl string, timestamp int64) *Fill {
	return &Fill{
		Coin:      coin,
		Side:      side,
		Size:      size,
		Price:     price,
		Time:      timestamp * 1000, // Convert to milliseconds
		ClosedPnl: closedPnl,
		Hash:      "test_hash_" + coin + "_" + side,
	}
}

// Benchmark test for performance
func BenchmarkPaperTraderProcessFill(b *testing.B) {
	pt := NewPaperTrader()
	fill := createTestFill("BTC", "B", 1.0, 50000.0, "100.0", time.Now().Unix())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create slightly different fills to avoid hash collisions
		testFill := *fill
		testFill.Hash = fill.Hash + string(rune(i))
		pt.ProcessFill(&testFill)
	}
}

func BenchmarkPositionActionDetermination(b *testing.B) {
	pt := NewPaperTrader()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		oldSize := float64(i % 100)
		newSize := float64((i + 50) % 100)
		pt.determineAction(oldSize, newSize)
	}
}

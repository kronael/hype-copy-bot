package main

import (
	"math"
	"testing"
	"time"
)

func TestCalculateAvailableCapital(t *testing.T) {
	// Start with $10k bankroll
	pt := &PaperTrader{
		Positions:        make(map[string]*Position),
		StartTime:        time.Now(),
		TradeHistory:     make([]*PaperTrade, 0),
		LastTradeTime:    make(map[string]time.Time),
		PendingFills:     make(map[string][]*Fill),
		PendingVolume:    make(map[string]float64),
		LastVolumeUpdate: make(map[string]time.Time),
		MinTradeInterval: 1 * time.Millisecond,
		VolumeThreshold:  0.0,
		VolumeDecayRate:  0.5,
		Bankroll:         10000.0,
		Leverage:         2.0,
		BaseNotional:     1000.0,
		TotalRealizedPnL: 0.0,
	}

	// Test 1: Initial capital should equal bankroll
	availableCapital := pt.calculateAvailableCapital()
	expectedCapital := 10000.0
	if math.Abs(availableCapital-expectedCapital) > 0.01 {
		t.Errorf("Initial available capital = %.2f, want %.2f", availableCapital, expectedCapital)
	}

	// Test 2: Add some realized PnL
	pt.TotalRealizedPnL = 2000.0
	availableCapital = pt.calculateAvailableCapital()
	expectedCapital = 12000.0 // 10k + 2k realized
	if math.Abs(availableCapital-expectedCapital) > 0.01 {
		t.Errorf("Available capital with realized PnL = %.2f, want %.2f", availableCapital, expectedCapital)
	}

	// Test 3: Add a position with unrealized PnL
	pt.Positions["BTC"] = &Position{
		Coin:           "BTC",
		Size:           1.0,
		AvgEntryPrice:  50000.0,
		LastPrice:      55000.0, // $5k unrealized profit
		TotalCostBasis: 50000.0,
		RealizedPnL:    0.0,
		OpenTime:       time.Now(),
		TradeCount:     1,
	}

	availableCapital = pt.calculateAvailableCapital()
	expectedCapital = 17000.0 // 10k + 2k realized + 5k unrealized
	if math.Abs(availableCapital-expectedCapital) > 0.01 {
		t.Errorf("Available capital with unrealized PnL = %.2f, want %.2f", availableCapital, expectedCapital)
	}

	// Test 4: Negative unrealized PnL
	pt.Positions["BTC"].LastPrice = 45000.0 // $5k unrealized loss
	availableCapital = pt.calculateAvailableCapital()
	expectedCapital = 7000.0 // 10k + 2k realized - 5k unrealized
	if math.Abs(availableCapital-expectedCapital) > 0.01 {
		t.Errorf("Available capital with unrealized loss = %.2f, want %.2f", availableCapital, expectedCapital)
	}
}

func TestCalculateDynamicTradeSize(t *testing.T) {
	// Setup paper trader with $10k bankroll, 2x leverage, $1k base notional
	pt := &PaperTrader{
		Positions:        make(map[string]*Position),
		StartTime:        time.Now(),
		TradeHistory:     make([]*PaperTrade, 0),
		LastTradeTime:    make(map[string]time.Time),
		PendingFills:     make(map[string][]*Fill),
		PendingVolume:    make(map[string]float64),
		LastVolumeUpdate: make(map[string]time.Time),
		MinTradeInterval: 1 * time.Millisecond,
		VolumeThreshold:  0.0,
		VolumeDecayRate:  0.5,
		Bankroll:         10000.0,
		Leverage:         2.0,     // 2x leverage = $20k max exposure
		BaseNotional:     1000.0,  // $1k base trade size
		TotalRealizedPnL: 0.0,
	}

	tests := []struct {
		name             string
		price            float64
		existingPosition *Position
		expectedSize     float64
		description      string
	}{
		{
			name:             "Fresh start - full base notional",
			price:            50000.0,
			existingPosition: nil,
			expectedSize:     0.02, // $1000 / $50000 = 0.02 BTC
			description:      "No existing positions, should use full base notional",
		},
		{
			name:  "Half capital used",
			price: 4000.0,
			existingPosition: &Position{
				Coin:           "BTC",
				Size:           0.2,      // 0.2 BTC
				AvgEntryPrice:  50000.0,  // at $50k
				LastPrice:      50000.0,  // = $10k position value
				TotalCostBasis: 10000.0,
				RealizedPnL:    0.0,
				OpenTime:       time.Now(),
				TradeCount:     1,
			},
			expectedSize: 0.25, // With $10k used, $10k remaining, so full $1k notional / $4k = 0.25 ETH
			description:  "Half capital used, should still get full base notional",
		},
		{
			name:  "Near capacity",
			price: 4000.0,
			existingPosition: &Position{
				Coin:           "BTC",
				Size:           0.38,     // 0.38 BTC
				AvgEntryPrice:  50000.0,  // at $50k
				LastPrice:      50000.0,  // = $19k position value (near $20k max)
				TotalCostBasis: 19000.0,
				RealizedPnL:    0.0,
				OpenTime:       time.Now(),
				TradeCount:     1,
			},
			expectedSize: 0.25, // Only $1k remaining capacity, so $1k / $4k = 0.25 ETH
			description:  "Near capacity, should use remaining capital",
		},
		{
			name:  "Over capacity",
			price: 4000.0,
			existingPosition: &Position{
				Coin:           "BTC",
				Size:           0.4,      // 0.4 BTC
				AvgEntryPrice:  50000.0,  // at $50k
				LastPrice:      50000.0,  // = $20k position value (at max)
				TotalCostBasis: 20000.0,
				RealizedPnL:    0.0,
				OpenTime:       time.Now(),
				TradeCount:     1,
			},
			expectedSize: 0.0, // No remaining capacity
			description:  "At capacity, should return 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset positions
			pt.Positions = make(map[string]*Position)
			pt.TotalRealizedPnL = 0.0

			// Add existing position if specified
			if tt.existingPosition != nil {
				pt.Positions["BTC"] = tt.existingPosition
			}

			// Create test fill
			fill := &Fill{
				Coin:      "ETH",
				Side:      "B",
				Size:      10.0, // This size should be ignored
				Price:     tt.price,
				Time:      time.Now().Unix() * 1000,
				ClosedPnl: "0.0",
				Hash:      "test_hash",
			}

			// Calculate dynamic trade size
			actualSize := pt.calculateDynamicTradeSize(fill)

			if math.Abs(actualSize-tt.expectedSize) > 0.01 {
				t.Errorf("%s: calculateDynamicTradeSize() = %.6f, want %.6f",
					tt.description, actualSize, tt.expectedSize)
			}
		})
	}
}

func TestDynamicSizingWithProfitableTrades(t *testing.T) {
	pt := &PaperTrader{
		Positions:        make(map[string]*Position),
		StartTime:        time.Now(),
		TradeHistory:     make([]*PaperTrade, 0),
		LastTradeTime:    make(map[string]time.Time),
		PendingFills:     make(map[string][]*Fill),
		PendingVolume:    make(map[string]float64),
		LastVolumeUpdate: make(map[string]time.Time),
		MinTradeInterval: 1 * time.Millisecond,
		VolumeThreshold:  0.0,
		VolumeDecayRate:  0.5,
		Bankroll:         10000.0,
		Leverage:         2.0,
		BaseNotional:     1000.0,
		TotalRealizedPnL: 5000.0, // $5k profit
	}

	// Add profitable position
	pt.Positions["BTC"] = &Position{
		Coin:           "BTC",
		Size:           0.2,
		AvgEntryPrice:  50000.0,
		LastPrice:      60000.0, // $2k unrealized profit (0.2 * (60k - 50k))
		TotalCostBasis: 10000.0,
		RealizedPnL:    0.0,
		OpenTime:       time.Now(),
		TradeCount:     1,
	}

	// Available capital should be 10k + 5k realized + 2k unrealized = 17k
	// Max exposure = 17k * 2x = 34k
	// Current exposure = 0.2 * 60k = 12k
	// Remaining capacity = 34k - 12k = 22k
	// Should still get full base notional of $1k

	fill := &Fill{
		Coin:      "ETH",
		Side:      "B",
		Size:      10.0,
		Price:     4000.0,
		Time:      time.Now().Unix() * 1000,
		ClosedPnl: "0.0",
		Hash:      "test_hash_2",
	}

	tradeSize := pt.calculateDynamicTradeSize(fill)
	expectedSize := 0.25 // $1000 / $4000 = 0.25 ETH

	if math.Abs(tradeSize-expectedSize) > 0.01 {
		t.Errorf("Dynamic sizing with profits: got %.6f ETH, want %.6f ETH", tradeSize, expectedSize)
	}
}

func TestDynamicSizingIntegration(t *testing.T) {
	pt := &PaperTrader{
		Positions:        make(map[string]*Position),
		StartTime:        time.Now(),
		TradeHistory:     make([]*PaperTrade, 0),
		LastTradeTime:    make(map[string]time.Time),
		PendingFills:     make(map[string][]*Fill),
		PendingVolume:    make(map[string]float64),
		LastVolumeUpdate: make(map[string]time.Time),
		MinTradeInterval: 1 * time.Millisecond,
		VolumeThreshold:  0.0,
		VolumeDecayRate:  0.5,
		Bankroll:         10000.0,
		Leverage:         2.0,
		BaseNotional:     1000.0,
		TotalRealizedPnL: 0.0,
	}

	// Test sequence of trades that should adapt to changing capital
	fills := []*Fill{
		// First trade: should get full $1k notional
		createTestFill("BTC", "B", 999.0, 50000.0, "0.0", time.Now().Unix()),
		// Second trade: should get $1k notional (plenty of room)
		createTestFill("ETH", "B", 999.0, 4000.0, "0.0", time.Now().Unix()),
		// Third trade: much larger original, but should be sized down
		createTestFill("SOL", "B", 999.0, 200.0, "0.0", time.Now().Unix()),
	}

	tradeSizes := make([]float64, 0)

	for i, fill := range fills {
		// Process the fill
		pt.ProcessFill(fill)

		// Check the resulting trade size
		if len(pt.TradeHistory) > i {
			tradeSizes = append(tradeSizes, pt.TradeHistory[i].Size)
		}
	}

	// Verify we got reasonable trade sizes
	if len(tradeSizes) != len(fills) {
		t.Fatalf("Expected %d trades, got %d", len(fills), len(tradeSizes))
	}

	// First trade should be about 0.02 BTC ($1k / $50k)
	expectedFirstSize := 0.02
	if math.Abs(tradeSizes[0]-expectedFirstSize) > 0.01 {
		t.Errorf("First trade size = %.6f, want around %.6f", tradeSizes[0], expectedFirstSize)
	}

	// All trades should be appropriately sized (not the massive original sizes)
	for i, size := range tradeSizes {
		if size > 100.0 { // No trade should be ridiculously large
			t.Errorf("Trade %d size too large: %.6f", i, size)
		}
		if size <= 0.0 { // All trades should have positive size
			t.Errorf("Trade %d size non-positive: %.6f", i, size)
		}
	}

	// Verify total exposure is within limits
	totalExposure := 0.0
	for _, pos := range pt.Positions {
		if pos.Size != 0 {
			totalExposure += math.Abs(pos.Size * pos.LastPrice)
		}
	}

	maxAllowed := pt.calculateAvailableCapital() * pt.Leverage
	if totalExposure > maxAllowed*1.01 { // Allow 1% tolerance
		t.Errorf("Total exposure %.2f exceeds limit %.2f", totalExposure, maxAllowed)
	}
}

func TestCapitalExhaustionScenario(t *testing.T) {
	pt := &PaperTrader{
		Positions:        make(map[string]*Position),
		StartTime:        time.Now(),
		TradeHistory:     make([]*PaperTrade, 0),
		LastTradeTime:    make(map[string]time.Time),
		PendingFills:     make(map[string][]*Fill),
		PendingVolume:    make(map[string]float64),
		LastVolumeUpdate: make(map[string]time.Time),
		MinTradeInterval: 1 * time.Millisecond,
		VolumeThreshold:  0.0,
		VolumeDecayRate:  0.5,
		Bankroll:         1000.0, // Small bankroll
		Leverage:         1.0,    // No leverage
		BaseNotional:     500.0,  // Half the bankroll
		TotalRealizedPnL: 0.0,
	}

	// First trade should consume most of the capital
	fill1 := createTestFill("BTC", "B", 999.0, 50000.0, "0.0", time.Now().Unix())
	pt.ProcessFill(fill1)

	// Second trade should get much smaller size due to limited remaining capital
	fill2 := createTestFill("ETH", "B", 999.0, 4000.0, "0.0", time.Now().Unix())
	pt.ProcessFill(fill2)

	// Third trade should be rejected due to insufficient capital
	fill3 := createTestFill("SOL", "B", 999.0, 200.0, "0.0", time.Now().Unix())
	initialTrades := pt.GetTotalTrades()
	pt.ProcessFill(fill3)
	finalTrades := pt.GetTotalTrades()

	// Should have processed first two trades
	if initialTrades < 2 {
		t.Errorf("Should have processed at least 2 trades before exhaustion, got %d", initialTrades)
	}

	// Third trade should be rejected
	if finalTrades > initialTrades {
		t.Errorf("Third trade should have been rejected, but trades increased from %d to %d", initialTrades, finalTrades)
	}

	// Verify total exposure is within limits
	totalExposure := 0.0
	for _, pos := range pt.Positions {
		if pos.Size != 0 {
			totalExposure += math.Abs(pos.Size * pos.LastPrice)
		}
	}

	if totalExposure > pt.Bankroll*1.01 { // Allow 1% tolerance
		t.Errorf("Total exposure %.2f exceeds bankroll %.2f", totalExposure, pt.Bankroll)
	}
}

package main

import (
	"testing"
	"time"
)

func TestBankrollLimits(t *testing.T) {
	// Create paper trader with small bankroll and low leverage
	pt := &PaperTrader{
		Positions:        make(map[string]*Position),
		StartTime:        time.Now(),
		TradeHistory:     make([]*PaperTrade, 0),
		LastTradeTime:    make(map[string]time.Time),
		PendingFills:     make(map[string][]*Fill),
		PendingVolume:    make(map[string]float64),
		LastVolumeUpdate: make(map[string]time.Time),
		MinTradeInterval: 1 * time.Millisecond,
		VolumeThreshold:  0.0, // Process immediately
		VolumeDecayRate:  0.5,
		Bankroll:         1000.0, // $1k bankroll
		Leverage:         2.0,    // 2x leverage = $2k max
		BaseNotional:     1000.0, // $1k base trade size
	}

	// Test 1: Position within limits should be accepted
	t.Run("WithinLimits", func(t *testing.T) {
		fill := createTestFill("BTC", "B", 0.02, 50000.0, "0.0", time.Now().Unix())
		pt.ProcessFill(fill)

		pos := pt.Positions["BTC"]
		if pos == nil {
			t.Fatal("Expected position to be created")
		}
		// With dynamic sizing, we expect $1000 / $50000 = 0.02 BTC
		expectedSize := 0.02
		if pos.Size != expectedSize {
			t.Errorf("Expected dynamic size %.6f, got %.6f", expectedSize, pos.Size)
		}
	})

	// Test 2: Dynamic sizing should scale down large trades to fit available capital
	t.Run("DynamicSizingForLargeTradeWithConstrainedCapital", func(t *testing.T) {
		// After first trade, we have $1000 used, $1000 remaining capacity
		// Try to add a position - should be dynamically sized to fit remaining capacity
		bigFill := createTestFill("ETH", "B", 1.0, 5000.0, "0.0", time.Now().Unix())

		initialTradeCount := pt.TotalTrades
		pt.ProcessFill(bigFill)

		// Position should exist but be sized to fit available capital
		pos := pt.Positions["ETH"]
		if pos == nil || pos.Size == 0 {
			t.Errorf("Expected position to be created with dynamic sizing, got %v", pos)
		}

		// Should have created a new trade
		if pt.TotalTrades <= initialTradeCount {
			t.Errorf("Expected new trade to be created, trades: %d", pt.TotalTrades)
		}

		// Position value should be around $500-1000 (based on remaining capacity)
		if pos != nil {
			posValue := pos.Size * pos.LastPrice
			if posValue > 1200 { // Should not exceed reasonable limit
				t.Errorf("Position value too large: $%.2f", posValue)
			}
		}
	})

	// Test 3: Multiple positions respecting total limit
	t.Run("MultiplePositionsWithinLimit", func(t *testing.T) {
		// Clear existing positions
		pt.Positions = make(map[string]*Position)
		pt.TotalTrades = 0

		// Add first position: should be $1000 value (0.02 BTC)
		fill1 := createTestFill("BTC", "B", 999.0, 50000.0, "0.0", time.Now().Unix())
		pt.ProcessFill(fill1)

		// Add second position: should be $1000 value (0.2 ETH)
		fill2 := createTestFill("ETH", "B", 999.0, 5000.0, "0.0", time.Now().Unix())
		pt.ProcessFill(fill2)

		// Both positions should exist with dynamic sizing
		btcPos := pt.Positions["BTC"]
		ethPos := pt.Positions["ETH"]

		if btcPos == nil || btcPos.Size == 0 {
			t.Errorf("BTC position should exist with size > 0, got %v", btcPos)
		}
		if ethPos == nil || ethPos.Size == 0 {
			t.Errorf("ETH position should exist with size > 0, got %v", ethPos)
		}

		// Verify positions are reasonably sized (around $1000 each)
		btcValue := btcPos.Size * btcPos.LastPrice
		ethValue := ethPos.Size * ethPos.LastPrice

		if btcValue < 900 || btcValue > 1100 {
			t.Errorf("BTC position value should be around $1000, got $%.2f", btcValue)
		}
		if ethValue < 900 || ethValue > 1100 {
			t.Errorf("ETH position value should be around $1000, got $%.2f", ethValue)
		}
	})

	// Test 4: Third position exceeding total limit should be rejected
	t.Run("ThirdPositionExceedingTotalLimit", func(t *testing.T) {
		// Try to add third position that would push total over limit
		// Current total: $1000, limit: $2000
		// Try to add $1500 position (total would be $2500 > $2000)
		fill3 := createTestFill("DOGE", "B", 3000, 0.5, "0.0", time.Now().Unix())

		initialTradeCount := pt.TotalTrades
		pt.ProcessFill(fill3)

		// DOGE position should not exist
		dogePos := pt.Positions["DOGE"]
		if dogePos != nil && dogePos.Size != 0 {
			t.Errorf("DOGE position should be rejected, but got size %f", dogePos.Size)
		}

		// Trade count should be unchanged
		if pt.TotalTrades != initialTradeCount {
			t.Errorf("Expected no new trades, trade count changed")
		}
	})
}

func TestLeverageCalculation(t *testing.T) {
	tests := []struct {
		name        string
		bankroll    float64
		leverage    float64
		positionUSD float64
		shouldPass  bool
	}{
		{"1x leverage within limit", 1000, 1.0, 500, true},
		{"1x leverage at limit", 1000, 1.0, 1000, true},
		{"1x leverage over limit", 1000, 1.0, 1500, false},
		{"3x leverage within limit", 1000, 3.0, 2500, true},
		{"3x leverage at limit", 1000, 3.0, 3000, true},
		{"3x leverage over limit", 1000, 3.0, 3500, false},
		{"10x leverage huge position", 1000, 10.0, 9000, true},
		{"10x leverage over limit", 1000, 10.0, 11000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := &PaperTrader{
				Positions:       make(map[string]*Position),
				Bankroll:        tt.bankroll,
				Leverage:        tt.leverage,
				VolumeThreshold: 0.0, // Process immediately
			}

			// Calculate size needed for desired USD position
			price := 50000.0 // $50k per BTC
			size := tt.positionUSD / price

			result := pt.validatePositionSize("BTC", size, price)
			if result != tt.shouldPass {
				t.Errorf("Expected %v, got %v for position $%.0f with %.1fx leverage on $%.0f bankroll",
					tt.shouldPass, result, tt.positionUSD, tt.leverage, tt.bankroll)
			}
		})
	}
}

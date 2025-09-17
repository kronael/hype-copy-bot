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
	}

	// Test 1: Position within limits should be accepted
	t.Run("WithinLimits", func(t *testing.T) {
		fill := createTestFill("BTC", "B", 0.02, 50000.0, "0.0", time.Now().Unix())
		pt.ProcessFill(fill)

		pos := pt.Positions["BTC"]
		if pos == nil {
			t.Fatal("Expected position to be created")
		}
		if pos.Size != 0.02 {
			t.Errorf("Expected size 0.02, got %f", pos.Size)
		}
	})

	// Test 2: Position exceeding limits should be rejected
	t.Run("ExceedingLimits", func(t *testing.T) {
		// Try to add a massive position that would exceed $2k limit
		bigFill := createTestFill("ETH", "B", 1.0, 5000.0, "0.0", time.Now().Unix())
		// This would be $5k position, exceeding $2k limit

		initialTradeCount := pt.TotalTrades
		pt.ProcessFill(bigFill)

		// Position should not exist
		pos := pt.Positions["ETH"]
		if pos != nil && pos.Size != 0 {
			t.Errorf("Expected position to be rejected, but got size %f", pos.Size)
		}

		// No new trade should have been recorded
		if pt.TotalTrades != initialTradeCount {
			t.Errorf("Expected no new trades, but got %d", pt.TotalTrades)
		}
	})

	// Test 3: Multiple positions respecting total limit
	t.Run("MultiplePositionsWithinLimit", func(t *testing.T) {
		// Clear existing positions
		pt.Positions = make(map[string]*Position)
		pt.TotalTrades = 0

		// Add first position: $500 value
		fill1 := createTestFill("BTC", "B", 0.01, 50000.0, "0.0", time.Now().Unix())
		pt.ProcessFill(fill1)

		// Add second position: $500 value (total = $1000 within $2k limit)
		fill2 := createTestFill("ETH", "B", 0.1, 5000.0, "0.0", time.Now().Unix())
		pt.ProcessFill(fill2)

		// Both positions should exist
		btcPos := pt.Positions["BTC"]
		ethPos := pt.Positions["ETH"]

		if btcPos == nil || btcPos.Size != 0.01 {
			t.Errorf("BTC position should exist with size 0.01, got %v", btcPos)
		}
		if ethPos == nil || ethPos.Size != 0.1 {
			t.Errorf("ETH position should exist with size 0.1, got %v", ethPos)
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

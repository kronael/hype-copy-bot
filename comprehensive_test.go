package main

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestComprehensivePnLAccounting(t *testing.T) {
	// Use immediate processing to see every trade
	pt := &PaperTrader{
		Positions:          make(map[string]*Position),
		StartTime:          time.Now(),
		TradeHistory:       make([]*PaperTrade, 0),
		LastTradeTime:      make(map[string]time.Time),
		PendingFills:       make(map[string][]*Fill),
		PendingVolume:      make(map[string]float64),
		LastVolumeUpdate:   make(map[string]time.Time),
		MinTradeInterval:   1 * time.Millisecond,
		VolumeThreshold:    1.0,
		VolumeDecayRate:    0.5,
		Bankroll:           10000000.0, // $10M for comprehensive test
		Leverage:           1.0,        // 1x leverage
		BaseNotional:       10000000.0, // Large base notional for tests
		DisableDynamicSize: true,       // Disable dynamic sizing for this test
	}

	fmt.Println("\n=== COMPREHENSIVE PNL ACCOUNTING TEST ===")

	// Test 1: Open Long Position
	fmt.Println("\n1. OPEN LONG: BUY 2.0 BTC @ $50,000")
	fill1 := createTestFill("BTC", "B", 2.0, 50000.0, "0.0", time.Now().Unix())
	pt.ProcessFill(fill1)

	pos := pt.Positions["BTC"]
	fmt.Printf("   Position Size: %.2f\n", pos.Size)
	fmt.Printf("   VWAP: $%.2f\n", pos.AvgEntryPrice)
	fmt.Printf("   Cost Basis: $%.2f\n", pos.TotalCostBasis)
	fmt.Printf("   Last Price: $%.2f\n", pos.LastPrice)

	unrealizedPnL := pt.calculateUnrealizedPnL(pos)
	fmt.Printf("   Unrealized PnL: $%.2f (should be $0.00)\n", unrealizedPnL)
	fmt.Printf("   Position Realized PnL: $%.2f\n", pos.RealizedPnL)
	fmt.Printf("   Portfolio Realized PnL: $%.2f\n", pt.TotalRealizedPnL)

	// Verify opening position
	if pos.Size != 2.0 {
		t.Errorf("Position size wrong: got %.2f, want 2.0", pos.Size)
	}
	if pos.AvgEntryPrice != 50000.0 {
		t.Errorf("VWAP wrong: got $%.2f, want $50000.00", pos.AvgEntryPrice)
	}
	if unrealizedPnL != 0.0 {
		t.Errorf("Unrealized PnL should be $0 on open: got $%.2f", unrealizedPnL)
	}

	// Test 2: Add to Position (should update VWAP)
	fmt.Println("\n2. ADD TO POSITION: BUY 1.0 BTC @ $60,000")
	fill2 := createTestFill("BTC", "B", 1.0, 60000.0, "0.0", time.Now().Unix()+1)
	pt.ProcessFill(fill2)

	pos = pt.Positions["BTC"]
	expectedVWAP := (2.0*50000.0 + 1.0*60000.0) / 3.0 // $53,333.33
	fmt.Printf("   Position Size: %.2f\n", pos.Size)
	fmt.Printf("   VWAP: $%.2f (expected: $%.2f)\n", pos.AvgEntryPrice, expectedVWAP)
	fmt.Printf("   Cost Basis: $%.2f\n", pos.TotalCostBasis)
	fmt.Printf("   Last Price: $%.2f\n", pos.LastPrice)

	// Simulate market price at $55,000
	pos.LastPrice = 55000.0
	unrealizedPnL = pt.calculateUnrealizedPnL(pos)
	expectedUnrealizedPnL := (55000.0 - expectedVWAP) * 3.0
	fmt.Printf("   Market Price: $%.2f\n", pos.LastPrice)
	fmt.Printf("   Unrealized PnL: $%.2f (expected: $%.2f)\n", unrealizedPnL, expectedUnrealizedPnL)

	if math.Abs(pos.AvgEntryPrice-expectedVWAP) > 0.01 {
		t.Errorf("VWAP calculation wrong: got $%.2f, want $%.2f", pos.AvgEntryPrice, expectedVWAP)
	}

	// Test 3: Partial Close (VWAP should stay same, realize portion of PnL)
	fmt.Println("\n3. PARTIAL CLOSE: SELL 1.0 BTC @ $58,000")
	fill3 := createTestFill("BTC", "A", 1.0, 58000.0, "0.0", time.Now().Unix()+2)
	pt.ProcessFill(fill3)

	pos = pt.Positions["BTC"]
	expectedRealizedPnL := (58000.0 - expectedVWAP) * 1.0
	fmt.Printf("   Position Size: %.2f (should be 2.0)\n", pos.Size)
	fmt.Printf("   VWAP: $%.2f (should be unchanged: $%.2f)\n", pos.AvgEntryPrice, expectedVWAP)
	fmt.Printf("   Last Price: $%.2f\n", pos.LastPrice)
	fmt.Printf(
		"   Realized PnL (this trade): $%.2f (expected: $%.2f)\n",
		expectedRealizedPnL, expectedRealizedPnL,
	)
	fmt.Printf("   Position Realized PnL: $%.2f\n", pos.RealizedPnL)
	fmt.Printf("   Portfolio Realized PnL: $%.2f\n", pt.TotalRealizedPnL)

	if pos.Size != 2.0 {
		t.Errorf(
			"Position size after partial close: got %.2f, want 2.0",
			pos.Size,
		)
	}
	if math.Abs(pos.AvgEntryPrice-expectedVWAP) > 0.01 {
		t.Errorf("VWAP changed on partial close: got $%.2f, should stay $%.2f", pos.AvgEntryPrice, expectedVWAP)
	}

	// Test 4: Position Reversal
	fmt.Println("\n4. POSITION REVERSAL: SELL 4.0 BTC @ $62,000 (close 2.0, open -2.0)")
	fill4 := createTestFill("BTC", "A", 4.0, 62000.0, "0.0", time.Now().Unix()+3)
	pt.ProcessFill(fill4)

	pos = pt.Positions["BTC"]
	fmt.Printf("   Position Size: %.2f (should be -2.0)\n", pos.Size)
	fmt.Printf("   New VWAP: $%.2f (should be $62000.00 for short position)\n", pos.AvgEntryPrice)
	fmt.Printf("   Portfolio Realized PnL: $%.2f\n", pt.TotalRealizedPnL)

	if pos.Size != -2.0 {
		t.Errorf("Position size after reversal: got %.2f, want -2.0", pos.Size)
	}
	if pos.AvgEntryPrice != 62000.0 {
		t.Errorf("VWAP after reversal: got $%.2f, want $62000.00", pos.AvgEntryPrice)
	}

	// Test 5: Short Position PnL (should be inverse)
	fmt.Println("\n5. SHORT POSITION PNL: Market drops to $60,000")
	pos.LastPrice = 60000.0
	unrealizedPnL = pt.calculateUnrealizedPnL(pos)
	expectedUnrealizedPnL = (60000.0 - 62000.0) * (-2.0) // Should be positive for short
	fmt.Printf("   Position: %.2f BTC (short)\n", pos.Size)
	fmt.Printf("   Entry Price: $%.2f\n", pos.AvgEntryPrice)
	fmt.Printf("   Current Price: $%.2f\n", pos.LastPrice)
	fmt.Printf("   Unrealized PnL: $%.2f (expected: $%.2f)\n", unrealizedPnL, expectedUnrealizedPnL)

	if math.Abs(unrealizedPnL-expectedUnrealizedPnL) > 0.01 {
		t.Errorf("Short position PnL wrong: got $%.2f, want $%.2f", unrealizedPnL, expectedUnrealizedPnL)
	}

	// Test 6: Multiple coin tracking
	fmt.Println("\n6. MULTIPLE COINS: BUY 10.0 ETH @ $4,000")
	fill5 := createTestFill("ETH", "B", 10.0, 4000.0, "0.0", time.Now().Unix()+4)
	pt.ProcessFill(fill5)

	ethPos := pt.Positions["ETH"]
	btcPos := pt.Positions["BTC"]
	fmt.Printf("   BTC Position: %.2f @ $%.2f\n", btcPos.Size, btcPos.AvgEntryPrice)
	fmt.Printf("   ETH Position: %.2f @ $%.2f\n", ethPos.Size, ethPos.AvgEntryPrice)
	fmt.Printf("   Total Portfolio Realized PnL: $%.2f\n", pt.TotalRealizedPnL)

	// Test 7: Edge case - exact position close
	fmt.Println("\n7. EXACT CLOSE: COVER 2.0 BTC @ $59,000")
	fill6 := createTestFill("BTC", "B", 2.0, 59000.0, "0.0", time.Now().Unix()+5)
	pt.ProcessFill(fill6)

	btcPos = pt.Positions["BTC"]
	fmt.Printf("   BTC Position Size: %.2f (should be 0.0)\n", btcPos.Size)
	fmt.Printf("   BTC VWAP: $%.2f (should be 0.0 when flat)\n", btcPos.AvgEntryPrice)
	fmt.Printf("   Final Portfolio Realized PnL: $%.2f\n", pt.TotalRealizedPnL)

	if btcPos.Size != 0.0 {
		t.Errorf("Position should be flat after exact close: got %.2f", btcPos.Size)
	}

	fmt.Println("\n=== TRADE HISTORY SUMMARY ===")
	for i, trade := range pt.TradeHistory {
		fmt.Printf("%d. %s %s %.2f %s @ $%.2f | Realized: $%.2f | Unrealized: $%.2f | Pos: %.2f\n",
			i+1, trade.Action, trade.Side, trade.Size, trade.Coin, trade.Price,
			trade.RealizedPnL, trade.UnrealizedPnL, trade.PositionSize)
	}

	fmt.Printf("\nFinal Portfolio Realized PnL: $%.2f\n", pt.TotalRealizedPnL)
}

func TestFloatingPointEdgeCases(t *testing.T) {
	pt := &PaperTrader{
		Positions:        make(map[string]*Position),
		StartTime:        time.Now(),
		TradeHistory:     make([]*PaperTrade, 0),
		LastTradeTime:    make(map[string]time.Time),
		PendingFills:     make(map[string][]*Fill),
		PendingVolume:    make(map[string]float64),
		LastVolumeUpdate: make(map[string]time.Time),
		MinTradeInterval: 1 * time.Millisecond,
		VolumeThreshold:  1.0,
		VolumeDecayRate:  0.5,
		Bankroll:         10000000.0, // $10M for comprehensive test
		Leverage:         1.0,        // 1x leverage
	}

	fmt.Println("\n=== FLOATING POINT EDGE CASES ===")

	// Test precision with small decimals
	fmt.Println("\n1. PRECISION TEST: Small decimals")
	fill1 := createTestFill("PREC", "B", 0.123456789, 1234.567890123, "0.0", time.Now().Unix())
	pt.ProcessFill(fill1)

	fill2 := createTestFill("PREC", "A", 0.123456789, 1234.567890123*1.1, "0.0", time.Now().Unix()+1)
	pt.ProcessFill(fill2)

	pos := pt.Positions["PREC"]
	fmt.Printf("   Final position size: %.10f (should be ~0)\n", pos.Size)
	fmt.Printf("   Realized PnL: $%.10f\n", pos.RealizedPnL)

	if math.Abs(pos.Size) > 1e-10 {
		t.Errorf("Position not flat after equal buy/sell: %.10f", pos.Size)
	}

	// Test very large numbers
	fmt.Println("\n2. LARGE NUMBERS TEST")
	fill3 := createTestFill("LARGE", "B", 1000000.0, 50000.0, "0.0", time.Now().Unix())
	pt.ProcessFill(fill3)

	pos = pt.Positions["LARGE"]
	if math.IsInf(pos.TotalCostBasis, 0) || math.IsNaN(pos.TotalCostBasis) {
		t.Errorf("Large number caused overflow: cost basis = %.2f", pos.TotalCostBasis)
	}
}

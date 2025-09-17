package main

import (
	"fmt"
	"testing"
	"time"
)

func TestCashflowAndBankrollManagement(t *testing.T) {
	// Enhanced paper trader with cashflow tracking
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
		Bankroll:         200000.0, // $200k for cashflow test
		Leverage:         1.0,      // 1x leverage
	}

	fmt.Println("\n=== CASHFLOW & BANKROLL MANAGEMENT TEST ===")

	// Starting bankroll
	startingCash := 100000.0
	currentCash := startingCash
	fmt.Printf("Starting Bankroll: $%.2f\n", currentCash)

	// Trade 1: Buy 1 BTC at $50,000 (cash outflow)
	fmt.Println("\n1. BUY 1.0 BTC @ $50,000")
	fill1 := createTestFill("BTC", "B", 1.0, 50000.0, "0.0", time.Now().Unix())
	pt.ProcessFill(fill1)

	// Calculate cashflow
	cashFlow1 := -1.0 * 50000.0 // Money out
	currentCash += cashFlow1

	pos := pt.Positions["BTC"]
	fmt.Printf("   Cashflow: $%.2f (money out)\n", cashFlow1)
	fmt.Printf("   Remaining Cash: $%.2f\n", currentCash)
	fmt.Printf("   Position: %.2f BTC @ VWAP $%.2f\n", pos.Size, pos.AvgEntryPrice)
	fmt.Printf("   Position Value: $%.2f (at cost)\n", pos.Size*pos.AvgEntryPrice)
	fmt.Printf("   Total Portfolio Value: $%.2f\n", currentCash+pos.Size*pos.AvgEntryPrice)

	// Market update: BTC goes to $55,000
	fmt.Println("\n2. MARKET UPDATE: BTC -> $55,000")
	marketPrice := 55000.0
	pos.LastPrice = marketPrice
	unrealizedPnL := pt.calculateUnrealizedPnL(pos)

	fmt.Printf("   Market Price: $%.2f\n", marketPrice)
	fmt.Printf("   Unrealized PnL: $%.2f\n", unrealizedPnL)
	fmt.Printf("   Position Market Value: $%.2f\n", pos.Size*marketPrice)
	fmt.Printf("   Total Portfolio Value: $%.2f\n", currentCash+pos.Size*marketPrice)
	fmt.Printf("   Total P&L: $%.2f\n", (currentCash+pos.Size*marketPrice)-startingCash)

	// Trade 2: Sell 0.5 BTC at $55,000 (cash inflow + realize PnL)
	fmt.Println("\n3. SELL 0.5 BTC @ $55,000")
	fill2 := createTestFill("BTC", "A", 0.5, 55000.0, "0.0", time.Now().Unix()+1)
	pt.ProcessFill(fill2)

	cashFlow2 := 0.5 * 55000.0 // Money in
	currentCash += cashFlow2

	pos = pt.Positions["BTC"]
	fmt.Printf("   Cashflow: $%.2f (money in)\n", cashFlow2)
	fmt.Printf("   Realized PnL: $%.2f\n", pos.RealizedPnL)
	fmt.Printf("   Remaining Cash: $%.2f\n", currentCash)
	fmt.Printf("   Remaining Position: %.2f BTC @ VWAP $%.2f\n", pos.Size, pos.AvgEntryPrice)
	fmt.Printf("   Remaining Position Value: $%.2f\n", pos.Size*marketPrice)
	fmt.Printf("   Total Portfolio Value: $%.2f\n", currentCash+pos.Size*marketPrice)

	// Check bankroll constraint
	fmt.Println("\n4. BANKROLL CONSTRAINT CHECK")
	maxBTCPurchase := currentCash / marketPrice
	fmt.Printf("   Available Cash: $%.2f\n", currentCash)
	fmt.Printf("   Current BTC Price: $%.2f\n", marketPrice)
	fmt.Printf("   Max BTC Purchase: %.4f BTC\n", maxBTCPurchase)

	if currentCash < 0 {
		t.Errorf("⚠️  MARGIN CALL: Negative cash balance!")
	}

	// Trade 3: Try to buy more than available cash (should fail in real system)
	fmt.Println("\n5. OVER-LEVERAGE TEST: Try to buy 2.0 BTC")
	requiredCash := 2.0 * marketPrice
	fmt.Printf("   Required Cash: $%.2f\n", requiredCash)
	fmt.Printf("   Available Cash: $%.2f\n", currentCash)

	if requiredCash > currentCash {
		fmt.Printf("   ❌ TRADE REJECTED: Insufficient funds\n")
		fmt.Printf("   Shortfall: $%.2f\n", requiredCash-currentCash)
	} else {
		fmt.Printf("   ✅ TRADE APPROVED: Sufficient funds\n")
	}

	// Final portfolio summary
	fmt.Println("\n=== FINAL PORTFOLIO SUMMARY ===")
	totalCashflow := cashFlow1 + cashFlow2
	totalPortfolioValue := currentCash + pos.Size*marketPrice
	totalPnL := totalPortfolioValue - startingCash

	fmt.Printf("Starting Bankroll: $%.2f\n", startingCash)
	fmt.Printf("Total Cashflow: $%.2f\n", totalCashflow)
	fmt.Printf("Current Cash: $%.2f\n", currentCash)
	fmt.Printf("Position Value: $%.2f (%.2f BTC @ $%.2f)\n", pos.Size*marketPrice, pos.Size, marketPrice)
	fmt.Printf("Total Portfolio Value: $%.2f\n", totalPortfolioValue)
	fmt.Printf("Total P&L: $%.2f\n", totalPnL)
	fmt.Printf("Realized P&L: $%.2f\n", pt.TotalRealizedPnL)
	fmt.Printf("Unrealized P&L: $%.2f\n", pt.calculateUnrealizedPnL(pos))

	// Verify accounting
	expectedPnL := pt.TotalRealizedPnL + pt.calculateUnrealizedPnL(pos)
	if fmt.Sprintf("%.2f", totalPnL) != fmt.Sprintf("%.2f", expectedPnL) {
		t.Errorf(
			"P&L accounting mismatch: portfolio P&L $%.2f != realized+unrealized $%.2f",
			totalPnL, expectedPnL,
		)
	}

	fmt.Println("\n✅ Key Insights:")
	fmt.Println("1. Cashflow ≠ P&L (cashflow is money in/out, P&L is profit/loss)")
	fmt.Println("2. Bankroll constraint: can't buy more than available cash")
	fmt.Println("3. Total P&L = Realized P&L + Unrealized P&L")
	fmt.Println("4. Portfolio Value = Cash + Position Market Value")
}

func TestRealisticMarketPriceScenario(t *testing.T) {
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
		Bankroll:         200000.0, // $200k for cashflow test
		Leverage:         1.0,      // 1x leverage
	}

	fmt.Println("\n=== REALISTIC MARKET PRICE SCENARIO ===")

	// Scenario: You're copying someone's trades, but by the time you execute,
	// the market has moved slightly

	fmt.Println("\n1. COPY TRADE: Target bought 1 BTC @ $50,000")
	fill1 := createTestFill("BTC", "B", 1.0, 50000.0, "0.0", time.Now().Unix())
	pt.ProcessFill(fill1)

	pos := pt.Positions["BTC"]
	fmt.Printf("   Your Position: %.2f BTC @ VWAP $%.2f\n", pos.Size, pos.AvgEntryPrice)
	fmt.Printf("   Your Fill Price: $%.2f\n", pos.LastPrice)

	// By the time your trade executed, market moved to $50,100
	fmt.Println("\n2. MARKET REALITY: BTC current price is $50,100")
	actualMarketPrice := 50100.0
	pos.LastPrice = actualMarketPrice

	unrealizedPnL := pt.calculateUnrealizedPnL(pos)
	fmt.Printf("   Current Market Price: $%.2f\n", actualMarketPrice)
	fmt.Printf("   Your VWAP: $%.2f\n", pos.AvgEntryPrice)
	fmt.Printf("   Unrealized P&L: $%.2f\n", unrealizedPnL)

	fmt.Println("\n3. LATER: Market drops to $49,500")
	pos.LastPrice = 49500.0
	unrealizedPnL = pt.calculateUnrealizedPnL(pos)
	fmt.Printf("   Current Market Price: $%.2f\n", pos.LastPrice)
	fmt.Printf("   Unrealized P&L: $%.2f (now negative)\n", unrealizedPnL)

	fmt.Println("\n✅ Key Insight:")
	fmt.Println("LastPrice should represent current market price, not your fill price")
	fmt.Println("Unrealized P&L = (Current Market Price - VWAP) × Position Size")
}

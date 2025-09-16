package main

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestExtremePositionSizes(t *testing.T) {
	pt := NewPaperTrader()

	extremeTests := []struct {
		name string
		size float64
	}{
		{"Tiny size", 0.000001},
		{"Large size", 1000000.0},
		{"Very large size", 1e9},
		{"Precision edge", 0.123456789},
	}

	for _, tt := range extremeTests {
		t.Run(tt.name, func(t *testing.T) {
			fill := createTestFill("BTC", "B", tt.size, 50000.0, "0.0", time.Now().Unix())
			pt.ProcessFill(fill)

			position := pt.Positions["BTC"]
			if math.IsNaN(position.Size) || math.IsInf(position.Size, 0) {
				t.Errorf("Position size became NaN or Inf: %f", position.Size)
			}

			if math.IsNaN(position.AvgEntryPrice) || math.IsInf(position.AvgEntryPrice, 0) {
				t.Errorf("Average price became NaN or Inf: %f", position.AvgEntryPrice)
			}
		})
	}
}

func TestExtremeMarketPrices(t *testing.T) {
	pt := NewPaperTrader()

	priceTests := []struct {
		name  string
		price float64
	}{
		{"Very low price", 0.001},
		{"Very high price", 1e9},
		{"Precision price", 12345.6789},
		{"Round number", 100000.0},
	}

	for _, tt := range priceTests {
		t.Run(tt.name, func(t *testing.T) {
			fill := createTestFill("TEST", "B", 1.0, tt.price, "0.0", time.Now().Unix())
			pt.ProcessFill(fill)

			position := pt.Positions["TEST"]
			if math.IsNaN(position.AvgEntryPrice) || math.IsInf(position.AvgEntryPrice, 0) {
				t.Errorf("Average price became NaN or Inf with price %f: got %f", tt.price, position.AvgEntryPrice)
			}

			unrealizedPnL := pt.calculateUnrealizedPnL(position)
			if math.IsNaN(unrealizedPnL) || math.IsInf(unrealizedPnL, 0) {
				t.Errorf("Unrealized PnL became NaN or Inf: %f", unrealizedPnL)
			}
		})
	}
}

func TestConcurrentPaperTrading(t *testing.T) {
	pt := NewPaperTrader()
	numGoroutines := 100
	tradesPerGoroutine := 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Test concurrent access to paper trader
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < tradesPerGoroutine; j++ {
				coin := fmt.Sprintf("COIN%d", id%5) // 5 different coins
				side := "B"
				if j%2 == 1 {
					side = "A"
				}

				fill := createTestFill(
					coin,
					side,
					float64(j+1),
					float64(50000+id*100+j*10),
					fmt.Sprintf("%.2f", float64(id*j)),
					time.Now().Unix()+int64(id*1000+j),
				)
				fill.Hash = fmt.Sprintf("concurrent_%d_%d", id, j)

				pt.ProcessFill(fill)
			}
		}(i)
	}

	wg.Wait()

	// Verify no data corruption occurred
	expectedTotalTrades := numGoroutines * tradesPerGoroutine
	if pt.TotalTrades != expectedTotalTrades {
		t.Errorf("Concurrent test: total trades = %d, want %d", pt.TotalTrades, expectedTotalTrades)
	}

	// Verify positions are reasonable
	for coin, position := range pt.Positions {
		if math.IsNaN(position.Size) || math.IsInf(position.Size, 0) {
			t.Errorf("Concurrent test: %s position size corrupted: %f", coin, position.Size)
		}

		if position.AvgEntryPrice < 0 {
			t.Errorf("Concurrent test: %s average price negative: %f", coin, position.AvgEntryPrice)
		}
	}
}

func TestMemoryLeakPrevention(t *testing.T) {
	pt := NewPaperTrader()

	// Simulate very long running session
	numTrades := 10000

	for i := 0; i < numTrades; i++ {
		fill := createTestFill(
			"BTC",
			"B",
			1.0,
			float64(50000+i),
			"10.0",
			time.Now().Unix()+int64(i),
		)
		fill.Hash = fmt.Sprintf("memory_test_%d", i)

		pt.ProcessFill(fill)

		// Periodically check memory isn't growing unbounded
		if i%1000 == 0 {
			if len(pt.TradeHistory) > numTrades {
				t.Errorf("Trade history growing unbounded: %d trades", len(pt.TradeHistory))
			}
		}
	}

	// Verify final state
	if pt.TotalTrades != numTrades {
		t.Errorf("Memory test: trades = %d, want %d", pt.TotalTrades, numTrades)
	}
}

func TestRapidPositionFlips(t *testing.T) {
	pt := NewPaperTrader()

	// Simulate rapid position flips like high-frequency trading
	basePrice := 50000.0
	size := 10.0

	for i := 0; i < 100; i++ {
		side := "B"
		if i%2 == 1 {
			side = "A"
			size = 20.0 // Larger size to flip position
		} else {
			size = 10.0
		}

		price := basePrice + float64(i)*10
		fill := createTestFill("BTC", side, size, price, "100.0", time.Now().Unix()+int64(i))
		fill.Hash = fmt.Sprintf("flip_test_%d", i)

		pt.ProcessFill(fill)

		// Verify position is never unreasonably large
		position := pt.Positions["BTC"]
		if math.Abs(position.Size) > 50 {
			t.Errorf("Position flip test: position too large at step %d: %f", i, position.Size)
		}
	}

	// Should have significant realized PnL from all the flips
	if pt.TotalRealizedPnL <= 0 {
		t.Errorf("Position flip test: no realized PnL generated: %f", pt.TotalRealizedPnL)
	}
}

func TestFloatingPointPrecision(t *testing.T) {
	pt := NewPaperTrader()

	// Test with values that could cause floating point precision issues
	precisionTests := []struct {
		name  string
		size  float64
		price float64
	}{
		{"Small decimals", 0.123456789, 1234.567890123},
		{"Large numbers", 999999.999999, 123456.789012},
		{"Scientific notation", 1e-8, 1e8},
		{"Repeating decimals", 1.0 / 3.0, 10000.0 / 3.0},
	}

	for _, tt := range precisionTests {
		t.Run(tt.name, func(t *testing.T) {
			// Buy then sell to test precision in PnL calculation
			buyFill := createTestFill("PREC", "B", tt.size, tt.price, "0.0", time.Now().Unix())
			buyFill.Hash = "precision_buy_" + tt.name

			sellFill := createTestFill("PREC", "A", tt.size, tt.price*1.1, "0.0", time.Now().Unix()+1)
			sellFill.Hash = "precision_sell_" + tt.name

			pt.ProcessFill(buyFill)
			pt.ProcessFill(sellFill)

			position := pt.Positions["PREC"]

			// Position should be flat after buy/sell same size
			if math.Abs(position.Size) > 1e-10 {
				t.Errorf("Precision test %s: position not flat: %e", tt.name, position.Size)
			}

			// Should have some realized PnL
			if pt.TotalRealizedPnL <= 0 {
				t.Errorf("Precision test %s: no PnL realized", tt.name)
			}
		})
	}
}

func TestTimeOrderingEdgeCases(t *testing.T) {
	pt := NewPaperTrader()

	now := time.Now().Unix()

	// Test trades with same timestamp
	fills := []*Fill{
		createTestFill("BTC", "B", 1.0, 50000.0, "0.0", now),
		createTestFill("BTC", "B", 1.0, 50001.0, "0.0", now), // Same timestamp
		createTestFill("BTC", "A", 1.0, 51000.0, "1000.0", now+1),
	}

	for i, fill := range fills {
		fill.Hash = fmt.Sprintf("time_test_%d", i)
		pt.ProcessFill(fill)
	}

	// Should handle same timestamps gracefully
	if pt.TotalTrades != len(fills) {
		t.Errorf("Time ordering test: trades = %d, want %d", pt.TotalTrades, len(fills))
	}

	position := pt.Positions["BTC"]
	expectedSize := 1.0 // 2 buys, 1 sell
	if position.Size != expectedSize {
		t.Errorf("Time ordering test: position = %f, want %f", position.Size, expectedSize)
	}
}

func TestNegativePnLHandling(t *testing.T) {
	pt := NewPaperTrader()

	// Test sequence that should generate losses
	fills := []*Fill{
		// Buy high
		createTestFill("LOSS", "B", 10.0, 60000.0, "0.0", time.Now().Unix()),
		// Sell low
		createTestFill("LOSS", "A", 10.0, 50000.0, "-100000.0", time.Now().Unix()+1),
	}

	for i, fill := range fills {
		fill.Hash = fmt.Sprintf("loss_test_%d", i)
		pt.ProcessFill(fill)
	}

	// Should handle negative PnL correctly
	if pt.TotalRealizedPnL >= 0 {
		t.Errorf("Negative PnL test: should have losses, got PnL = %f", pt.TotalRealizedPnL)
	}

	position := pt.Positions["LOSS"]
	if position.Size != 0 {
		t.Errorf("Negative PnL test: position should be flat: %f", position.Size)
	}
}

func TestMassiveVolumeHandling(t *testing.T) {
	pt := NewPaperTrader()

	// Test with very large trade volumes
	largeFill := createTestFill("WHALE", "B", 1e6, 50000.0, "0.0", time.Now().Unix())
	largeFill.Hash = "massive_volume"

	pt.ProcessFill(largeFill)

	position := pt.Positions["WHALE"]

	// Verify no overflow/underflow
	if math.IsInf(position.Size, 0) || math.IsNaN(position.Size) {
		t.Errorf("Massive volume test: position size invalid: %f", position.Size)
	}

	if math.IsInf(position.TotalCostBasis, 0) || math.IsNaN(position.TotalCostBasis) {
		t.Errorf("Massive volume test: cost basis invalid: %f", position.TotalCostBasis)
	}

	// Calculate unrealized PnL with current price
	position.LastPrice = 55000.0
	unrealizedPnL := pt.calculateUnrealizedPnL(position)

	if math.IsInf(unrealizedPnL, 0) || math.IsNaN(unrealizedPnL) {
		t.Errorf("Massive volume test: unrealized PnL invalid: %f", unrealizedPnL)
	}

	// Should be exactly 5 billion profit (1M * 5000 price difference)
	expectedPnL := 5e9
	if math.Abs(unrealizedPnL-expectedPnL) > 1e6 { // Allow small precision errors
		t.Errorf("Massive volume test: unrealized PnL = %f, want ~%f", unrealizedPnL, expectedPnL)
	}
}

func TestRandomizedTradingStress(t *testing.T) {
	pt := NewPaperTrader()
	rand.Seed(42) // Deterministic randomness

	coins := []string{"BTC", "ETH", "SOL", "AVAX", "DOT"}
	sides := []string{"B", "A"}

	numTrades := 1000

	for i := 0; i < numTrades; i++ {
		coin := coins[rand.Intn(len(coins))]
		side := sides[rand.Intn(len(sides))]
		size := rand.Float64()*100 + 0.01     // 0.01 to 100
		price := rand.Float64()*100000 + 1000 // 1000 to 101000
		pnl := (rand.Float64() - 0.5) * 10000 // -5000 to 5000

		fill := createTestFill(coin, side, size, price, fmt.Sprintf("%.2f", pnl), time.Now().Unix()+int64(i))
		fill.Hash = fmt.Sprintf("random_%d", i)

		pt.ProcessFill(fill)

		// Sanity checks every 100 trades
		if i%100 == 0 {
			for coinName, position := range pt.Positions {
				if math.IsNaN(position.Size) || math.IsInf(position.Size, 0) {
					t.Fatalf("Random stress test: %s position corrupted at trade %d: %f", coinName, i, position.Size)
				}

				if position.AvgEntryPrice < 0 && position.Size != 0 {
					t.Fatalf("Random stress test: %s negative avg price at trade %d: %f", coinName, i, position.AvgEntryPrice)
				}
			}
		}
	}

	// Final verification
	if pt.TotalTrades != numTrades {
		t.Errorf("Random stress test: trades = %d, want %d", pt.TotalTrades, numTrades)
	}

	// Should have positions in multiple coins
	if len(pt.Positions) != len(coins) {
		t.Errorf("Random stress test: positions = %d, want %d", len(pt.Positions), len(coins))
	}

	// Total realized PnL should be reasonable (not zero, not infinite)
	if math.IsNaN(pt.TotalRealizedPnL) || math.IsInf(pt.TotalRealizedPnL, 0) {
		t.Errorf("Random stress test: total PnL corrupted: %f", pt.TotalRealizedPnL)
	}
}

// Stress test for performance under heavy load
func BenchmarkHighFrequencyTrading(b *testing.B) {
	pt := NewPaperTrader()

	// Pre-generate fills to avoid allocation overhead in benchmark
	fills := make([]*Fill, b.N)
	for i := 0; i < b.N; i++ {
		side := "B"
		if i%2 == 1 {
			side = "A"
		}

		fills[i] = createTestFill(
			"BTC",
			side,
			1.0+float64(i%10)*0.1,
			50000.0+float64(i)*0.01,
			"10.0",
			time.Now().Unix()+int64(i),
		)
		fills[i].Hash = fmt.Sprintf("bench_%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt.ProcessFill(fills[i])
	}
}

func BenchmarkUnrealizedPnLCalculation(b *testing.B) {
	pt := NewPaperTrader()

	// Setup position
	fill := createTestFill("BTC", "B", 100.0, 50000.0, "0.0", time.Now().Unix())
	pt.ProcessFill(fill)

	position := pt.Positions["BTC"]
	position.LastPrice = 55000.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt.calculateUnrealizedPnL(position)
	}
}

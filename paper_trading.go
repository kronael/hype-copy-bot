package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PaperTrader struct {
	mu               sync.Mutex
	Positions        map[string]*Position
	TotalRealizedPnL float64
	TotalTrades      int
	StartTime        time.Time
	TradeHistory     []*PaperTrade
	LastTradeTime    map[string]time.Time
	PendingFills     map[string][]*Fill
	MinTradeInterval time.Duration
	VolumeThreshold  float64              // Dollar volume threshold to trigger trade
	PendingVolume    map[string]float64   // Accumulated volume per coin
	LastVolumeUpdate map[string]time.Time // When volume started accumulating per coin
	VolumeDecayRate  float64              // Rate of volume decay per minute (e.g., 0.5 = 50%)
}

type Position struct {
	Coin           string
	Size           float64 // positive = long, negative = short, 0 = flat
	AvgEntryPrice  float64 // volume-weighted average
	TotalCostBasis float64 // total cost of position
	RealizedPnL    float64 // only when position reduced/closed
	LastPrice      float64 // for unrealized PnL calculation
	OpenTime       time.Time
	TradeCount     int
}

type PaperTrade struct {
	Timestamp     time.Time
	Coin          string
	Action        string // OPEN, ADD, REDUCE, CLOSE, REVERSE
	Side          string // BUY, SELL
	Size          float64
	Price         float64
	RealizedPnL   float64
	PositionSize  float64 // position after this trade
	UnrealizedPnL float64
}

type PositionAction int

const (
	ActionOpen PositionAction = iota
	ActionAdd
	ActionReduce
	ActionClose
	ActionReverse
)

func (pa PositionAction) String() string {
	switch pa {
	case ActionOpen:
		return "OPEN"
	case ActionAdd:
		return "ADD"
	case ActionReduce:
		return "REDUCE"
	case ActionClose:
		return "CLOSE"
	case ActionReverse:
		return "REVERSE"
	default:
		return "UNKNOWN"
	}
}

func (pa PositionAction) Emoji() string {
	switch pa {
	case ActionOpen:
		return "🟢"
	case ActionAdd:
		return "🔵"
	case ActionReduce:
		return "🟡"
	case ActionClose:
		return "🔴"
	case ActionReverse:
		return "🔄"
	default:
		return "❓"
	}
}

func NewPaperTrader() *PaperTrader {
	return &PaperTrader{
		Positions:        make(map[string]*Position),
		StartTime:        time.Now(),
		TradeHistory:     make([]*PaperTrade, 0),
		LastTradeTime:    make(map[string]time.Time),
		PendingFills:     make(map[string][]*Fill),
		PendingVolume:    make(map[string]float64),
		LastVolumeUpdate: make(map[string]time.Time),
		MinTradeInterval: 60 * time.Second, // 1 minute minimum between trades
		VolumeThreshold:  1000.0,           // $1000 volume threshold to trigger trade
		VolumeDecayRate:  0.5,              // 50% decay per minute
	}
}

// NewTestPaperTrader creates a paper trader optimized for testing
func NewTestPaperTrader() *PaperTrader {
	return &PaperTrader{
		Positions:        make(map[string]*Position),
		StartTime:        time.Now(),
		TradeHistory:     make([]*PaperTrade, 0),
		LastTradeTime:    make(map[string]time.Time),
		PendingFills:     make(map[string][]*Fill),
		PendingVolume:    make(map[string]float64),
		LastVolumeUpdate: make(map[string]time.Time),
		MinTradeInterval: 1 * time.Millisecond, // Almost immediate for tests
		VolumeThreshold:  0.0,                  // No volume threshold - process immediately for tests
		VolumeDecayRate:  0.5,                  // 50% decay per minute
	}
}

func (pt *PaperTrader) ProcessFill(fill *Fill) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Skip zero-size trades entirely
	if fill.Size == 0 {
		return
	}

	// Add fill to pending queue
	if pt.PendingFills[fill.Coin] == nil {
		pt.PendingFills[fill.Coin] = make([]*Fill, 0)
	}
	pt.PendingFills[fill.Coin] = append(pt.PendingFills[fill.Coin], fill)

	// Apply volume decay before adding new volume
	pt.applyVolumeDecay(fill.Coin)

	// Add to pending volume
	fillVolume := fill.Size * fill.Price
	pt.PendingVolume[fill.Coin] += fillVolume

	// Track when volume accumulation started for this coin
	if _, exists := pt.LastVolumeUpdate[fill.Coin]; !exists {
		pt.LastVolumeUpdate[fill.Coin] = time.Now()
	}

	// Check if we should process trades (volume threshold OR time threshold)
	shouldProcessByVolume := pt.PendingVolume[fill.Coin] >= pt.VolumeThreshold

	// Time threshold: only check if we have pending volume accumulating
	shouldProcessByTime := false
	if pt.PendingVolume[fill.Coin] > 0 {
		volumeStartTime, timeExists := pt.LastVolumeUpdate[fill.Coin]
		if timeExists && time.Since(volumeStartTime) >= pt.MinTradeInterval {
			shouldProcessByTime = true
		}
	}

	if shouldProcessByVolume || shouldProcessByTime {
		pt.processAggregatedFills(fill.Coin)
	}
}

func (pt *PaperTrader) processAggregatedFills(coin string) {
	fills := pt.PendingFills[coin]
	if len(fills) == 0 {
		return
	}

	// Calculate aggregated values
	var totalSize, totalValue, totalClosedPnL float64
	var lastPrice float64
	var side string
	var lastTime int64

	for _, fill := range fills {
		size := fill.Size
		if fill.Side == "A" { // sell
			size = -fill.Size
		}
		totalSize += size
		totalValue += math.Abs(size) * fill.Price
		lastPrice = fill.Price
		side = fill.Side
		lastTime = fill.Time

		// Sum up closed PnL
		if closedPnL, err := strconv.ParseFloat(fill.ClosedPnl, 64); err == nil {
			totalClosedPnL += closedPnL
		}
	}

	// Always process - we've already hit the volume or time threshold

	// Calculate volume-weighted average price
	avgPrice := totalValue / math.Abs(totalSize)

	// Get or create position
	position := pt.getPosition(coin)

	// Calculate trade details
	oldSize := position.Size
	newSize := oldSize + totalSize

	// Determine action type
	action := pt.determineAction(oldSize, newSize)

	// Calculate realized PnL for position changes
	realizedPnL := pt.calculateRealizedPnL(position, totalSize, avgPrice, totalClosedPnL, action)

	// Update position
	pt.updatePosition(position, totalSize, avgPrice, realizedPnL)

	// Update last price for unrealized PnL calculation
	position.LastPrice = lastPrice

	// Update totals
	pt.TotalTrades++
	pt.TotalRealizedPnL += realizedPnL

	// Create trade record
	trade := &PaperTrade{
		Timestamp:     time.Unix(lastTime/1000, 0),
		Coin:          coin,
		Action:        action.String(),
		Side:          map[string]string{"B": "BUY", "A": "SELL"}[side],
		Size:          math.Abs(totalSize),
		Price:         avgPrice,
		RealizedPnL:   realizedPnL,
		PositionSize:  position.Size,
		UnrealizedPnL: pt.calculateUnrealizedPnL(position),
	}
	pt.TradeHistory = append(pt.TradeHistory, trade)

	// Update last trade time
	pt.LastTradeTime[coin] = time.Now()

	// Clear pending fills and volume
	pt.PendingFills[coin] = nil
	pt.PendingVolume[coin] = 0
	delete(pt.LastVolumeUpdate, coin)

	// Save fill data and account snapshot
	for _, fill := range fills {
		pt.SaveFill(fill, action.String(), trade.RealizedPnL, trade.UnrealizedPnL)
	}
	pt.SaveAccount()

	// Print trade with proper formatting
	pt.printTrade(trade, action)
}

// applyVolumeDecay reduces pending volume based on time since volume accumulation started
func (pt *PaperTrader) applyVolumeDecay(coin string) {
	lastUpdate, exists := pt.LastVolumeUpdate[coin]
	if !exists || pt.PendingVolume[coin] == 0 {
		return
	}

	elapsed := time.Since(lastUpdate)

	// Only apply decay after at least 10 seconds have passed
	// This prevents micro-second decay from affecting rapid fills
	if elapsed < 10*time.Second {
		return
	}

	minutes := elapsed.Minutes()

	// Apply exponential decay: volume * (1 - decayRate)^minutes
	decayFactor := math.Pow(1.0-pt.VolumeDecayRate, minutes)
	pt.PendingVolume[coin] *= decayFactor

	// If volume becomes very small, clear it completely
	if pt.PendingVolume[coin] < 1.0 {
		pt.PendingVolume[coin] = 0
		pt.PendingFills[coin] = nil
		delete(pt.LastVolumeUpdate, coin)
	}
}

func (pt *PaperTrader) getPosition(coin string) *Position {
	if pos, exists := pt.Positions[coin]; exists {
		return pos
	}

	pos := &Position{
		Coin:           coin,
		Size:           0,
		AvgEntryPrice:  0,
		TotalCostBasis: 0,
		RealizedPnL:    0,
		OpenTime:       time.Now(),
		TradeCount:     0,
	}
	pt.Positions[coin] = pos
	return pos
}

func (pt *PaperTrader) determineAction(oldSize, newSize float64) PositionAction {
	// Flat to Long/Short
	if oldSize == 0 && newSize != 0 {
		return ActionOpen
	}

	// Long/Short to Flat
	if oldSize != 0 && newSize == 0 {
		return ActionClose
	}

	// Position reversal (long to short or short to long)
	if (oldSize > 0 && newSize < 0) || (oldSize < 0 && newSize > 0) {
		return ActionReverse
	}

	// Same direction, bigger position
	if (oldSize > 0 && newSize > oldSize) || (oldSize < 0 && newSize < oldSize) {
		return ActionAdd
	}

	// Same direction, smaller position
	if (oldSize > 0 && newSize < oldSize && newSize > 0) || (oldSize < 0 && newSize > oldSize && newSize < 0) {
		return ActionReduce
	}

	return ActionAdd // default
}

func (pt *PaperTrader) calculateRealizedPnL(position *Position, tradeSize float64, price float64, closedPnL float64, action PositionAction) float64 {
	// ADD actions never realize PnL - we're just building the position
	if action == ActionAdd || action == ActionOpen {
		return 0
	}

	// Only use API's closedPnL for position reductions
	if closedPnL != 0 && (action == ActionReduce || action == ActionClose || action == ActionReverse) {
		return closedPnL
	}

	// If reducing or closing position, calculate realized PnL
	if (position.Size > 0 && tradeSize < 0) || (position.Size < 0 && tradeSize > 0) {
		// Calculate based on average entry price
		if position.AvgEntryPrice > 0 {
			reducedSize := math.Abs(tradeSize)

			// Ensure we don't reduce more than the position size
			if reducedSize > math.Abs(position.Size) {
				reducedSize = math.Abs(position.Size)
			}

			pnlPerUnit := price - position.AvgEntryPrice
			if position.Size < 0 {
				pnlPerUnit = position.AvgEntryPrice - price // short position
			}

			return pnlPerUnit * reducedSize
		}
	}

	return 0
}

func (pt *PaperTrader) updatePosition(position *Position, tradeSize float64, price float64, realizedPnL float64) {
	oldSize := position.Size
	newSize := oldSize + tradeSize

	// Update realized PnL
	position.RealizedPnL += realizedPnL

	// Update position size
	position.Size = newSize
	position.TradeCount++

	// Update average entry price (volume-weighted)
	if newSize == 0 {
		// Position closed
		position.AvgEntryPrice = 0
		position.TotalCostBasis = 0
	} else if oldSize == 0 {
		// New position
		position.AvgEntryPrice = price
		position.TotalCostBasis = price * math.Abs(tradeSize)
		position.OpenTime = time.Now()
	} else if (oldSize > 0 && newSize < 0) || (oldSize < 0 && newSize > 0) {
		// Position reversal - new position in opposite direction
		reversedSize := math.Abs(newSize)
		position.AvgEntryPrice = price
		position.TotalCostBasis = price * reversedSize
		position.OpenTime = time.Now()
	} else if (oldSize > 0 && tradeSize > 0) || (oldSize < 0 && tradeSize < 0) {
		// Adding to position - recalculate weighted average
		totalCost := position.TotalCostBasis + (price * math.Abs(tradeSize))
		position.TotalCostBasis = totalCost
		position.AvgEntryPrice = totalCost / math.Abs(newSize)
	}
	// For reducing positions, keep the same average entry price
}

func (pt *PaperTrader) calculateUnrealizedPnL(position *Position) float64 {
	if position.Size == 0 || position.AvgEntryPrice == 0 {
		return 0
	}

	pnlPerUnit := position.LastPrice - position.AvgEntryPrice
	return pnlPerUnit * position.Size
}

func (pt *PaperTrader) printTrade(trade *PaperTrade, action PositionAction) {
	actionStr := fmt.Sprintf("%s %s", action.Emoji(), action.String())

	// Position info
	positionStr := ""
	if trade.PositionSize == 0 {
		positionStr = "Position: FLAT"
	} else if trade.PositionSize > 0 {
		positionStr = fmt.Sprintf("Position: +%.2f %s", trade.PositionSize, trade.Coin)
	} else {
		positionStr = fmt.Sprintf("Position: %.2f %s", trade.PositionSize, trade.Coin)
	}

	// PnL info - always show both realized and unrealized
	pnlParts := []string{}
	if trade.RealizedPnL != 0 {
		pnlParts = append(pnlParts, fmt.Sprintf("Realized: $%.2f", trade.RealizedPnL))
	}
	// Always show unrealized PnL for clarity
	pnlParts = append(pnlParts, fmt.Sprintf("Unrealized: $%.2f", trade.UnrealizedPnL))

	pnlStr := strings.Join(pnlParts, " | ")

	log.Printf("%s %s %.2f %s @ $%.2f | %s | %s",
		actionStr,
		trade.Side,
		trade.Size,
		trade.Coin,
		trade.Price,
		positionStr,
		pnlStr)
}

func (pt *PaperTrader) PrintPortfolioSummary() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📊 PAPER TRADING PORTFOLIO SUMMARY")
	fmt.Println(strings.Repeat("=", 80))

	// Calculate total unrealized PnL
	totalUnrealized := 0.0
	activePositions := 0

	for _, position := range pt.Positions {
		if position.Size != 0 {
			activePositions++
			totalUnrealized += pt.calculateUnrealizedPnL(position)
		}
	}

	// Time and performance
	elapsed := time.Since(pt.StartTime)
	totalPnL := pt.TotalRealizedPnL + totalUnrealized

	fmt.Printf("⏱️  Session Duration: %v\n", elapsed.Round(time.Second))
	fmt.Printf("💰 Total Realized PnL: $%.2f\n", pt.TotalRealizedPnL)
	fmt.Printf("📈 Total Unrealized PnL: $%.2f\n", totalUnrealized)
	fmt.Printf("🎯 Total Portfolio PnL: $%.2f\n", totalPnL)
	fmt.Printf("📊 Total Trades: %d\n", pt.TotalTrades)
	fmt.Printf("📍 Active Positions: %d\n", activePositions)

	if pt.TotalTrades > 0 {
		fmt.Printf("📊 Avg PnL per Trade: $%.2f\n", totalPnL/float64(pt.TotalTrades))
	}

	// Active positions
	if activePositions > 0 {
		fmt.Println("\n🔄 ACTIVE POSITIONS:")
		fmt.Println(strings.Repeat("-", 60))
		for coin, position := range pt.Positions {
			if position.Size != 0 {
				unrealizedPnL := pt.calculateUnrealizedPnL(position)
				pnlPercent := 0.0
				if position.AvgEntryPrice > 0 {
					pnlPercent = ((position.LastPrice - position.AvgEntryPrice) / position.AvgEntryPrice) * 100
				}

				sizeStr := fmt.Sprintf("%.2f", position.Size)
				if position.Size > 0 {
					sizeStr = "+" + sizeStr
				}

				fmt.Printf("%-8s | %s | Avg: $%.2f | Last: $%.2f | PnL: $%.2f (%.2f%%)\n",
					coin, sizeStr, position.AvgEntryPrice, position.LastPrice,
					unrealizedPnL, pnlPercent)
			}
		}
	}

	fmt.Println(strings.Repeat("=", 80))
}

func (pt *PaperTrader) GetTotalTrades() int {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.TotalTrades
}

// SetVolumeThreshold sets the volume threshold for triggering trades
func (pt *PaperTrader) SetVolumeThreshold(threshold float64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.VolumeThreshold = threshold
}

// SetMinTradeInterval sets the minimum time between trades
func (pt *PaperTrader) SetMinTradeInterval(interval time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.MinTradeInterval = interval
}

func (pt *PaperTrader) PrintRecentTrades(count int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if len(pt.TradeHistory) == 0 {
		return
	}

	fmt.Printf("\n📋 LAST %d TRADES:\n", count)
	fmt.Println(strings.Repeat("-", 80))

	start := len(pt.TradeHistory) - count
	if start < 0 {
		start = 0
	}

	for i := start; i < len(pt.TradeHistory); i++ {
		trade := pt.TradeHistory[i]
		action := PositionAction(0)
		switch trade.Action {
		case "OPEN":
			action = ActionOpen
		case "ADD":
			action = ActionAdd
		case "REDUCE":
			action = ActionReduce
		case "CLOSE":
			action = ActionClose
		case "REVERSE":
			action = ActionReverse
		}

		fmt.Printf("%s | %s %s %.2f %s @ $%.2f | PnL: $%.2f\n",
			trade.Timestamp.Format("15:04:05"),
			action.Emoji(),
			trade.Side,
			trade.Size,
			trade.Coin,
			trade.Price,
			trade.RealizedPnL)
	}
}

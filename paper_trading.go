package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

type PaperTrader struct {
	Positions          map[string]*Position
	TotalRealizedPnL   float64
	TotalUnrealizedPnL float64
	TotalTrades        int
	StartTime          time.Time
	TradeHistory       []*PaperTrade
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
		Positions:    make(map[string]*Position),
		StartTime:    time.Now(),
		TradeHistory: make([]*PaperTrade, 0),
	}
}

func (pt *PaperTrader) ProcessFill(fill *Fill) {
	// Parse closed PnL
	closedPnL, err := strconv.ParseFloat(fill.ClosedPnl, 64)
	if err != nil {
		closedPnL = 0
	}

	// Get or create position
	position := pt.getPosition(fill.Coin)

	// Calculate trade details
	oldSize := position.Size
	tradeSize := fill.Size
	if fill.Side == "A" { // sell
		tradeSize = -fill.Size
	}
	newSize := oldSize + tradeSize

	// Determine action type
	action := pt.determineAction(oldSize, newSize)

	// Calculate realized PnL for position changes
	realizedPnL := pt.calculateRealizedPnL(position, tradeSize, fill.Price, closedPnL)

	// Update position
	pt.updatePosition(position, tradeSize, fill.Price, realizedPnL)

	// Update totals
	pt.TotalTrades++
	pt.TotalRealizedPnL += realizedPnL

	// Create trade record
	trade := &PaperTrade{
		Timestamp:     time.Unix(fill.Time/1000, 0),
		Coin:          fill.Coin,
		Action:        action.String(),
		Side:          map[string]string{"B": "BUY", "A": "SELL"}[fill.Side],
		Size:          fill.Size,
		Price:         fill.Price,
		RealizedPnL:   realizedPnL,
		PositionSize:  position.Size,
		UnrealizedPnL: pt.calculateUnrealizedPnL(position),
	}
	pt.TradeHistory = append(pt.TradeHistory, trade)

	// Update last price for unrealized PnL
	position.LastPrice = fill.Price

	// Print trade with proper formatting
	pt.printTrade(trade, action)
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

func (pt *PaperTrader) calculateRealizedPnL(position *Position, tradeSize float64, price float64, closedPnL float64) float64 {
	// Use the closed PnL from the API if available and not zero
	if closedPnL != 0 {
		return closedPnL
	}

	// If reducing or closing position, calculate realized PnL
	if (position.Size > 0 && tradeSize < 0) || (position.Size < 0 && tradeSize > 0) {
		// Calculate based on average entry price
		if position.AvgEntryPrice > 0 {
			reducedSize := tradeSize
			if position.Size > 0 {
				reducedSize = -tradeSize // selling long position
			} else {
				reducedSize = -tradeSize // covering short position
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
		position.TotalCostBasis = price * tradeSize
		position.OpenTime = time.Now()
	} else if (oldSize > 0 && tradeSize > 0) || (oldSize < 0 && tradeSize < 0) {
		// Adding to position - recalculate weighted average
		totalCost := position.TotalCostBasis + (price * tradeSize)
		position.TotalCostBasis = totalCost
		position.AvgEntryPrice = totalCost / newSize
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

	// PnL info
	pnlStr := ""
	if trade.RealizedPnL != 0 {
		pnlStr += fmt.Sprintf("Realized: $%.2f", trade.RealizedPnL)
	}
	if trade.UnrealizedPnL != 0 {
		if pnlStr != "" {
			pnlStr += " | "
		}
		pnlStr += fmt.Sprintf("Unrealized: $%.2f", trade.UnrealizedPnL)
	}

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

func (pt *PaperTrader) PrintRecentTrades(count int) {
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

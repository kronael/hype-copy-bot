package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

type PnLTracker struct {
	TotalRealizedPnL float64
	TotalTrades      int
	WinningTrades    int
	LosingTrades     int
	TotalVolume      float64
	StartTime        time.Time
	CoinPositions    map[string]*CoinPosition
	TradeHistory     []*TradeRecord
}

type CoinPosition struct {
	Coin          string
	NetPosition   float64 // positive = long, negative = short
	AvgEntryPrice float64
	RealizedPnL   float64
	UnrealizedPnL float64
	TotalVolume   float64
	TradeCount    int
}

type TradeRecord struct {
	Timestamp    time.Time
	Coin         string
	Side         string
	Size         float64
	Price        float64
	RealizedPnL  float64
	RunningTotal float64
}

func NewPnLTracker() *PnLTracker {
	return &PnLTracker{
		StartTime:     time.Now(),
		CoinPositions: make(map[string]*CoinPosition),
		TradeHistory:  make([]*TradeRecord, 0),
	}
}

func (p *PnLTracker) ProcessFill(fill *Fill) {
	// Parse the closed PnL from the fill
	closedPnL, err := strconv.ParseFloat(fill.ClosedPnl, 64)
	if err != nil {
		log.Printf("Error parsing closed PnL %s: %v", fill.ClosedPnl, err)
		closedPnL = 0
	}

	// Calculate trade value
	tradeValue := fill.Size * fill.Price

	// Update total statistics
	p.TotalTrades++
	p.TotalVolume += tradeValue
	p.TotalRealizedPnL += closedPnL

	// Track wins/losses based on realized PnL
	if closedPnL > 0 {
		p.WinningTrades++
	} else if closedPnL < 0 {
		p.LosingTrades++
	}

	// Update coin-specific position
	position := p.getCoinPosition(fill.Coin)
	position.TradeCount++
	position.TotalVolume += tradeValue
	position.RealizedPnL += closedPnL

	// Update net position (positive for buys, negative for sells)
	if fill.Side == "B" {
		position.NetPosition += fill.Size
	} else {
		position.NetPosition -= fill.Size
	}

	// Recalculate average entry price
	p.updateAvgEntryPrice(position, fill)

	// Create trade record
	record := &TradeRecord{
		Timestamp:    time.Unix(fill.Time/1000, 0),
		Coin:         fill.Coin,
		Side:         fill.Side,
		Size:         fill.Size,
		Price:        fill.Price,
		RealizedPnL:  closedPnL,
		RunningTotal: p.TotalRealizedPnL,
	}
	p.TradeHistory = append(p.TradeHistory, record)

	// Print trade details
	sideStr := "BUY"
	if fill.Side == "A" {
		sideStr = "SELL"
	}

	log.Printf("📊 TRADE: %s %s %.2f @ $%.6f | PnL: $%.2f | Total PnL: $%.2f",
		sideStr, fill.Coin, fill.Size, fill.Price, closedPnL, p.TotalRealizedPnL)
}

func (p *PnLTracker) getCoinPosition(coin string) *CoinPosition {
	if position, exists := p.CoinPositions[coin]; exists {
		return position
	}

	position := &CoinPosition{
		Coin:        coin,
		NetPosition: 0,
		RealizedPnL: 0,
	}
	p.CoinPositions[coin] = position
	return position
}

func (p *PnLTracker) updateAvgEntryPrice(position *CoinPosition, fill *Fill) {
	// Simple average price calculation for demonstration
	// In a real implementation, you'd want volume-weighted average
	if position.TradeCount == 1 {
		position.AvgEntryPrice = fill.Price
	} else {
		// This is a simplified calculation - real implementation would be more complex
		position.AvgEntryPrice = (position.AvgEntryPrice + fill.Price) / 2
	}
}

func (p *PnLTracker) PrintSummary() {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🏆 TRADING PERFORMANCE SUMMARY")
	fmt.Println(strings.Repeat("=", 80))

	// Time elapsed
	elapsed := time.Since(p.StartTime)
	fmt.Printf("⏱️  Session Duration: %v\n", elapsed.Round(time.Second))

	// Overall stats
	fmt.Printf("💰 Total Realized PnL: $%.2f\n", p.TotalRealizedPnL)
	fmt.Printf("📈 Total Trades: %d\n", p.TotalTrades)
	fmt.Printf("💵 Total Volume: $%.2f\n", p.TotalVolume)

	if p.TotalTrades > 0 {
		winRate := float64(p.WinningTrades) / float64(p.TotalTrades) * 100
		fmt.Printf("✅ Win Rate: %.1f%% (%d wins, %d losses)\n",
			winRate, p.WinningTrades, p.LosingTrades)
		fmt.Printf("📊 Avg PnL per Trade: $%.2f\n", p.TotalRealizedPnL/float64(p.TotalTrades))
	}

	// Top performing coins
	fmt.Println("\n🥇 TOP PERFORMING COINS:")
	fmt.Println(strings.Repeat("-", 50))
	for coin, position := range p.CoinPositions {
		if position.RealizedPnL != 0 {
			fmt.Printf("%-12s | PnL: $%8.2f | Trades: %3d | Volume: $%10.2f\n",
				coin, position.RealizedPnL, position.TradeCount, position.TotalVolume)
		}
	}

	fmt.Println(strings.Repeat("=", 80))
}

func (p *PnLTracker) PrintRecentTrades(count int) {
	if len(p.TradeHistory) == 0 {
		return
	}

	fmt.Printf("\n📋 LAST %d TRADES:\n", count)
	fmt.Println(strings.Repeat("-", 70))

	start := len(p.TradeHistory) - count
	if start < 0 {
		start = 0
	}

	for i := start; i < len(p.TradeHistory); i++ {
		trade := p.TradeHistory[i]
		sideStr := "BUY"
		if trade.Side == "A" {
			sideStr = "SELL"
		}
		fmt.Printf("%s | %s %-8s %8.2f @ $%.6f | PnL: $%8.2f\n",
			trade.Timestamp.Format("15:04:05"),
			sideStr, trade.Coin, trade.Size, trade.Price, trade.RealizedPnL)
	}
}

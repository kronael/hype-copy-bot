package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type Bot struct {
	config      *Config
	client      *HyperliquidClient
	running     bool
	stopChan    chan struct{}
	wg          sync.WaitGroup
	lastFillHash string
	processedFills map[string]bool
}

func NewBot(config *Config) (*Bot, error) {
	client, err := NewHyperliquidClient(config)
	if err != nil {
		return nil, err
	}

	return &Bot{
		config:         config,
		client:         client,
		stopChan:       make(chan struct{}),
		processedFills: make(map[string]bool),
	}, nil
}

func (b *Bot) Start() error {
	log.Println("Starting trade following bot...")
	b.running = true

	b.wg.Add(1)
	go b.monitorTrades()

	return nil
}

func (b *Bot) Stop() {
	if !b.running {
		return
	}

	log.Println("Stopping bot...")
	b.running = false
	close(b.stopChan)
	b.wg.Wait()
	b.client.Close()
}

func (b *Bot) monitorTrades() {
	defer b.wg.Done()

	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	log.Printf("Starting to monitor trades for account: %s", b.config.TargetAccount)

	for {
		select {
		case <-b.stopChan:
			return
		case <-ticker.C:
			if err := b.checkForNewTradesWithRetry(); err != nil {
				log.Printf("Error checking trades after retries: %v", err)
			}
		}
	}
}

func (b *Bot) checkForNewTrades() error {
	fills, err := b.client.GetUserFills(b.config.TargetAccount)
	if err != nil {
		return err
	}

	newFillsCount := 0
	maxFillsPerCheck := 50 // Safety limit to prevent overloading

	for _, fill := range fills {
		// Skip if we've already processed this fill
		if b.processedFills[fill.Hash] {
			continue
		}

		// Safety limit check
		if newFillsCount >= maxFillsPerCheck {
			log.Printf("Reached maximum fills per check (%d), deferring remaining fills", maxFillsPerCheck)
			break
		}

		// Mark as processed
		b.processedFills[fill.Hash] = true
		newFillsCount++

		log.Printf("New fill detected: %s %s %.6f @ %.6f (hash: %s)",
			fill.Side, fill.Coin, fill.Size, fill.Price, fill.Hash[:8])

		if err := b.processFill(fill); err != nil {
			log.Printf("Error processing fill: %v", err)
		}
	}

	if newFillsCount > 0 {
		log.Printf("Processed %d new fills", newFillsCount)
	}

	return nil
}

func (b *Bot) processFill(fill *Fill) error {
	// Calculate trade value
	tradeValue := fill.Size * fill.Price
	if tradeValue < b.config.CopyThreshold {
		log.Printf("Skipping small trade: value %.6f < threshold %.6f", tradeValue, b.config.CopyThreshold)
		return nil
	}

	// Convert fill side to order side (B = buy, A = sell)
	side := "buy"
	if fill.Side == "A" {
		side = "sell"
	}

	log.Printf("Copying trade: %s %s %.6f @ %.6f (value: %.2f)",
		side, fill.Coin, fill.Size, fill.Price, tradeValue)

	// For now, just log the trade copy since we don't have signing implemented
	// TODO: Implement actual order placement once signing is ready
	log.Printf("Would place order: %s %s %.6f @ %.6f", side, fill.Coin, fill.Size, fill.Price)

	// Uncomment when signing is implemented:
	// return b.client.PlaceOrder(&Order{
	// 	Coin:  fill.Coin,
	// 	Side:  side,
	// 	Size:  fill.Size,
	// 	Price: fill.Price,
	// 	Type:  "limit",
	// })

	return nil
}

func (b *Bot) checkForNewTradesWithRetry() error {
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := b.checkForNewTrades()
		if err == nil {
			return nil
		}

		log.Printf("Attempt %d/%d failed: %v", attempt, maxRetries, err)

		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * 2 * time.Second
			log.Printf("Retrying in %v...", waitTime)
			time.Sleep(waitTime)
		}
	}

	return fmt.Errorf("failed after %d attempts", maxRetries)
}
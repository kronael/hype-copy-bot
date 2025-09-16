package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type Bot struct {
	config         *Config
	client         *Client
	running        bool
	stopChan       chan struct{}
	wg             sync.WaitGroup
	lastFillHash   string
	processedFills map[string]bool
	paperTrader    *PaperTrader
}

func NewBot(config *Config) (*Bot, error) {
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}

	return &Bot{
		config:         config,
		client:         client,
		stopChan:       make(chan struct{}),
		processedFills: make(map[string]bool),
		paperTrader:    NewPaperTrader(),
	}, nil
}

func (b *Bot) Start() error {
	log.Println("starting trade following bot...")
	b.running = true

	b.wg.Add(1)
	go b.monitorTrades()

	return nil
}

func (b *Bot) Stop() {
	if !b.running {
		return
	}

	log.Println("stopping bot...")
	b.running = false
	close(b.stopChan)
	b.wg.Wait()
	b.client.Close()

	// Show final paper trading summary
	b.paperTrader.PrintPortfolioSummary()
	b.paperTrader.PrintRecentTrades(10)
}

func (b *Bot) monitorTrades() {
	defer b.wg.Done()

	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	log.Printf("starting to monitor trades for account: %s", b.config.TargetAccount)

	for {
		select {
		case <-b.stopChan:
			return
		case <-ticker.C:
			if err := b.checkTrades(); err != nil {
				log.Printf("Error checking trades after retries: %v", err)
				// Continue monitoring - API failures are expected and recoverable
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
		// Safety limit check
		if newFillsCount >= maxFillsPerCheck {
			log.Printf("Reached maximum fills per check (%d), deferring remaining fills", maxFillsPerCheck)
			break
		}

		// Skip if already processed (checked in process)
		if b.processedFills[fill.Hash] {
			continue
		}

		log.Printf("new fill detected: %s %s %.6f @ %.6f (hash: %s)",
			fill.Side, fill.Coin, fill.Size, fill.Price, fill.Hash[:8])

		if err := b.process(fill); err != nil {
			log.Printf("Error processing fill: %v", err)
		} else {
			// Only increment if actually processed (not skipped due to threshold)
			if b.processedFills[fill.Hash] {
				newFillsCount++
			}
		}
	}

	if newFillsCount > 0 {
		log.Printf("processed %d new fills", newFillsCount)

		// Show summary every 10 trades
		totalTrades := b.paperTrader.GetTotalTrades()
		if totalTrades > 0 && totalTrades%10 == 0 {
			b.paperTrader.PrintPortfolioSummary()
		}
	}

	return nil
}

func (b *Bot) process(fill *Fill) error {
	// Skip if we've already processed this fill
	if b.processedFills[fill.Hash] {
		return nil
	}

	// Calculate trade value
	tradeValue := fill.Size * fill.Price
	if tradeValue < b.config.CopyThreshold {
		return nil
	}

	// Mark as processed
	b.processedFills[fill.Hash] = true

	// Process this trade in paper trader
	b.paperTrader.ProcessFill(fill)

	return nil
}

func (b *Bot) checkTrades() error {
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := b.checkForNewTrades()
		if err == nil {
			return nil
		}

		log.Printf("Attempt %d/%d failed: %v", attempt, maxRetries, err)

		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * 2 * time.Second
			log.Printf("retrying in %v...", waitTime)
			time.Sleep(waitTime)
		}
	}

	return fmt.Errorf("failed after %d attempts", maxRetries)
}

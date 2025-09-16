package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

func testBasicConnection() {
	config := &Config{
		TargetAccount: os.Getenv("TARGET_ACCOUNT"),
		APIKey:        os.Getenv("API_KEY"),
		PrivateKey:    os.Getenv("PRIVATE_KEY"),
		UseTestnet:    true,
		CopyThreshold: 0.01,
	}

	if config.TargetAccount == "" {
		config.TargetAccount = "0x0000000000000000000000000000000000000000" // test address
	}
	if config.PrivateKey == "" {
		config.PrivateKey = "0000000000000000000000000000000000000000000000000000000000000000" // dummy key for testing
	}

	client, err := NewHyperliquidClient(config)
	if err != nil {
		log.Fatal("Failed to create client:", err)
	}

	fmt.Println("Testing basic connection to Hyperliquid API...")

	fills, err := client.GetUserFills(config.TargetAccount)
	if err != nil {
		log.Printf("Error getting fills (expected if target account is empty): %v", err)
	} else {
		fmt.Printf("Successfully retrieved %d fills\n", len(fills))
		for i, fill := range fills {
			if i >= 3 { // Only show first 3 fills
				break
			}
			fmt.Printf("Fill %d: %s %s %.6f @ %.6f\n", i+1, fill.Side, fill.Coin, fill.Size, fill.Price)
		}
	}

	fmt.Println("Basic connection test completed")
}

func testBot() {
	config := &Config{
		TargetAccount: os.Getenv("TARGET_ACCOUNT"),
		APIKey:        os.Getenv("API_KEY"),
		PrivateKey:    os.Getenv("PRIVATE_KEY"),
		UseTestnet:    false,  // Use mainnet to follow real trader
		CopyThreshold: 1000.0, // Set threshold to $1000 to focus on significant trades
	}

	if config.TargetAccount == "" {
		// Follow "The White Whale" - top Hyperliquid trader
		config.TargetAccount = "0xb8b9e3097c8b1dddf9c5ea9d48a7ebeaf09d67d2"
	}
	if config.PrivateKey == "" {
		config.PrivateKey = "0000000000000000000000000000000000000000000000000000000000000000" // dummy key for testing
	}

	fmt.Printf("🐋 Following 'The White Whale' - Top Hyperliquid Trader\n")
	fmt.Printf("📍 Target Account: %s\n", config.TargetAccount)
	fmt.Printf("💰 Copy Threshold: $%.2f\n", config.CopyThreshold)

	bot, err := NewBot(config)
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}

	fmt.Println("Starting PnL tracking for 30 seconds...")
	if err := bot.Start(); err != nil {
		log.Fatal("Failed to start bot:", err)
	}

	// Let it run for 30 seconds to capture more trades
	time.Sleep(30 * time.Second)

	fmt.Println("Stopping bot...")
	bot.Stop()
}

func init() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "test":
			testBasicConnection()
			os.Exit(0)
		case "bot":
			testBot()
			os.Exit(0)
		}
	}
}

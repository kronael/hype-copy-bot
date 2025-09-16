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
		UseTestnet:    true,
		CopyThreshold: 100.0, // Set a higher threshold for testing
	}

	if config.TargetAccount == "" {
		config.TargetAccount = "0x0000000000000000000000000000000000000000" // test address
	}
	if config.PrivateKey == "" {
		config.PrivateKey = "0000000000000000000000000000000000000000000000000000000000000000" // dummy key for testing
	}

	fmt.Printf("Testing bot with target account: %s\n", config.TargetAccount)
	fmt.Printf("Copy threshold: %.2f\n", config.CopyThreshold)

	bot, err := NewBot(config)
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}

	fmt.Println("Starting bot for 15 seconds...")
	if err := bot.Start(); err != nil {
		log.Fatal("Failed to start bot:", err)
	}

	// Let it run for 15 seconds
	time.Sleep(15 * time.Second)

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

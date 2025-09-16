package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Println("Starting Hyperliquid Trade Following Bot...")

	config, err := loadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	bot, err := NewBot(config)
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}

	if err := bot.Start(); err != nil {
		log.Fatal("Failed to start bot:", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down bot...")
	bot.Stop()
}

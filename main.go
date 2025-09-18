package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Configure Unix-style logging (no timestamp, simple format)
	log.SetFlags(0)
	log.SetPrefix("")

	log.Println("hype-copy-bot: starting")

	var configFile string
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	config, err := loadConfig(configFile)
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

	log.Println("hype-copy-bot: shutting down")
	bot.Stop()
}

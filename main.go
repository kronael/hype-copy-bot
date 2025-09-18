package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type unixLogger struct{}

func (u *unixLogger) Write(p []byte) (n int, err error) {
	timestamp := time.Now().Format("Jan 2 15:04:05")
	return fmt.Printf("%s %s", timestamp, string(p))
}

func main() {
	// Configure Unix syslog-style timestamp format (Jan 20 10:30:28)
	log.SetFlags(0)
	log.SetOutput(&unixLogger{})

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

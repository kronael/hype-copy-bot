package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the complete configuration structure
type Config struct {
	TargetAccount    string  `toml:"target_account"`
	APIKey           string  `toml:"api_key"`
	PrivateKey       string  `toml:"private_key"`
	CopyThreshold    float64 `toml:"copy_threshold"`
	PaperTradingOnly bool    `toml:"paper_trading_only"`
	DataDir          string  `toml:"data_dir"`
}

// GetDataDir returns the full data directory path with PREFIX env var support
func (c *Config) GetDataDir() string {
	dataDir := c.DataDir
	if dataDir == "" {
		dataDir = "data/hype-copy-bot" // default
	}

	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/srv" // default
	}

	return filepath.Join(prefix, dataDir)
}

func loadConfig(configFile string) (*Config, error) {
	// Load from specified config file or default config.toml
	return loadTOMLConfig(configFile)
}

func loadTOMLConfig(configFile string) (*Config, error) {
	var config Config

	// Use provided config file or default to config.toml
	if configFile == "" {
		configFile = "config.toml"
	}

	// Try to load the config file
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		return nil, err
	}

	// Set defaults
	if config.CopyThreshold == 0 {
		config.CopyThreshold = 1000.0
	}
	if !config.PaperTradingOnly {
		config.PaperTradingOnly = true
	}

	// Validate required fields
	if config.TargetAccount == "" {
		return nil, errors.New("target_account is required in config.toml")
	}

	// For paper trading, allow placeholder values for API credentials
	if config.PaperTradingOnly {
		log.Println("Paper trading mode: using placeholder API credentials")
		if config.APIKey == "your_api_key_here" {
			config.APIKey = "paper_trading_placeholder_api_key"
		}
		if config.PrivateKey == "your_64_character_hex_private_key_here" {
			config.PrivateKey = "0000000000000000000000000000000000000000000000000000000000000000"
		}
	} else {
		// Real trading requires real credentials
		if config.APIKey == "" || config.APIKey == "your_api_key_here" {
			return nil, errors.New("api_key not configured - real trading requires valid API key")
		}
		if config.PrivateKey == "" || config.PrivateKey == "your_64_character_hex_private_key_here" {
			return nil, errors.New("private_key not configured - real trading requires valid private key")
		}
	}

	return &config, nil
}

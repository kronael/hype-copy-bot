package main

import (
	"errors"
	"os"
)

type Config struct {
	TargetAccount    string
	APIKey          string
	PrivateKey      string
	UseTestnet      bool
	CopyThreshold   float64 // minimum trade size to copy
}

func loadConfig() (*Config, error) {
	config := &Config{
		UseTestnet:    true, // start with testnet for safety
		CopyThreshold: 0.01, // minimum $0.01 trade size
	}

	config.TargetAccount = os.Getenv("TARGET_ACCOUNT")
	if config.TargetAccount == "" {
		return nil, errors.New("TARGET_ACCOUNT environment variable is required")
	}

	config.APIKey = os.Getenv("API_KEY")
	if config.APIKey == "" {
		return nil, errors.New("API_KEY environment variable is required")
	}

	config.PrivateKey = os.Getenv("PRIVATE_KEY")
	if config.PrivateKey == "" {
		return nil, errors.New("PRIVATE_KEY environment variable is required")
	}

	return config, nil
}
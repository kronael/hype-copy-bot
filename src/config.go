package main

import (
	"errors"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
)

// Config represents the complete configuration structure
type Config struct {
	// Legacy environment variable support
	TargetAccount string  `toml:"-"`
	APIKey        string  `toml:"-"`
	PrivateKey    string  `toml:"-"`
	UseTestnet    bool    `toml:"-"`
	CopyThreshold float64 `toml:"-"`

	// TOML configuration structure
	Hyperliquid struct {
		TargetAccount string `toml:"target_account"`
		APIKey        string `toml:"api_key"`
		PrivateKey    string `toml:"private_key"`
		UseTestnet    bool   `toml:"use_testnet"`
		MainnetURL    string `toml:"mainnet_url"`
		TestnetURL    string `toml:"testnet_url"`
	} `toml:"hyperliquid"`

	Trading struct {
		CopyThreshold    float64 `toml:"copy_threshold"`
		PaperTradingOnly bool    `toml:"paper_trading_only"`
		MaxPositionSize  float64 `toml:"max_position_size"`
		MaxTotalExposure float64 `toml:"max_total_exposure"`
	} `toml:"trading"`

	Monitoring struct {
		PollInterval      int `toml:"poll_interval"`
		MaxFillsPerCheck  int `toml:"max_fills_per_check"`
		MaxRetries        int `toml:"max_retries"`
		RetryDelaySeconds int `toml:"retry_delay_seconds"`
	} `toml:"monitoring"`

	Logging struct {
		Level      string `toml:"level"`
		Format     string `toml:"format"`
		Structured bool   `toml:"structured"`
	} `toml:"logging"`

	Portfolio struct {
		SummaryInterval   int  `toml:"summary_interval"`
		RecentTradesCount int  `toml:"recent_trades_count"`
		RealtimePnL       bool `toml:"realtime_pnl"`
	} `toml:"portfolio"`

	Data struct {
		SaveTradeHistory bool   `toml:"save_trade_history"`
		HistoryFile      string `toml:"history_file"`
		SaveState        bool   `toml:"save_state"`
		StateFile        string `toml:"state_file"`
		CompressHistory  bool   `toml:"compress_history"`
	} `toml:"data"`
}

func loadConfig(configFile string) (*Config, error) {
	// First try to load from specified config file or default config.toml
	if config, err := loadTOMLConfig(configFile); err == nil {
		return config, nil
	}

	// Fall back to environment variables (legacy support)
	return loadEnvConfig()
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

	// Set defaults for any missing values
	setTOMLDefaults(&config)

	// Validate required fields
	if config.Hyperliquid.TargetAccount == "" {
		return nil, errors.New("hyperliquid.target_account is required in config.toml")
	}
	if config.Hyperliquid.APIKey == "" {
		return nil, errors.New("hyperliquid.api_key is required in config.toml")
	}
	if config.Hyperliquid.PrivateKey == "" {
		return nil, errors.New("hyperliquid.private_key is required in config.toml")
	}

	// Copy TOML values to legacy fields for backwards compatibility
	config.TargetAccount = config.Hyperliquid.TargetAccount
	config.APIKey = config.Hyperliquid.APIKey
	config.PrivateKey = config.Hyperliquid.PrivateKey
	config.UseTestnet = config.Hyperliquid.UseTestnet
	config.CopyThreshold = config.Trading.CopyThreshold

	return &config, nil
}

func loadEnvConfig() (*Config, error) {
	config := &Config{
		UseTestnet:    true, // start with testnet for safety
		CopyThreshold: 0.01, // minimum $0.01 trade size
	}

	// Check for prefixed environment variables first
	if account := os.Getenv("HYPERLIQUID_TARGET_ACCOUNT"); account != "" {
		config.TargetAccount = account
	} else if account := os.Getenv("TARGET_ACCOUNT"); account != "" {
		config.TargetAccount = account
	} else {
		return nil, errors.New("HYPERLIQUID_TARGET_ACCOUNT environment variable is required")
	}

	if apiKey := os.Getenv("HYPERLIQUID_API_KEY"); apiKey != "" {
		config.APIKey = apiKey
	} else if apiKey := os.Getenv("API_KEY"); apiKey != "" {
		config.APIKey = apiKey
	} else {
		return nil, errors.New("HYPERLIQUID_API_KEY environment variable is required")
	}

	if privateKey := os.Getenv("HYPERLIQUID_PRIVATE_KEY"); privateKey != "" {
		config.PrivateKey = privateKey
	} else if privateKey := os.Getenv("PRIVATE_KEY"); privateKey != "" {
		config.PrivateKey = privateKey
	} else {
		return nil, errors.New("HYPERLIQUID_PRIVATE_KEY environment variable is required")
	}

	// Optional settings with defaults
	if testnet := os.Getenv("HYPERLIQUID_USE_TESTNET"); testnet != "" {
		if val, err := strconv.ParseBool(testnet); err == nil {
			config.UseTestnet = val
		}
	}

	if threshold := os.Getenv("HYPERLIQUID_THRESHOLD"); threshold != "" {
		if val, err := strconv.ParseFloat(threshold, 64); err == nil {
			config.CopyThreshold = val
		}
	}

	return config, nil
}

func setTOMLDefaults(config *Config) {
	// Hyperliquid defaults
	if config.Hyperliquid.UseTestnet != true && config.Hyperliquid.UseTestnet != false {
		config.Hyperliquid.UseTestnet = true
	}
	if config.Hyperliquid.MainnetURL == "" {
		config.Hyperliquid.MainnetURL = "https://api.hyperliquid.xyz"
	}
	if config.Hyperliquid.TestnetURL == "" {
		config.Hyperliquid.TestnetURL = "https://api.hyperliquid-testnet.xyz"
	}

	// Trading defaults
	if config.Trading.CopyThreshold == 0 {
		config.Trading.CopyThreshold = 1000.0
	}
	if !config.Trading.PaperTradingOnly {
		config.Trading.PaperTradingOnly = true
	}

	// Monitoring defaults
	if config.Monitoring.PollInterval == 0 {
		config.Monitoring.PollInterval = 5
	}
	if config.Monitoring.MaxFillsPerCheck == 0 {
		config.Monitoring.MaxFillsPerCheck = 50
	}
	if config.Monitoring.MaxRetries == 0 {
		config.Monitoring.MaxRetries = 3
	}
	if config.Monitoring.RetryDelaySeconds == 0 {
		config.Monitoring.RetryDelaySeconds = 2
	}

	// Logging defaults
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "text"
	}

	// Portfolio defaults
	if config.Portfolio.SummaryInterval == 0 {
		config.Portfolio.SummaryInterval = 10
	}
	if config.Portfolio.RecentTradesCount == 0 {
		config.Portfolio.RecentTradesCount = 10
	}
	if !config.Portfolio.RealtimePnL {
		config.Portfolio.RealtimePnL = true
	}

	// Data defaults
	if !config.Data.SaveTradeHistory {
		config.Data.SaveTradeHistory = true
	}
	if config.Data.HistoryFile == "" {
		config.Data.HistoryFile = "data/trade_history.json"
	}
	if !config.Data.SaveState {
		config.Data.SaveState = true
	}
	if config.Data.StateFile == "" {
		config.Data.StateFile = "data/bot_state.json"
	}
}

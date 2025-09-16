# Hyperliquid Trade Following Bot

A simple Go bot that follows trades from a target account on Hyperliquid.

## Setup

1. Copy `.env.example` to `.env` and fill in your credentials:
   ```bash
   cp .env.example .env
   ```

2. Get your API credentials from https://app.hyperliquid.xyz/API

3. Set the target account address you want to follow

## Running

```bash
# Install dependencies
go mod tidy

# Run the bot
source .env && go run .
```

## Features

- Monitors target account for new trades
- Copies trades above a minimum threshold
- Uses testnet by default for safety
- Basic error handling and logging

## Warning

This is a basic implementation for testing. Use with caution and always test on testnet first.

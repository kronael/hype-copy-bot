# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

This represents accumulated wisdom from multiple projects and should be
adapted thoughtfully to this codebase.

## Project Overview

This is a Hyperliquid trade following bot (hype-copy-bot) that monitors
successful traders and provides comprehensive paper trading analytics with
real-time position management.

## Key Principles

**Think through data flow before implementing**

- Fills from target account come via API polling every 5 seconds
- Duplicate detection via hash tracking prevents reprocessing
- Position updates follow state machine: OPEN -> ADD/REDUCE -> CLOSE/REVERSE
- PnL calculations happen on position changes, not fills
- Volume-weighted average pricing requires cost basis tracking

**Only record surprising things, not common sense**

- Shorter is better (main binary, not hyperliquid-bot)
- Debug builds by default (make build, not make release)
- No redundant naming - position.Size not position.PositionSize
- Continue from last state via processedFills map
- Nothing baroque - old Unix style
- Use lowercase for normal logging messages
- Capitalize first letter ONLY for error names and error messages
- Simple, direct commit messages - no fluff or marketing speak
- Commit automatically after completing meaningful work
- Just do it, don't ask
- Short CLI flags and file extensions (.json not .jsonl)
- No fancy outputs - no progress bars, spinners, or animations unless essential
- Stick to 80-character line length, max 100 if absolutely necessary

**Error handling philosophy**

- Return errors from functions, never panic except at top level (main)
- Try to recover from errors, but if you can't recover properly, exit the whole app
- Retry logic with exponential backoff (3 attempts, 2-4-6 second delays)
- Safe parsing with fallback to zero values for malformed data
- Use proper error types, not generic strings

## Development Setup

**Important:** This project uses Go 1.19+ with specific dependencies for
Hyperliquid API integration.

### Configuration Setup

**TOML Configuration (Recommended):**
```bash
# Copy and edit configuration
cp config.toml.example config.toml
# Edit config.toml with your credentials and settings
```

**Environment Variables (Legacy):**
```bash
export HYPERLIQUID_TARGET_ACCOUNT="0x..."
export HYPERLIQUID_API_KEY="your_api_key"
export HYPERLIQUID_PRIVATE_KEY="your_private_key_hex"
export HYPERLIQUID_USE_TESTNET="true"
export HYPERLIQUID_COPY_THRESHOLD="1000.0"
```

## Build Commands

```bash
# Prepare dependencies (always run first)
make prepare

# Build for development
make build

# Run benchmarks
go test -bench=. src/...

# Test with coverage
go test -cover src/...

# Clean build artifacts
make clean

# Build Docker image
make image

# Run the bot (after building)
./main
```

## Architecture Overview

The codebase consists of modular Go files:

1. **main.go**: Entry point with signal handling and bot lifecycle
2. **config.go**: TOML configuration with environment variable fallback
3. **bot.go**: Core monitoring logic with retry mechanisms
4. **client.go**: Hyperliquid API client with authentication scaffolding
5. **paper_trading.go**: Sophisticated position management and PnL calculations
6. **test.go**: Basic connectivity testing

## Data Flow

```
API Polling (5s) -> Fill Detection -> Duplicate Check -> Threshold Filter ->
Position Update -> PnL Calculation -> Trade Record
```

### Key Functions

- `loadConfig()`: TOML-first configuration loading with env var fallback
- `ProcessFill()`: Core position management with action determination
- `determineAction()`: State machine for position lifecycle (OPEN/ADD/REDUCE/CLOSE/REVERSE)
- `calculateRealizedPnL()`: FIFO-based PnL calculation on position reductions
- `updatePosition()`: Volume-weighted average price and cost basis management
- `checkTrades()`: Resilient API polling with exponential backoff

### Architecture Decisions

1. **Hash-based Duplicate Detection**: Prevents reprocessing same fills across restarts
2. **Volume-Weighted Average Pricing**: Accurate entry prices for complex position building
3. **Position Action Classification**: Clear state machine for all trading scenarios
4. **TOML Configuration**: Structured config with backwards compatibility
5. **Comprehensive Testing**: Unit tests, integration tests, stress tests, benchmarks
6. **Paper Trading Only**: Safe by design, real trading as future enhancement
7. **Error Resilience**: Continue operation despite individual failures

## Important Notes

- Hyperliquid API calls can be rate-limited - respect exponential backoff
- Position reversals require careful handling of cost basis reset
- Floating-point precision matters for large position sizes
- Concurrent access to position maps requires careful consideration
- Default to testnet for safety (HYPERLIQUID_USE_TESTNET=true)
- Never commit binaries (main) - use make clean before commits
- Docker base should be Debian (avoid Alpine compatibility issues)
- Keep Makefile targets minimal - only essential commands
- TOML configuration preferred over environment variables for complex setups

## Technical Wisdom

**Position Management Complexity:**
- OPEN: Flat to Long/Short (new position creation)
- ADD: Same direction, increase size (weighted average recalculation)
- REDUCE: Same direction, decrease size (realized PnL calculation)
- CLOSE: Position to Flat (final PnL realization)
- REVERSE: Long to Short or Short to Long (most complex scenario)

**Error Scenarios to Handle:**
- Malformed PnL strings from API (fallback to 0.0)
- Network timeouts and rate limiting (retry with backoff)
- Invalid position sizes (NaN/Inf detection)
- Concurrent access to shared state (careful map usage)
- Memory leaks in long-running sessions (proper cleanup)

**Testing Philosophy:**
- Unit tests for core logic (position management, PnL calculations)
- Integration tests with mock HTTP servers
- Stress tests for edge cases (extreme values, concurrency)
- Benchmark tests for performance optimization
- Comprehensive coverage including error paths

**Test Files Structure:**
- `src/paper_trading_test.go`: Core position management and PnL logic
- `src/bot_test.go`: Bot integration and configuration tests
- `src/stress_test.go`: Performance, concurrency, and edge case tests

## Quick Development Reference

**Configuration Priority:** TOML config file > Environment variables > Defaults

**Key State Management:**
- `processedFills` map prevents duplicate processing across restarts
- Position state tracked in `PositionInfo` with cost basis
- Session state in `PaperTradingSession` for portfolio tracking

**Adding New Trading Logic:**
1. Implement in `src/paper_trading.go` following existing patterns
2. Add unit tests in `src/paper_trading_test.go`
3. Test edge cases in `src/stress_test.go`
4. Ensure proper error handling and logging

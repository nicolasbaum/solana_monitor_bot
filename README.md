# Solana Transaction Monitor

Go service that monitors USDC transactions on the Solana blockchain and sends notifications via Telegram.

## Commands

- `/start` - Start the bot and see available commands
- `/add <address>` - Add a wallet address to monitor
- `/list` - List all monitored addresses
- `/clear` - Remove all monitored addresses
- `/help` - Show help message

## Configuration

Create a `.env` file in the root directory with the following variables:

```env
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
SOLANA_RPC_ENDPOINT=your_solana_rpc_endpoint
USDC_THRESHOLD=1000  # Minimum USDC amount to trigger notifications (default: 1000)
```

## Rate Limiting

- Token bucket algorithm with 10 requests/second rate limit
- Burst capacity of 15 requests for handling spikes
- Minimum 100ms spacing between requests
- Exponential backoff for rate limit errors (5s initial, up to 2min max)
- Smart request batching and caching

## Transaction Handling

- Support for versioned transactions
- Proper handling of transaction metadata
- Extraction of from/to addresses
- USDC amount calculation
- Graceful handling of unsupported transaction versions

## Logging

All logs are centralized in the `logs` directory:

- `logs/app.log` - Main application logs
- `logs/monitor.log` - Transaction monitoring logs
- `logs/telegram.log` - Telegram bot interaction logs
- `logs/solana-rpc.log` - Solana RPC client logs

## Installation

1. Clone the repository
2. Create and configure the `.env` file (See `.env.example`)
3. Run:
```bash
make build
```

## Usage

```bash
make run
```

## Requirements

- Go 1.21 or higher
- Access to a Solana RPC endpoint
- Telegram Bot Token
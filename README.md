# Solana Transaction Monitor

A Go-based service that monitors Solana transactions for large USDC transfers and sends alerts via Telegram.

## Features

- 🔍 Monitors Solana transactions for specific addresses
- 💰 Detects USDC transfers exceeding a configurable threshold
- 🚨 Sends real-time alerts via Telegram
- 🔗 Provides transaction links to Solscan
- ⚡ Efficient and concurrent monitoring

## Prerequisites

- Go 1.19 or higher
- Telegram Bot Token (from [@BotFather](https://t.me/botfather))
- Solana RPC endpoint (optional, defaults to public mainnet-beta)

## Configuration

The service is configured using environment variables, which can be set in a `.env` file:

| Variable              | Description                             | Required | Default                             |
| --------------------- | --------------------------------------- | -------- | ----------------------------------- |
| `TELEGRAM_BOT_TOKEN`  | Your Telegram bot token from @BotFather | Yes      | -                                   |
| `SOLANA_RPC_ENDPOINT` | Solana RPC endpoint URL                 | No       | https://api.mainnet-beta.solana.com |
| `USDC_THRESHOLD`      | Minimum USDC amount to trigger alerts   | No       | 1000                                |
| `LOG_LEVEL`           | Logging verbosity (debug, info, error)  | No       | info                                |

You can copy the `.env.example` file and modify it with your settings:
```bash
cp .env.example .env
```

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd solana-monitor
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file:
```env
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
SOLANA_RPC_ENDPOINT=https://api.mainnet-beta.solana.com
USDC_THRESHOLD=1000.0  # Optional: Set custom threshold for USDC transfers (default: 1000.0)
```

## Building

```bash
go build -o monitor ./cmd/monitor
```

## Running

```bash
./monitor
```

## Usage

1. Start a chat with your Telegram bot
2. Use the following commands:
   - `/start` - Start monitoring
   - `/help` - Show available commands

## Architecture

The service consists of several components:

1. **Solana Client**: Connects to Solana RPC and monitors transactions
2. **Telegram Bot**: Handles user interaction and sends alerts
3. **Monitor Service**: Coordinates between components and manages state
4. **Configuration**: Handles environment variables and settings

## Development

To run in development mode with hot reloading:

```bash
go install github.com/cosmtrek/air@latest
air
```

## Security Considerations

- Never share your Telegram bot token
- Use a secure RPC endpoint
- Monitor resource usage when tracking many addresses
- Consider rate limiting for public RPC endpoints

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details. 
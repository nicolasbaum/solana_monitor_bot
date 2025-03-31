package telegram

import (
	"fmt"
	"log"
	"strings"

	"solana-monitor/internal/config"
	"solana-monitor/internal/logging"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BotAPIInterface defines the interface for the Telegram Bot API
type BotAPIInterface interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
}

// BotAPIFactory is a function type that creates a new BotAPI instance
type BotAPIFactory func(token string) (BotAPIInterface, error)

// DefaultBotAPIFactory is the default factory function that creates a real BotAPI instance
func DefaultBotAPIFactory(token string) (BotAPIInterface, error) {
	return tgbotapi.NewBotAPI(token)
}

// Bot represents a Telegram bot instance
type Bot struct {
	api            BotAPIInterface
	addressHandler func(chatID int64, address string)
	getAddresses   func(chatID int64) []string
	logger         *log.Logger
	config         *config.Config
}

// NewBot creates a new Telegram bot instance
func NewBot(token string, addressHandler func(chatID int64, address string), getAddresses func(chatID int64) []string, config *config.Config) (*Bot, error) {
	return NewBotWithFactory(token, addressHandler, getAddresses, DefaultBotAPIFactory, config)
}

// NewBotWithFactory creates a new Telegram bot instance using a custom BotAPI factory
func NewBotWithFactory(token string, addressHandler func(chatID int64, address string), getAddresses func(chatID int64) []string, factory BotAPIFactory, config *config.Config) (*Bot, error) {
	api, err := factory(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %v", err)
	}

	// Create a logger that writes to a file with timestamp
	logFile, err := logging.CreateLogFile("telegram_bot.log")
	if err != nil {
		log.Printf("Failed to open log file: %v, falling back to stdout", err)
		return nil, fmt.Errorf("failed to create log file: %v", err)
	}

	logger := log.New(logging.CreateMultiWriter(logFile), "[TELEGRAM] ", log.LstdFlags)
	logger.Printf("Initializing Telegram bot...")

	return &Bot{
		api:            api,
		addressHandler: addressHandler,
		getAddresses:   getAddresses,
		logger:         logger,
		config:         config,
	}, nil
}

// SendAlert sends a transaction alert to the specified chat ID
func (b *Bot) SendAlert(chatID int64, message string) error {
	b.logger.Printf("Sending alert to chat %d", chatID)

	msg := tgbotapi.NewMessage(chatID, message)
	_, err := b.api.Send(msg)
	if err != nil {
		b.logger.Printf("Failed to send alert to chat %d: %v", chatID, err)
		return fmt.Errorf("failed to send alert: %v", err)
	}

	b.logger.Printf("Successfully sent alert to chat %d", chatID)
	return nil
}

// Start starts listening for incoming messages
func (b *Bot) Start() {
	b.logger.Printf("Starting Telegram bot...")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Handle commands
		if update.Message.IsCommand() {
			chatID := update.Message.Chat.ID
			b.logger.Printf("Received command '%s' from chat %d", update.Message.Command(), chatID)

			switch update.Message.Command() {
			case "start":
				msg := tgbotapi.NewMessage(chatID,
					"[Welcome] Welcome to the Solana Transaction Monitor!\n\n"+
						"I'll notify you about large USDC transfers on Solana.\n"+
						"Use /help to see available commands.")
				b.api.Send(msg)

			case "help":
				msg := tgbotapi.NewMessage(chatID,
					"Available commands:\n\n"+
						"/start - Start monitoring\n"+
						"/watch <address> - Add a Solana address to monitor\n"+
						"/list - List currently monitored addresses\n"+
						"/help - Show this help message\n\n"+
						"Example:\n"+
						"/watch FMvbLJC5bZtik6WqMz7kzQYzJXEqyWHkQzpqGxgMozS2")
				b.api.Send(msg)

			case "watch":
				// Extract address from command
				args := strings.Fields(update.Message.Text)
				if len(args) != 2 {
					msg := tgbotapi.NewMessage(chatID,
						"[Error] Please provide a Solana address to monitor.\n\n"+
							"Example:\n"+
							"/watch FMvbLJC5bZtik6WqMz7kzQYzJXEqyWHkQzpqGxgMozS2")
					b.api.Send(msg)
					continue
				}

				address := args[1]
				b.logger.Printf("Adding address %s for chat %d", address, chatID)

				// Call the address handler
				b.addressHandler(chatID, address)

				msg := tgbotapi.NewMessage(chatID,
					fmt.Sprintf("[Success] Now monitoring address:\n%s\n\n"+
						"You'll receive alerts for USDC transfers exceeding %.2f USDC.", address, b.config.USDCThreshold))
				b.api.Send(msg)

			case "list":
				addresses := b.getAddresses(chatID)
				b.logger.Printf("Retrieved %d addresses for chat %d", len(addresses), chatID)

				var message string
				if len(addresses) == 0 {
					message = "[Info] You are not monitoring any addresses.\n\nUse /watch to add an address."
				} else {
					message = "[List] Currently monitored addresses:\n\n"
					for i, addr := range addresses {
						message += fmt.Sprintf("%d. `%s`\n", i+1, addr)
					}
					message += "\nUse /watch to add more addresses."
				}

				msg := tgbotapi.NewMessage(chatID, message)
				msg.ParseMode = "Markdown" // Enable markdown for code formatting
				b.api.Send(msg)
			}
		}
	}
}

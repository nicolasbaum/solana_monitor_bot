package telegram

import (
	"fmt"
	"io"
	"log"
	"os"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"solana-monitor/internal/config"
)

// MockBotAPI is a mock implementation of the Telegram bot API
type MockBotAPI struct {
	mock.Mock
}

func (m *MockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

func (m *MockBotAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.UpdatesChannel)
}

func TestNewBot(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		TelegramToken:     "test-token",
		SolanaRPCEndpoint: "https://test-endpoint.com",
		USDCThreshold:     1000.0,
	}

	// Create mock API
	mockAPI := new(MockBotAPI)

	// Create mock factory that returns our mock API
	mockFactory := func(token string) (BotAPIInterface, error) {
		return mockAPI, nil
	}

	// Create mock address handler and getter
	addressHandler := func(chatID int64, address string) {}
	getAddresses := func(chatID int64) []string { return nil }

	// Create bot with mock factory
	bot, err := NewBotWithFactory("test-token", addressHandler, getAddresses, mockFactory, cfg)

	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, bot)
	assert.Equal(t, cfg, bot.config)
}

func TestSendAlert(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		TelegramToken:     "test-token",
		SolanaRPCEndpoint: "https://test-endpoint.com",
		USDCThreshold:     1000.0,
	}

	// Create mock bot API
	mockAPI := new(MockBotAPI)

	// Set up expectations for Send
	expectedMsg := tgbotapi.NewMessage(123456, "[ALERT] Large USDC Transfer Detected!\n\n"+
		"Amount: 2000.00 USDC\n"+
		"From: test-from\n"+
		"To: test-to\n\n"+
		"View on Solscan: https://solscan.io/tx/test-signature")

	mockAPI.On("Send", expectedMsg).Return(tgbotapi.Message{}, nil)

	// Create bot with mock API
	bot := &Bot{
		api:    mockAPI,
		logger: log.New(os.Stdout, "", log.LstdFlags),
		config: cfg,
	}

	// Send alert
	message := fmt.Sprintf("[ALERT] Large USDC Transfer Detected!\n\n"+
		"Amount: %.2f USDC\n"+
		"From: %s\n"+
		"To: %s\n\n"+
		"View on Solscan: https://solscan.io/tx/%s",
		2000.0, "test-from", "test-to", "test-signature")
	err := bot.SendAlert(123456, message)

	// Assert results
	assert.NoError(t, err)
	mockAPI.AssertExpectations(t)
}

// NewTestLogger creates a logger for testing
func NewTestLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

package telegram

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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
	return args.Get(0).(chan tgbotapi.Update)
}

func TestNewBot(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		TelegramToken:     "test-token",
		SolanaRPCEndpoint: "https://test-endpoint.com",
	}

	// Create mock API
	mockAPI := new(MockBotAPI)

	// Create mock factory that returns our mock API
	mockFactory := func(token string) (BotAPIInterface, error) {
		return mockAPI, nil
	}

	// Create mock handlers
	addressHandler := func(chatID int64, address string) {}
	getAddresses := func(chatID int64) []string { return nil }
	clearAddresses := func(chatID int64) {}

	// Create bot with mock factory
	bot, err := NewBotWithFactory("test-token", addressHandler, getAddresses, clearAddresses, mockFactory, cfg)

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

func TestClearCommand(t *testing.T) {
	// Create mocks
	mockAPI := &MockBotAPI{}
	mockAddressHandler := func(chatID int64, address string) {}
	mockGetAddresses := func(chatID int64) []string {
		return []string{"addr1", "addr2"}
	}
	clearCalled := false
	mockClearAddresses := func(chatID int64) {
		clearCalled = true
	}

	// Create bot
	bot, err := NewBotWithFactory(
		"test-token",
		mockAddressHandler,
		mockGetAddresses,
		mockClearAddresses,
		func(token string) (BotAPIInterface, error) {
			return mockAPI, nil
		},
		&config.Config{},
	)
	require.NoError(t, err)
	require.NotNil(t, bot)

	// Set up update channel
	updates := make(chan tgbotapi.Update, 1)
	mockAPI.On("GetUpdatesChan", mock.Anything).Return(updates)

	// Test clear command with addresses
	chatID := int64(12345)
	mockAPI.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
		return msg.ChatID == chatID && strings.Contains(msg.Text, "[Success] All monitored addresses have been removed")
	})).Return(tgbotapi.Message{}, nil).Once()

	// Start bot in background
	go bot.Start()

	// Send clear command
	updates <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{
				ID: chatID,
			},
			Text: "/clear",
			Entities: []tgbotapi.MessageEntity{
				{
					Type:   "bot_command",
					Offset: 0,
					Length: 6,
				},
			},
		},
	}

	// Allow time for processing
	time.Sleep(100 * time.Millisecond)

	// Verify expectations
	mockAPI.AssertExpectations(t)
	require.True(t, clearCalled, "clearAddresses should have been called")

	// Test clear command with no addresses
	mockGetAddresses = func(chatID int64) []string {
		return nil
	}
	bot.getAddresses = mockGetAddresses

	mockAPI.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
		return msg.ChatID == chatID && strings.Contains(msg.Text, "[Info] You are not monitoring any addresses")
	})).Return(tgbotapi.Message{}, nil).Once()

	// Send clear command again
	updates <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{
				ID: chatID,
			},
			Text: "/clear",
			Entities: []tgbotapi.MessageEntity{
				{
					Type:   "bot_command",
					Offset: 0,
					Length: 6,
				},
			},
		},
	}

	// Allow time for processing
	time.Sleep(100 * time.Millisecond)

	// Verify expectations
	mockAPI.AssertExpectations(t)

	// Close update channel
	close(updates)
}

// NewTestLogger creates a logger for testing
func NewTestLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

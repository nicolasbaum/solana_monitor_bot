package monitor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"solana-monitor/internal/config"
	"solana-monitor/internal/solana"
)

// MockSolanaClient is a mock implementation of the Solana client
type MockSolanaClient struct {
	mock.Mock
}

func (m *MockSolanaClient) MonitorTransactions(ctx context.Context, addresses []string, threshold float64) (<-chan solana.Transaction, error) {
	args := m.Called(ctx, addresses, threshold)
	return args.Get(0).(<-chan solana.Transaction), args.Error(1)
}

// MockTelegramBot is a mock implementation of the Telegram bot
type MockTelegramBot struct {
	mock.Mock
}

func (m *MockTelegramBot) SendAlert(chatID int64, message string) error {
	args := m.Called(chatID, message)
	return args.Error(0)
}

func TestNewService(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		SolanaRPCEndpoint: "https://test-endpoint.com",
		USDCThreshold:     1000.0,
	}

	// Create mock Solana client
	mockClient := new(MockSolanaClient)

	// Create service
	service := NewService(mockClient, cfg)

	// Assert service is properly initialized
	assert.NotNil(t, service)
	assert.Equal(t, mockClient, service.solanaClient)
	assert.Equal(t, cfg, service.config)
	assert.NotNil(t, service.addresses)
	assert.NotNil(t, service.monitors)
	assert.NotNil(t, service.logger)
}

func TestAddAddress(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		SolanaRPCEndpoint: "https://test-endpoint.com",
		USDCThreshold:     1000.0,
	}

	// Create mock Solana client
	mockClient := new(MockSolanaClient)

	// Create mock transaction channel
	txChan := make(chan solana.Transaction)

	// Set up expectations for MonitorTransactions
	mockClient.On("MonitorTransactions", mock.Anything, []string{"test-address"}, float64(1000.0)).
		Return((<-chan solana.Transaction)(txChan), nil)

	// Create service
	service := NewService(mockClient, cfg)

	// Add address
	service.AddAddress(123456, "test-address")

	// Verify address was added
	addresses := service.GetMonitoredAddresses(123456)
	assert.Equal(t, 1, len(addresses))
	assert.Equal(t, "test-address", addresses[0])

	// Add same address again
	service.AddAddress(123456, "test-address")

	// Verify no duplicate was added
	addresses = service.GetMonitoredAddresses(123456)
	assert.Equal(t, 1, len(addresses))
	assert.Equal(t, "test-address", addresses[0])

	// Clean up
	close(txChan)
}

func TestRemoveAddress(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		SolanaRPCEndpoint: "https://test-endpoint.com",
		USDCThreshold:     1000.0,
	}

	// Create mock Solana client
	mockClient := new(MockSolanaClient)

	// Create mock transaction channel
	txChan := make(chan solana.Transaction)

	// Set up expectations for MonitorTransactions
	mockClient.On("MonitorTransactions", mock.Anything, []string{"test-address"}, float64(1000.0)).
		Return((<-chan solana.Transaction)(txChan), nil)

	// Create service
	service := NewService(mockClient, cfg)

	// Add address
	service.AddAddress(123456, "test-address")

	// Verify address was added
	addresses := service.GetMonitoredAddresses(123456)
	assert.Equal(t, 1, len(addresses))

	// Remove address
	service.RemoveAddress(123456, "test-address")

	// Verify address was removed
	addresses = service.GetMonitoredAddresses(123456)
	assert.Equal(t, 0, len(addresses))

	// Clean up
	close(txChan)
}

func TestMonitorTransactions(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		SolanaRPCEndpoint: "https://test-endpoint.com",
		USDCThreshold:     1000.0,
	}

	// Create mock Solana client
	mockClient := new(MockSolanaClient)

	// Create mock Telegram bot
	mockBot := new(MockTelegramBot)

	// Create mock transaction channel
	txChan := make(chan solana.Transaction)

	// Set up expectations for MonitorTransactions
	mockClient.On("MonitorTransactions", mock.Anything, []string{"test-address"}, float64(1000.0)).
		Return((<-chan solana.Transaction)(txChan), nil)

	// Set up expectations for SendAlert
	expectedMessage := fmt.Sprintf("[ALERT] Large USDC Transfer Detected!\n\n"+
		"Amount: %.2f USDC\n"+
		"From: %s\n"+
		"To: %s\n\n"+
		"View on Solscan: https://solscan.io/tx/%s",
		2000.0, "from-address", "to-address", "test-signature")
	mockBot.On("SendAlert", int64(123456), expectedMessage).Return(nil)

	// Create service
	service := NewService(mockClient, cfg)
	service.SetBot(mockBot)

	// Add address
	service.AddAddress(123456, "test-address")

	// Create test transaction
	testTx := solana.Transaction{
		Signature:  "test-signature",
		Timestamp:  time.Now(),
		USDCAmount: 2000.0,
		FromAddr:   "from-address",
		ToAddr:     "to-address",
	}

	// Start service in background
	ctx, cancel := context.WithCancel(context.Background())
	go service.Start(ctx)

	// Send test transaction
	txChan <- testTx

	// Wait for transaction to be processed
	time.Sleep(100 * time.Millisecond)

	// Clean up
	cancel()
	close(txChan)

	// Verify expectations
	mockClient.AssertExpectations(t)
	mockBot.AssertExpectations(t)
}

func TestGetMonitoredAddresses(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		SolanaRPCEndpoint: "https://test-endpoint.com",
		USDCThreshold:     1000.0,
	}

	// Create mock Solana client
	mockClient := new(MockSolanaClient)

	// Create mock transaction channel
	txChan := make(chan solana.Transaction)

	// Set up expectations for MonitorTransactions
	mockClient.On("MonitorTransactions", mock.Anything, []string{"address1"}, float64(1000.0)).
		Return((<-chan solana.Transaction)(txChan), nil)
	mockClient.On("MonitorTransactions", mock.Anything, []string{"address1", "address2"}, float64(1000.0)).
		Return((<-chan solana.Transaction)(txChan), nil)

	// Create service
	service := NewService(mockClient, cfg)

	// Test empty list
	addresses := service.GetMonitoredAddresses(123456)
	assert.Nil(t, addresses)

	// Add addresses
	service.AddAddress(123456, "address1")
	service.AddAddress(123456, "address2")

	// Test getting addresses
	addresses = service.GetMonitoredAddresses(123456)
	assert.Equal(t, 2, len(addresses))
	assert.Contains(t, addresses, "address1")
	assert.Contains(t, addresses, "address2")

	// Test getting addresses for different chat ID
	addresses = service.GetMonitoredAddresses(789012)
	assert.Nil(t, addresses)

	// Clean up
	close(txChan)
}

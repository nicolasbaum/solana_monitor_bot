package monitor

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"solana-monitor/internal/config"
	"solana-monitor/internal/solana"
)

// MockSolanaClient is a mock implementation of the SolanaClientInterface
type MockSolanaClient struct {
	mock.Mock
}

func (m *MockSolanaClient) MonitorTransactions(ctx context.Context, addresses []string, threshold float64) (<-chan solana.Transaction, error) {
	args := m.Called(ctx, addresses, threshold)
	ch := make(chan solana.Transaction)
	close(ch) // Close immediately since we don't need to send any transactions in tests
	return ch, args.Error(0)
}

// MockBot is a mock implementation of the TelegramBotInterface
type MockBot struct {
	mock.Mock
}

func (m *MockBot) SendAlert(chatID int64, message string) error {
	args := m.Called(chatID, message)
	return args.Error(0)
}

func TestNewService(t *testing.T) {
	// Set test environment variables
	os.Setenv("USDC_THRESHOLD", "2500.5")
	defer os.Clearenv()

	cfg := &config.Config{
		SolanaRPCEndpoint: "https://api.mainnet-beta.solana.com",
		TelegramToken:     "test-token",
	}

	mockClient := new(MockSolanaClient)
	service := NewService(mockClient, cfg)

	assert.NotNil(t, service)
	assert.Equal(t, mockClient, service.solanaClient)
	assert.Equal(t, cfg, service.config)
	assert.NotNil(t, service.rules)
	assert.Len(t, service.rules, 1) // Verify we have one rule (USDC threshold)
}

func TestAddAddress(t *testing.T) {
	// Set test environment variables
	os.Setenv("USDC_THRESHOLD", "2500.5")
	defer os.Clearenv()

	cfg := &config.Config{
		SolanaRPCEndpoint: "https://api.mainnet-beta.solana.com",
		TelegramToken:     "test-token",
	}

	mockClient := new(MockSolanaClient)
	service := NewService(mockClient, cfg)

	// Set up expectations for monitoring with the threshold from environment variable
	mockClient.On("MonitorTransactions", mock.Anything, []string{"TestAddress1"}, 2500.5).Return(nil, nil)

	// Test adding an address
	chatID := int64(123)
	address := "TestAddress1"

	// Add address
	service.AddAddress(chatID, address)

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Verify the address is being monitored
	service.mu.RLock()
	addresses, exists := service.addresses[chatID]
	service.mu.RUnlock()

	assert.True(t, exists)
	assert.Contains(t, addresses, address)

	// Clean up
	if cancel, exists := service.monitors[chatID]; exists {
		cancel()
	}

	// Give the goroutine time to stop
	time.Sleep(10 * time.Millisecond)

	// Verify expectations
	mockClient.AssertExpectations(t)
}

func TestRemoveAddress(t *testing.T) {
	// Set test environment variables
	os.Setenv("USDC_THRESHOLD", "2500.5")
	defer os.Clearenv()

	cfg := &config.Config{
		SolanaRPCEndpoint: "https://api.mainnet-beta.solana.com",
		TelegramToken:     "test-token",
	}

	mockClient := new(MockSolanaClient)
	service := NewService(mockClient, cfg)

	// Set up expectations for monitoring with the threshold from environment variable
	mockClient.On("MonitorTransactions", mock.Anything, []string{"TestAddress1"}, 2500.5).Return(nil, nil)

	// Test adding and removing an address
	chatID := int64(123)
	address := "TestAddress1"

	// First add the address
	service.AddAddress(chatID, address)

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Then remove it
	service.RemoveAddress(chatID, address)

	// Verify the address is no longer being monitored
	service.mu.RLock()
	addresses, exists := service.addresses[chatID]
	service.mu.RUnlock()

	assert.True(t, exists)
	assert.Empty(t, addresses)

	// Clean up
	if cancel, exists := service.monitors[chatID]; exists {
		cancel()
	}

	// Give the goroutine time to stop
	time.Sleep(10 * time.Millisecond)

	// Verify expectations
	mockClient.AssertExpectations(t)
}

func TestSetBot(t *testing.T) {
	// Set test environment variables
	os.Setenv("USDC_THRESHOLD", "2500.5")
	defer os.Clearenv()

	cfg := &config.Config{
		SolanaRPCEndpoint: "https://api.mainnet-beta.solana.com",
		TelegramToken:     "test-token",
	}

	mockClient := new(MockSolanaClient)
	mockBot := new(MockBot)
	service := NewService(mockClient, cfg)

	// Set the bot
	service.SetBot(mockBot)

	// Verify the bot is set
	assert.Equal(t, mockBot, service.bot)
}

func TestStart(t *testing.T) {
	// Set test environment variables
	os.Setenv("USDC_THRESHOLD", "2500.5")
	defer os.Clearenv()

	cfg := &config.Config{
		SolanaRPCEndpoint: "https://api.mainnet-beta.solana.com",
		TelegramToken:     "test-token",
	}

	mockClient := new(MockSolanaClient)
	service := NewService(mockClient, cfg)

	// Set up expectations for monitoring with the threshold from environment variable
	mockClient.On("MonitorTransactions", mock.Anything, []string{"TestAddress1"}, 2500.5).Return(nil, nil)
	mockClient.On("MonitorTransactions", mock.Anything, []string{"TestAddress1", "TestAddress2"}, 2500.5).Return(nil, nil)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start service in background
	go func() {
		err := service.Start(ctx)
		assert.NoError(t, err)
	}()

	// Add some addresses to monitor
	service.AddAddress(123, "TestAddress1")
	time.Sleep(10 * time.Millisecond) // Give first monitor time to start
	service.AddAddress(123, "TestAddress2")
	time.Sleep(10 * time.Millisecond) // Give second monitor time to start

	// Wait for service to stop
	<-ctx.Done()

	// Clean up
	if cancel, exists := service.monitors[123]; exists {
		cancel()
	}

	// Give the goroutine time to stop
	time.Sleep(10 * time.Millisecond)

	// Verify expectations
	mockClient.AssertExpectations(t)
}

func TestGetMonitoredAddresses(t *testing.T) {
	// Set test environment variables
	os.Setenv("USDC_THRESHOLD", "2500.5")
	defer os.Clearenv()

	cfg := &config.Config{
		SolanaRPCEndpoint: "https://api.mainnet-beta.solana.com",
		TelegramToken:     "test-token",
	}

	mockClient := new(MockSolanaClient)
	service := NewService(mockClient, cfg)

	// Set up expectations for monitoring with the threshold from environment variable
	mockClient.On("MonitorTransactions", mock.Anything, []string{"address1"}, 2500.5).Return(nil, nil)
	mockClient.On("MonitorTransactions", mock.Anything, []string{"address1", "address2"}, 2500.5).Return(nil, nil)

	// Test empty list
	addresses := service.GetMonitoredAddresses(123456)
	assert.Nil(t, addresses)

	// Add addresses
	service.AddAddress(123456, "address1")
	time.Sleep(10 * time.Millisecond) // Give first monitor time to start
	service.AddAddress(123456, "address2")
	time.Sleep(10 * time.Millisecond) // Give second monitor time to start

	// Test getting addresses
	addresses = service.GetMonitoredAddresses(123456)
	assert.Equal(t, 2, len(addresses))
	assert.Contains(t, addresses, "address1")
	assert.Contains(t, addresses, "address2")

	// Test getting addresses for different chat ID
	addresses = service.GetMonitoredAddresses(789012)
	assert.Nil(t, addresses)

	// Clean up
	if cancel, exists := service.monitors[123456]; exists {
		cancel()
	}

	// Give the goroutine time to stop
	time.Sleep(10 * time.Millisecond)

	// Verify expectations
	mockClient.AssertExpectations(t)
}

func TestClearAddresses(t *testing.T) {
	// Create mock client and config
	mockClient := &MockSolanaClient{}
	cfg := &config.Config{
		SolanaRPCEndpoint: "test-endpoint",
	}

	// Set up mock expectations
	mockClient.On("MonitorTransactions", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Create service
	service := NewService(mockClient, cfg)
	require.NotNil(t, service)

	// Add some test addresses
	chatID := int64(12345)
	testAddrs := []string{
		"addr1",
		"addr2",
		"addr3",
	}

	// Add addresses to monitor
	for _, addr := range testAddrs {
		service.AddAddress(chatID, addr)
	}

	// Verify addresses were added
	addrs := service.GetMonitoredAddresses(chatID)
	require.Equal(t, len(testAddrs), len(addrs))

	// Clear all addresses
	service.ClearAddresses(chatID)

	// Verify addresses were cleared
	addrs = service.GetMonitoredAddresses(chatID)
	require.Nil(t, addrs)

	// Verify monitor was cancelled
	_, exists := service.monitors[chatID]
	require.False(t, exists)

	// Allow time for goroutines to complete
	time.Sleep(100 * time.Millisecond)

	// Verify mock expectations
	mockClient.AssertExpectations(t)
}

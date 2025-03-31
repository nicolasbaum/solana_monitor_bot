package monitor

import (
	"context"
	"log"
	"sync"
	"time"

	"solana-monitor/internal/config"
	"solana-monitor/internal/logging"
	"solana-monitor/internal/solana"
)

// SolanaClientInterface defines the interface for the Solana client
type SolanaClientInterface interface {
	MonitorTransactions(ctx context.Context, addresses []string, threshold float64) (<-chan solana.Transaction, error)
}

// TelegramBotInterface defines the interface for the Telegram bot
type TelegramBotInterface interface {
	SendAlert(chatID int64, message string) error
}

// Service represents the monitoring service
type Service struct {
	solanaClient SolanaClientInterface
	bot          TelegramBotInterface
	config       *config.Config
	addresses    map[int64][]string // Map of chat ID to monitored addresses
	mu           sync.RWMutex
	monitors     map[int64]context.CancelFunc // Map of chat ID to monitor cancel functions
	logger       *log.Logger
	rules        []Rule // List of rules to apply to transactions
}

// NewService creates a new monitoring service
func NewService(solanaClient SolanaClientInterface, cfg *config.Config) *Service {
	// Create service logger
	logFile, err := logging.CreateLogFile("monitor_service.log")
	if err != nil {
		log.Printf("Failed to create log file, using default logger")
		return nil
	}

	logger := log.New(logging.CreateMultiWriter(logFile), "[MONITOR] ", log.LstdFlags)

	logger.Printf("[INIT] Initializing monitoring service...")
	logger.Printf("[CONFIG] RPC Endpoint=%s, USDC Threshold=%.2f",
		cfg.SolanaRPCEndpoint, cfg.USDCThreshold)

	// Create default USDC threshold rule
	usdcRule := NewUSDCThresholdRule(cfg.USDCThreshold)

	return &Service{
		solanaClient: solanaClient,
		config:       cfg,
		addresses:    make(map[int64][]string),
		monitors:     make(map[int64]context.CancelFunc),
		logger:       logger,
		rules:        []Rule{usdcRule},
	}
}

// SetBot sets the Telegram bot for the service
func (s *Service) SetBot(bot TelegramBotInterface) {
	s.bot = bot
	s.logger.Printf("[SUCCESS] Telegram bot configured and ready")
}

// Start starts the monitoring service
func (s *Service) Start(ctx context.Context) error {
	s.logger.Printf("[STARTUP] Starting monitoring service...")

	// Log current state
	s.mu.RLock()
	if len(s.addresses) == 0 {
		s.logger.Printf("[INFO] No addresses currently being monitored")
	} else {
		for chatID, addrs := range s.addresses {
			s.logger.Printf("📋 Chat %d monitoring addresses: %v", chatID, addrs)
		}
	}
	s.mu.RUnlock()

	// Wait for context cancellation
	<-ctx.Done()

	s.logger.Printf("[SHUTDOWN] Shutting down monitoring service...")

	// Cancel all monitors
	s.mu.Lock()
	for chatID, cancel := range s.monitors {
		s.logger.Printf("[SHUTDOWN] Stopping monitor for chat %d", chatID)
		cancel()
	}
	s.mu.Unlock()

	s.logger.Printf("[SUCCESS] Monitoring service shutdown complete")
	return nil
}

// AddAddress adds an address to monitor for a specific chat ID
func (s *Service) AddAddress(chatID int64, address string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Printf("[ADD] Adding address %s for chat %d", address, chatID)

	// Initialize address list if it doesn't exist
	if _, exists := s.addresses[chatID]; !exists {
		s.logger.Printf("[INIT] Initializing address list for chat %d", chatID)
		s.addresses[chatID] = make([]string, 0)
	}

	// Add address if it's not already being monitored
	found := false
	for _, addr := range s.addresses[chatID] {
		if addr == address {
			found = true
			s.logger.Printf("[INFO] Address %s is already being monitored for chat %d", address, chatID)
			break
		}
	}

	if !found {
		s.addresses[chatID] = append(s.addresses[chatID], address)
		s.logger.Printf("[SUCCESS] Added new address %s for chat %d", address, chatID)

		// Cancel existing monitor if any
		if cancel, exists := s.monitors[chatID]; exists {
			s.logger.Printf("[SHUTDOWN] Cancelling existing monitor for chat %d", chatID)
			cancel()
		}

		// Start new monitor for the updated address list
		ctx, cancel := context.WithCancel(context.Background())
		s.monitors[chatID] = cancel
		s.logger.Printf("[STARTUP] Starting new monitor for chat %d with addresses: %v", chatID, s.addresses[chatID])
		go s.monitorAddresses(ctx, chatID, s.addresses[chatID])
	}
}

// monitorAddresses monitors transactions for a specific chat ID
func (s *Service) monitorAddresses(ctx context.Context, chatID int64, addresses []string) {
	s.logger.Printf("Starting transaction monitoring for chat %d, addresses: %v", chatID, addresses)

	txChan, err := s.solanaClient.MonitorTransactions(ctx, addresses, s.config.USDCThreshold)
	if err != nil {
		s.logger.Printf("Failed to start monitoring for chat %d: %v", chatID, err)
		return
	}

	startTime := time.Now()
	var txCount int

	for {
		select {
		case <-ctx.Done():
			s.logger.Printf("Stopping monitor for chat %d after %v (%d transactions processed)",
				chatID, time.Since(startTime), txCount)
			return
		case tx, ok := <-txChan:
			if !ok {
				s.logger.Printf("Transaction channel closed for chat %d", chatID)
				return
			}
			txCount++
			s.logger.Printf("Processing transaction for chat %d: %s (%.2f USDC)",
				chatID, tx.Signature, tx.USDCAmount)

			// Apply each rule to the transaction
			for _, rule := range s.rules {
				if rule.Apply(&tx) {
					message := rule.BuildMessage(&tx)
					if err := s.bot.SendAlert(chatID, message); err != nil {
						s.logger.Printf("Failed to send alert for chat %d: %v", chatID, err)
					} else {
						s.logger.Printf("Successfully sent alert for transaction %s to chat %d",
							tx.Signature, chatID)
					}
				}
			}
		}
	}
}

// RemoveAddress removes an address from monitoring for a specific chat ID
func (s *Service) RemoveAddress(chatID int64, address string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Printf("Removing address %s for chat %d", address, chatID)

	if addrs, exists := s.addresses[chatID]; exists {
		newAddrs := make([]string, 0)
		for _, addr := range addrs {
			if addr != address {
				newAddrs = append(newAddrs, addr)
			}
		}
		s.addresses[chatID] = newAddrs
		s.logger.Printf("Updated address list for chat %d: %v", chatID, newAddrs)

		// Cancel existing monitor
		if cancel, exists := s.monitors[chatID]; exists {
			s.logger.Printf("Cancelling existing monitor for chat %d", chatID)
			cancel()
		}

		// Start new monitor if there are still addresses to monitor
		if len(newAddrs) > 0 {
			ctx, cancel := context.WithCancel(context.Background())
			s.monitors[chatID] = cancel
			go s.monitorAddresses(ctx, chatID, newAddrs)
		} else {
			s.logger.Printf("No more addresses to monitor for chat %d, removing monitor", chatID)
			delete(s.monitors, chatID)
		}
	}
}

// GetMonitoredAddresses returns the list of monitored addresses for a specific chat ID
func (s *Service) GetMonitoredAddresses(chatID int64) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if addrs, exists := s.addresses[chatID]; exists {
		s.logger.Printf("Retrieved monitored addresses for chat %d: %v", chatID, addrs)
		return addrs
	}

	s.logger.Printf("No monitored addresses found for chat %d", chatID)
	return nil
}

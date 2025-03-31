package monitor

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"solana-monitor/internal/config"
	"solana-monitor/internal/logging"
	"solana-monitor/internal/solana"
)

type SolanaClientInterface interface {
	MonitorTransactions(ctx context.Context, addresses []string, threshold float64) (<-chan solana.Transaction, error)
}

type TelegramBotInterface interface {
	SendAlert(chatID int64, message string) error
}

type Service struct {
	solanaClient SolanaClientInterface
	bot          TelegramBotInterface
	config       *config.Config
	addresses    map[int64][]string
	mu           sync.RWMutex
	monitors     map[int64]context.CancelFunc
	logger       *log.Logger
	rules        []Rule
}

func NewService(solanaClient SolanaClientInterface, config *config.Config) *Service {
	logFile, err := logging.CreateLogFile("monitor.log")
	if err != nil {
		log.Printf("Failed to open log file: %v, falling back to stdout", err)
		return nil
	}

	logger := log.New(logging.CreateMultiWriter(logFile), "[MONITOR] ", log.LstdFlags)
	logger.Printf("[INIT] Initializing monitoring service...")
	logger.Printf("[CONFIG] RPC Endpoint=%s", config.SolanaRPCEndpoint)

	threshold := 1000.0
	if thresholdStr := os.Getenv("USDC_THRESHOLD"); thresholdStr != "" {
		if val, err := strconv.ParseFloat(thresholdStr, 64); err == nil {
			threshold = val
		}
	}
	logger.Printf("[CONFIG] USDC Threshold=%.2f", threshold)

	return &Service{
		solanaClient: solanaClient,
		config:       config,
		addresses:    make(map[int64][]string),
		monitors:     make(map[int64]context.CancelFunc),
		rules:        []Rule{NewUSDCThresholdRule(threshold)},
		logger:       logger,
	}
}

func (s *Service) SetBot(bot TelegramBotInterface) {
	s.bot = bot
	s.logger.Printf("[SUCCESS] Telegram bot configured and ready")
}

func (s *Service) Start(ctx context.Context) error {
	s.logger.Printf("[STARTUP] Starting monitoring service...")

	s.mu.RLock()
	if len(s.addresses) == 0 {
		s.logger.Printf("[INFO] No addresses currently being monitored")
	} else {
		for chatID, addrs := range s.addresses {
			s.logger.Printf("📋 Chat %d monitoring addresses: %v", chatID, addrs)
		}
	}
	s.mu.RUnlock()

	<-ctx.Done()

	s.logger.Printf("[SHUTDOWN] Shutting down monitoring service...")

	s.mu.Lock()
	for chatID, cancel := range s.monitors {
		s.logger.Printf("[SHUTDOWN] Stopping monitor for chat %d", chatID)
		cancel()
	}
	s.mu.Unlock()

	s.logger.Printf("[SUCCESS] Monitoring service shutdown complete")
	return nil
}

func (s *Service) AddAddress(chatID int64, address string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Printf("[ADD] Adding address %s for chat %d", address, chatID)

	if _, exists := s.addresses[chatID]; !exists {
		s.logger.Printf("[INIT] Initializing address list for chat %d", chatID)
		s.addresses[chatID] = make([]string, 0)
	}

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

		if cancel, exists := s.monitors[chatID]; exists {
			s.logger.Printf("[SHUTDOWN] Cancelling existing monitor for chat %d", chatID)
			cancel()
		}

		ctx, cancel := context.WithCancel(context.Background())
		s.monitors[chatID] = cancel
		s.logger.Printf("[STARTUP] Starting new monitor for chat %d with addresses: %v", chatID, s.addresses[chatID])
		go s.monitorAddresses(ctx, chatID, s.addresses[chatID])
	}
}

func (s *Service) monitorAddresses(ctx context.Context, chatID int64, addresses []string) {
	s.logger.Printf("Starting transaction monitoring for chat %d, addresses: %v", chatID, addresses)

	var threshold float64 = 1000.0 // default value
	for _, rule := range s.rules {
		if usdcRule, ok := rule.(*USDCThresholdRule); ok {
			threshold = usdcRule.threshold
			break
		}
	}

	txChan, err := s.solanaClient.MonitorTransactions(ctx, addresses, threshold)
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

		if cancel, exists := s.monitors[chatID]; exists {
			s.logger.Printf("Cancelling existing monitor for chat %d", chatID)
			cancel()
		}

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

// ClearAddresses removes all monitored addresses for a given chat ID
func (s *Service) ClearAddresses(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Printf("Clearing all addresses for chat %d", chatID)

	// Cancel existing monitor if it exists
	if cancel, exists := s.monitors[chatID]; exists {
		s.logger.Printf("Cancelling monitor for chat %d", chatID)
		cancel()
		delete(s.monitors, chatID)
	}

	// Remove all addresses
	delete(s.addresses, chatID)
	s.logger.Printf("Successfully cleared all addresses for chat %d", chatID)
}

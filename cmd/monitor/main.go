package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"solana-monitor/internal/config"
	"solana-monitor/internal/logging"
	"solana-monitor/internal/monitor"
	"solana-monitor/internal/solana"
	"solana-monitor/internal/telegram"
)

func main() {
	// Initialize logging system
	logging.SetLogDirectory("logs")

	// Set up logging with timestamp in filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFile, err := logging.CreateLogFile(fmt.Sprintf("service_%s.log", timestamp))
	if err != nil {
		log.Fatalf("Failed to create log file: %v", err)
	}

	// Create a logger that writes to both file and console
	logger := log.New(logging.CreateMultiWriter(logFile), "[MAIN] ", log.LstdFlags|log.Lmicroseconds)

	logger.Printf("[STARTUP] Starting Solana Transaction Monitor...")
	logger.Printf("[INFO] Log file: %s", logFile.Name())

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("[ERROR] Failed to load configuration: %v", err)
	}
	logger.Printf("[SUCCESS] Configuration loaded successfully")
	logger.Printf("[CONFIG] RPC Endpoint: %s", cfg.SolanaRPCEndpoint)
	logger.Printf("[CONFIG] USDC Threshold: %.2f", cfg.USDCThreshold)

	// Create Solana client
	solanaClient, err := solana.NewClient(cfg.SolanaRPCEndpoint)
	if err != nil {
		logger.Fatalf("[ERROR] Failed to create Solana client: %v", err)
	}
	logger.Printf("[SUCCESS] Solana client initialized")

	// Create monitoring service
	monitorService := monitor.NewService(solanaClient, cfg)
	logger.Printf("[SUCCESS] Monitoring service created")

	// Create Telegram bot with address handler and getter
	bot, err := telegram.NewBot(
		cfg.TelegramToken,
		monitorService.AddAddress,
		monitorService.GetMonitoredAddresses,
		cfg,
	)
	if err != nil {
		logger.Fatalf("[ERROR] Failed to create Telegram bot: %v", err)
	}
	logger.Printf("[SUCCESS] Telegram bot created successfully")

	// Set the bot in the monitoring service
	monitorService.SetBot(bot)
	logger.Printf("[SUCCESS] Bot configured in monitoring service")

	// Create context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Printf("[SHUTDOWN] Received signal %v, initiating shutdown...", sig)
		cancel()
	}()

	// Start the bot in a goroutine
	go bot.Start()
	logger.Printf("[SUCCESS] Telegram bot started")

	// Print startup complete message
	logger.Printf("[STARTUP] Startup complete! Monitoring for USDC transfers...")
	logger.Printf("[INFO] Press Ctrl+C to stop")

	// Start monitoring
	if err := monitorService.Start(ctx); err != nil {
		logger.Fatalf("[ERROR] Monitoring service failed: %v", err)
	}

	logger.Printf("[SHUTDOWN] Service shutdown complete")
}

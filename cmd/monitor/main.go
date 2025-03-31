package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"solana-monitor/internal/config"
	"solana-monitor/internal/logging"
	"solana-monitor/internal/monitor"
	"solana-monitor/internal/solana"
	"solana-monitor/internal/telegram"
)

func main() {
	// Create a logger that writes to a file with timestamp
	logFile, err := logging.CreateLogFile("app.log")
	if err != nil {
		log.Fatalf("[ERROR] Failed to create log file: %v", err)
	}
	defer logFile.Close()

	logger := log.New(logging.CreateMultiWriter(logFile), "[APP] ", log.LstdFlags)
	logger.Printf("[STARTUP] Starting Solana Transaction Monitor...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("[ERROR] Failed to load configuration: %v", err)
	}
	logger.Printf("[CONFIG] Configuration loaded successfully")
	logger.Printf("[CONFIG] RPC Endpoint=%s", cfg.SolanaRPCEndpoint)

	// Create Solana client
	solanaClient, err := solana.NewClient(cfg.SolanaRPCEndpoint)
	if err != nil {
		logger.Fatalf("[ERROR] Failed to create Solana client: %v", err)
	}

	// Create monitoring service
	monitorService := monitor.NewService(solanaClient, cfg)
	if monitorService == nil {
		logger.Fatalf("[ERROR] Failed to create monitoring service")
	}

	// Create Telegram bot
	bot, err := telegram.NewBot(
		cfg.TelegramToken,
		monitorService.AddAddress,
		monitorService.GetMonitoredAddresses,
		monitorService.ClearAddresses,
		cfg,
	)
	if err != nil {
		logger.Fatalf("[ERROR] Failed to create Telegram bot: %v", err)
	}

	// Configure bot in monitoring service
	monitorService.SetBot(bot)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start services
	go func() {
		if err := monitorService.Start(ctx); err != nil {
			logger.Printf("[ERROR] Monitor service error: %v", err)
		}
	}()

	go bot.Start()

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Printf("[SHUTDOWN] Received signal %v, initiating graceful shutdown...", sig)

	// Cancel context to stop monitoring service
	cancel()

	logger.Printf("[SHUTDOWN] Shutdown complete")
}

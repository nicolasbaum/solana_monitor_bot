package solana

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"solana-monitor/internal/logging"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"golang.org/x/time/rate"
)

// USDC token mint address on Solana mainnet
const (
	USDCMint       = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	USDCDecimals   = 6
	initialBackoff = 5 * time.Second
	maxBackoff     = 2 * time.Minute

	// Rate limiting constants
	rateLimit   = 10                     // requests per second
	burstLimit  = 15                     // burst capacity
	minWaitTime = 100 * time.Millisecond // minimum wait between requests
)

// Client represents a Solana RPC client with rate limiting
type Client struct {
	rpcClient    *rpc.Client
	endpoint     string
	logger       *log.Logger
	backoff      time.Duration
	rateLimiter  *rate.Limiter
	lastRequest  time.Time
	lastReqMutex sync.Mutex
}

// NewClient creates a new Solana client with rate limiting
func NewClient(endpoint string) (*Client, error) {
	logFile, err := logging.CreateLogFile("solana-rpc.log")
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %v", err)
	}

	logger := log.New(logging.CreateMultiWriter(logFile), "[SOLANA-RPC] ", log.LstdFlags|log.Lmicroseconds)
	logger.Printf("Initializing Solana RPC client with endpoint: %s", endpoint)

	client := rpc.New(endpoint)

	// Test connection
	logger.Printf("Testing RPC connection...")
	slot, err := client.GetSlot(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC endpoint: %v", err)
	}
	logger.Printf("[SUCCESS] RPC connection successful! Current slot: %d", slot)

	return &Client{
		rpcClient:   client,
		logger:      logger,
		backoff:     initialBackoff,
		rateLimiter: rate.NewLimiter(rate.Limit(rateLimit), burstLimit),
		lastRequest: time.Now(),
	}, nil
}

// Transaction represents a simplified Solana transaction
type Transaction struct {
	Signature  string
	Timestamp  time.Time
	USDCAmount float64
	FromAddr   string
	ToAddr     string
}

// waitForRateLimit waits for rate limit token and enforces minimum wait time
func (c *Client) waitForRateLimit(ctx context.Context) error {
	c.lastReqMutex.Lock()
	timeSinceLastReq := time.Since(c.lastRequest)
	if timeSinceLastReq < minWaitTime {
		time.Sleep(minWaitTime - timeSinceLastReq)
	}
	c.lastReqMutex.Unlock()

	if err := c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter wait failed: %v", err)
	}

	c.lastReqMutex.Lock()
	c.lastRequest = time.Now()
	c.lastReqMutex.Unlock()
	return nil
}

// ptr returns a pointer to the provided uint64 value
func ptr(v uint64) *uint64 {
	return &v
}

// MonitorTransactions monitors USDC transactions for specific addresses
func (c *Client) MonitorTransactions(ctx context.Context, addresses []string, threshold float64) (<-chan Transaction, error) {
	c.logger.Printf("Starting transaction monitoring for addresses: %v (threshold: %.2f USDC)", addresses, threshold)

	// Validate addresses
	for _, addr := range addresses {
		if err := c.validateAddress(addr); err != nil {
			return nil, fmt.Errorf("invalid address %s: %v", addr, err)
		}
		c.logger.Printf("[SUCCESS] Successfully validated address: %s", addr)
	}

	txChan := make(chan Transaction)

	go func() {
		defer close(txChan)
		c.logger.Printf("Starting transaction monitoring goroutine")

		for _, address := range addresses {
			select {
			case <-ctx.Done():
				c.logger.Printf("Context canceled, stopping transaction monitoring for %s", address)
				return
			default:
				if err := c.waitForRateLimit(ctx); err != nil {
					c.logger.Printf("[ERROR] Rate limit wait failed: %v", err)
					continue
				}

				c.logger.Printf("Fetching signatures for address: %s", address)
				pubkey, _ := solana.PublicKeyFromBase58(address)
				sigs, err := c.rpcClient.GetSignaturesForAddress(ctx, pubkey)
				if err != nil {
					if strings.Contains(err.Error(), "Too many requests") {
						c.logger.Printf("[RATE LIMIT] Hit rate limit, backing off for %v", c.backoff)
						time.Sleep(c.backoff)
						c.backoff *= 2
						if c.backoff > maxBackoff {
							c.backoff = maxBackoff
						}
					}
					c.logger.Printf("[ERROR] Failed to get signatures for %s: %v", address, err)
					continue
				}
				c.backoff = initialBackoff // Reset backoff on success

				for _, sig := range sigs {
					select {
					case <-ctx.Done():
						c.logger.Printf("Context canceled, stopping transaction processing")
						return
					default:
						if err := c.waitForRateLimit(ctx); err != nil {
							c.logger.Printf("[ERROR] Rate limit wait failed: %v", err)
							continue
						}

						c.logger.Printf("Processing new transaction: %s", sig.Signature)
						opts := &rpc.GetTransactionOpts{
							Encoding:                       solana.EncodingBase64,
							Commitment:                     rpc.CommitmentConfirmed,
							MaxSupportedTransactionVersion: ptr(0),
						}
						tx, err := c.rpcClient.GetTransaction(ctx, sig.Signature, opts)
						if err != nil {
							if ctx.Err() != nil {
								c.logger.Printf("Context canceled while processing transaction %s", sig.Signature)
								return
							}
							if strings.Contains(err.Error(), "Too many requests") {
								c.logger.Printf("[RATE LIMIT] Hit rate limit, backing off for %v", c.backoff)
								time.Sleep(c.backoff)
								c.backoff *= 2
								if c.backoff > maxBackoff {
									c.backoff = maxBackoff
								}
								continue
							}
							// Skip transaction version errors as they are not critical
							if strings.Contains(err.Error(), "Transaction version") {
								c.logger.Printf("[WARN] Skipping unsupported transaction version for %s", sig.Signature)
								continue
							}
							c.logger.Printf("[ERROR] Failed to get transaction %s: %v (backing off)", sig.Signature, err)
							continue
						}
						c.backoff = initialBackoff // Reset backoff on success

						// Skip if transaction is nil or has no metadata
						if tx == nil || tx.Meta == nil {
							c.logger.Printf("[WARN] Transaction %s has no metadata, skipping", sig.Signature)
							continue
						}

						// Process transaction and send if it meets criteria
						if amount := c.extractUSDCAmount(tx); amount >= threshold {
							fromAddr, toAddr := c.extractAddresses(tx)
							select {
							case <-ctx.Done():
								return
							case txChan <- Transaction{
								Signature:  sig.Signature.String(),
								USDCAmount: amount,
								Timestamp:  time.Unix(int64(*tx.BlockTime), 0),
								FromAddr:   fromAddr,
								ToAddr:     toAddr,
							}:
							}
						}
					}
				}
			}
		}
	}()

	return txChan, nil
}

// extractAddresses extracts the sender and receiver addresses from a transaction
func (c *Client) extractAddresses(tx *rpc.GetTransactionResult) (string, string) {
	if tx == nil || tx.Transaction == nil {
		return "", ""
	}

	// Parse the transaction data
	decoded, err := tx.Transaction.GetTransaction()
	if err != nil {
		return "", ""
	}

	// Get the first two account keys as from/to addresses
	accounts := decoded.Message.AccountKeys
	if len(accounts) < 2 {
		return "", ""
	}

	return accounts[0].String(), accounts[1].String()
}

// extractUSDCAmount extracts the USDC amount from a transaction
func (c *Client) extractUSDCAmount(tx *rpc.GetTransactionResult) float64 {
	if tx == nil || tx.Meta == nil {
		return 0
	}

	// Look for USDC token transfers in post token balances
	usdcMintPubkey := solana.MustPublicKeyFromBase58(USDCMint)

	for _, postBalance := range tx.Meta.PostTokenBalances {
		if postBalance.Mint.Equals(usdcMintPubkey) {
			// Convert from raw amount (with decimals) to float
			rawAmount := postBalance.UiTokenAmount.Amount
			if rawAmount == "" {
				continue
			}

			amount, err := strconv.ParseFloat(rawAmount, 64)
			if err != nil {
				continue
			}

			return amount / math.Pow10(USDCDecimals)
		}
	}

	return 0
}

func (c *Client) validateAddress(address string) error {
	_, err := solana.PublicKeyFromBase58(address)
	return err
}

package solana

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"solana-monitor/internal/logging"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// USDC token mint address on Solana mainnet
const (
	USDCMint     = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	USDCDecimals = 6
)

// Client represents a Solana RPC client
type Client struct {
	rpcClient *rpc.Client
	endpoint  string
	logger    *log.Logger
}

// NewClient creates a new Solana client
func NewClient(endpoint string) (*Client, error) {
	// Create a logger that writes to both file and console
	logFile, err := logging.CreateLogFile("solana_rpc.log")
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %v", err)
	}

	// Create a multi-writer that writes to both file and console
	multiWriter := logging.CreateMultiWriter(logFile)
	logger := log.New(multiWriter, "[SOLANA-RPC] ", log.LstdFlags|log.Lmicroseconds)

	logger.Printf("Initializing Solana RPC client with endpoint: %s", endpoint)

	rpcClient := rpc.New(endpoint)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Printf("Testing RPC connection...")
	slot, err := rpcClient.GetSlot(ctx, rpc.CommitmentFinalized)
	if err != nil {
		logger.Printf("[ERROR] RPC connection test failed: %v", err)
		return nil, fmt.Errorf("failed to connect to RPC endpoint: %v", err)
	}
	logger.Printf("[SUCCESS] RPC connection successful! Current slot: %d", slot)

	return &Client{
		rpcClient: rpcClient,
		endpoint:  endpoint,
		logger:    logger,
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

// getRateLimitDelay extracts the retry delay from an RPC error
func (c *Client) getRateLimitDelay(err error) time.Duration {
	// Try to convert to jsonrpc.RPCError
	if errStr := err.Error(); strings.Contains(errStr, "jsonrpc.RPCError") {
		if strings.Contains(errStr, "Too many requests") {
			// For rate limit errors (429), use a progressive backoff starting at 5 seconds
			return 5 * time.Second
		}
	}
	return 0
}

// MonitorTransactions monitors USDC transactions for specific addresses
func (c *Client) MonitorTransactions(ctx context.Context, addresses []string, threshold float64) (<-chan Transaction, error) {
	txChan := make(chan Transaction)
	c.logger.Printf("Starting transaction monitoring for addresses: %v (threshold: %.2f USDC)", addresses, threshold)

	// Convert addresses to pubkeys
	var pubkeys []solana.PublicKey
	for _, addr := range addresses {
		pubkey, err := solana.PublicKeyFromBase58(addr)
		if err != nil {
			c.logger.Printf("[ERROR] Invalid address %s: %v", addr, err)
			return nil, fmt.Errorf("invalid address %s: %v", addr, err)
		}
		pubkeys = append(pubkeys, pubkey)
		c.logger.Printf("[SUCCESS] Successfully validated address: %s", addr)
	}

	// Start monitoring in a goroutine
	go func() {
		defer close(txChan)
		c.logger.Printf("Starting transaction monitoring goroutine")

		lastSignatures := make(map[solana.PublicKey]string)
		errorCount := 0
		baseDelay := 5 * time.Second // Increased base delay
		maxDelay := 2 * time.Minute
		rateLimitCount := 0
		maxRateLimitDelay := 30 * time.Second

		for {
			select {
			case <-ctx.Done():
				c.logger.Printf("Stopping transaction monitoring (context cancelled)")
				return
			default:
				for _, pubkey := range pubkeys {
					// Create a timeout context for each RPC call
					rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

					c.logger.Printf("Fetching signatures for address: %s", pubkey.String())
					sigs, err := c.rpcClient.GetSignaturesForAddress(rpcCtx, pubkey)
					cancel()

					if err != nil {
						// Check for rate limit error first
						if rateLimitDelay := c.getRateLimitDelay(err); rateLimitDelay > 0 {
							rateLimitCount++
							// Progressive backoff for rate limits
							actualDelay := time.Duration(math.Min(
								float64(rateLimitDelay)*math.Pow(1.5, float64(rateLimitCount-1)),
								float64(maxRateLimitDelay),
							))
							c.logger.Printf("🚫 Rate limited: waiting for %v (attempt %d)", actualDelay, rateLimitCount)
							time.Sleep(actualDelay)
							continue
						}

						// If not rate limited, use exponential backoff
						errorCount++
						delay := time.Duration(math.Min(float64(baseDelay)*math.Pow(2, float64(errorCount-1)), float64(maxDelay)))
						c.logger.Printf("[ERROR] Failed to get signatures for %s: %v (error count: %d, backing off for %v)",
							pubkey.String(), err, errorCount, delay)
						time.Sleep(delay)
						continue
					}
					errorCount = 0     // Reset error count on success
					rateLimitCount = 0 // Reset rate limit count on success

					// Process new signatures
					for _, sig := range sigs {
						if lastSignatures[pubkey] == sig.Signature.String() {
							continue // Skip already processed transactions
						}

						c.logger.Printf("Processing new transaction: %s", sig.Signature.String())

						// Get transaction with parsed data
						rpcCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
						maxVersion := uint64(0)
						opts := &rpc.GetTransactionOpts{
							Encoding:                       solana.EncodingBase64,
							MaxSupportedTransactionVersion: &maxVersion,
							Commitment:                     rpc.CommitmentConfirmed,
						}

						tx, err := c.rpcClient.GetTransaction(rpcCtx, sig.Signature, opts)
						cancel()

						if err != nil {
							// Check for rate limit error first
							if rateLimitDelay := c.getRateLimitDelay(err); rateLimitDelay > 0 {
								rateLimitCount++
								// Progressive backoff for rate limits
								actualDelay := time.Duration(math.Min(
									float64(rateLimitDelay)*math.Pow(1.5, float64(rateLimitCount-1)),
									float64(maxRateLimitDelay),
								))
								c.logger.Printf("🚫 Rate limited: waiting for %v (attempt %d)", actualDelay, rateLimitCount)
								time.Sleep(actualDelay)
								continue
							}

							// If not rate limited, use exponential backoff
							errorCount++
							delay := time.Duration(math.Min(float64(baseDelay)*math.Pow(2, float64(errorCount-1)), float64(maxDelay)))
							c.logger.Printf("[ERROR] Failed to get transaction %s: %v (backing off for %v)",
								sig.Signature.String(), err, delay)
							time.Sleep(delay)
							continue
						}
						errorCount = 0     // Reset error count on success
						rateLimitCount = 0 // Reset rate limit count on success

						// Process transaction if it involves USDC
						if amount := c.extractUSDCAmount(tx); amount >= threshold {
							c.logger.Printf("💰 Found large USDC transfer: %.2f USDC in tx %s",
								amount, sig.Signature.String())

							// Extract addresses
							fromAddr, toAddr := c.extractAddresses(tx)

							txChan <- Transaction{
								Signature:  sig.Signature.String(),
								Timestamp:  time.Unix(int64(*tx.BlockTime), 0),
								USDCAmount: amount,
								FromAddr:   fromAddr,
								ToAddr:     toAddr,
							}
						}

						// Update last processed signature
						lastSignatures[pubkey] = sig.Signature.String()

						// Sleep between transaction requests to avoid rate limiting
						time.Sleep(baseDelay)
					}
				}

				// Sleep between address checks
				time.Sleep(5 * time.Second)
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

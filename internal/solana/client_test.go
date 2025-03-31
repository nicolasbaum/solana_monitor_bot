package solana

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/time/rate"
)

// MockRPCClient is a mock implementation of the Solana RPC client
type MockRPCClient struct {
	mock.Mock
}

func (m *MockRPCClient) CallForInto(ctx context.Context, out interface{}, method string, params []interface{}) error {
	args := m.Called(ctx, out, method, params)
	// Simulate the RPC response based on the method
	switch method {
	case "getSignaturesForAddress":
		if outPtr, ok := out.(*[]*rpc.TransactionSignature); ok {
			signature := solana.MustSignatureFromBase58("5wHu1qwD8ka3Z4CiGHh8dBjsb7tStV4nJgCz2Fi7CqYHRX6MXTEKQbjW2zQEGiRsHKc8DkwmNghZ4VKGJDvhDhYj")
			*outPtr = []*rpc.TransactionSignature{{Signature: signature}}
		}
	case "getTransaction":
		if outPtr, ok := out.(*rpc.GetTransactionResult); ok {
			fromAddr := solana.MustPublicKeyFromBase58("DjuMPGThkGdyk2vDvDDYjTFSyxzTuJKK7LkGy9xCv3t7")
			toAddr := solana.MustPublicKeyFromBase58("2xNweLHLqrbx4zo1waDvgWJHgsUpPj8Y8icbAFeR4a8i")
			blockTime := solana.UnixTimeSeconds(1234567890)

			// Create a transaction with the test data
			tx := &solana.Transaction{
				Message: solana.Message{
					AccountKeys: []solana.PublicKey{fromAddr, toAddr},
				},
			}

			// Create a transaction envelope by marshaling and unmarshaling the transaction
			txBytes, _ := json.Marshal(tx)
			envelope := &rpc.TransactionResultEnvelope{}
			json.Unmarshal(txBytes, envelope)

			*outPtr = rpc.GetTransactionResult{
				BlockTime:   &blockTime,
				Transaction: envelope,
				Meta: &rpc.TransactionMeta{
					PostTokenBalances: []rpc.TokenBalance{
						{
							Mint: solana.MustPublicKeyFromBase58(USDCMint),
							UiTokenAmount: &rpc.UiTokenAmount{
								Amount: "2000000000", // 2000 USDC
							},
						},
					},
				},
			}
		}
	case "getSlot":
		if outPtr, ok := out.(*uint64); ok {
			*outPtr = 12345
		}
	}
	return args.Error(0)
}

func (m *MockRPCClient) CallWithCallback(ctx context.Context, method string, params []interface{}, callback func(*http.Request, *http.Response) error) error {
	args := m.Called(ctx, method, params, callback)
	return args.Error(0)
}

func (m *MockRPCClient) CallBatch(ctx context.Context, requests jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	args := m.Called(ctx, requests)
	return args.Get(0).(jsonrpc.RPCResponses), args.Error(1)
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{
			name:     "Valid endpoint",
			endpoint: "https://api.mainnet-beta.solana.com",
			wantErr:  false,
		},
		{
			name:     "Empty endpoint",
			endpoint: "",
			wantErr:  false, // solana-go allows empty endpoint
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRPC := new(MockRPCClient)
			mockRPC.On("CallForInto", mock.Anything, mock.Anything, "getSlot", mock.Anything).Return(nil)

			// Create a client with our mock
			client := &Client{
				rpcClient: rpc.NewWithCustomRPCClient(mockRPC),
				endpoint:  tt.endpoint,
				logger:    NewTestLogger(),
			}

			assert.NotNil(t, client)
			assert.Equal(t, tt.endpoint, client.endpoint)
		})
	}
}

func TestExtractUSDCAmount(t *testing.T) {
	tests := []struct {
		name     string
		tx       *rpc.GetTransactionResult
		want     float64
		wantZero bool
	}{
		{
			name: "Valid USDC transfer",
			tx: &rpc.GetTransactionResult{
				Meta: &rpc.TransactionMeta{
					PostTokenBalances: []rpc.TokenBalance{
						{
							Mint: solana.MustPublicKeyFromBase58(USDCMint),
							UiTokenAmount: &rpc.UiTokenAmount{
								Amount: "1000000000", // 1000 USDC
							},
						},
					},
				},
			},
			want:     1000.0,
			wantZero: false,
		},
		{
			name:     "Nil transaction",
			tx:       nil,
			want:     0,
			wantZero: true,
		},
		{
			name: "No USDC transfer",
			tx: &rpc.GetTransactionResult{
				Meta: &rpc.TransactionMeta{
					PostTokenBalances: []rpc.TokenBalance{
						{
							Mint: solana.MustPublicKeyFromBase58("DjuMPGThkGdyk2vDvDDYjTFSyxzTuJKK7LkGy9xCv3t7"),
							UiTokenAmount: &rpc.UiTokenAmount{
								Amount: "1000000000",
							},
						},
					},
				},
			},
			want:     0,
			wantZero: true,
		},
	}

	client := &Client{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.extractUSDCAmount(tt.tx)
			if tt.wantZero {
				assert.Zero(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExtractAddresses(t *testing.T) {
	fromAddr := solana.MustPublicKeyFromBase58("DjuMPGThkGdyk2vDvDDYjTFSyxzTuJKK7LkGy9xCv3t7")
	toAddr := solana.MustPublicKeyFromBase58("2xNweLHLqrbx4zo1waDvgWJHgsUpPj8Y8icbAFeR4a8i")

	// Create a transaction with the test data
	tx := &solana.Transaction{
		Message: solana.Message{
			AccountKeys: []solana.PublicKey{fromAddr, toAddr},
		},
	}

	// Create a transaction envelope by marshaling and unmarshaling the transaction
	txBytes, _ := json.Marshal(tx)
	envelope := &rpc.TransactionResultEnvelope{}
	json.Unmarshal(txBytes, envelope)

	// Create a transaction with insufficient accounts
	insufficientTx := &solana.Transaction{
		Message: solana.Message{
			AccountKeys: []solana.PublicKey{fromAddr},
		},
	}

	// Create a transaction envelope for insufficient accounts
	insufficientTxBytes, _ := json.Marshal(insufficientTx)
	insufficientEnvelope := &rpc.TransactionResultEnvelope{}
	json.Unmarshal(insufficientTxBytes, insufficientEnvelope)

	tests := []struct {
		name          string
		tx            *rpc.GetTransactionResult
		wantFrom      string
		wantTo        string
		wantBothEmpty bool
	}{
		{
			name: "Valid transaction with addresses",
			tx: &rpc.GetTransactionResult{
				Transaction: envelope,
			},
			wantFrom:      fromAddr.String(),
			wantTo:        toAddr.String(),
			wantBothEmpty: false,
		},
		{
			name:          "Nil transaction",
			tx:            nil,
			wantBothEmpty: true,
		},
		{
			name: "Transaction with insufficient accounts",
			tx: &rpc.GetTransactionResult{
				Transaction: insufficientEnvelope,
			},
			wantBothEmpty: true,
		},
	}

	client := &Client{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to := client.extractAddresses(tt.tx)
			if tt.wantBothEmpty {
				assert.Empty(t, from)
				assert.Empty(t, to)
			} else {
				assert.Equal(t, tt.wantFrom, from)
				assert.Equal(t, tt.wantTo, to)
			}
		})
	}
}

func TestMonitorTransactions(t *testing.T) {
	mockRPC := new(MockRPCClient)
	logger := NewTestLogger()
	client := &Client{
		rpcClient:   rpc.NewWithCustomRPCClient(mockRPC), // Use our mock as the JSONRPCClient
		logger:      logger,
		rateLimiter: rate.NewLimiter(rate.Limit(100), 1), // Higher rate limit for tests
		lastRequest: time.Now(),
		backoff:     initialBackoff,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) // Increased timeout
	defer cancel()

	addr := "DjuMPGThkGdyk2vDvDDYjTFSyxzTuJKK7LkGy9xCv3t7"
	addresses := []string{addr}
	threshold := 1000.0

	// Create test transaction data
	fromAddr := solana.MustPublicKeyFromBase58("DjuMPGThkGdyk2vDvDDYjTFSyxzTuJKK7LkGy9xCv3t7")
	toAddr := solana.MustPublicKeyFromBase58("2xNweLHLqrbx4zo1waDvgWJHgsUpPj8Y8icbAFeR4a8i")
	tx := &solana.Transaction{
		Message: solana.Message{
			AccountKeys: []solana.PublicKey{fromAddr, toAddr},
		},
	}

	// Create transaction envelope
	txBytes, _ := json.Marshal(tx)
	envelope := &rpc.TransactionResultEnvelope{}
	json.Unmarshal(txBytes, envelope)

	// Create block time
	blockTime := solana.UnixTimeSeconds(1234567890)

	// Mock getSignaturesForAddress response
	mockRPC.On("CallForInto", mock.MatchedBy(func(ctx interface{}) bool {
		_, ok := ctx.(context.Context)
		return ok
	}), mock.MatchedBy(func(result interface{}) bool {
		if _, ok := result.(*[]*rpc.TransactionSignature); ok {
			// Set the result
			r := result.(*[]*rpc.TransactionSignature)
			signature := solana.MustSignatureFromBase58("5wHu1qwD8ka3Z4CiGHh8dBjsb7tStV4nJgCz2Fi7CqYHRX6MXTEKQbjW2zQEGiRsHKc8DkwmNghZ4VKGJDvhDhYj")
			*r = []*rpc.TransactionSignature{
				{
					Signature: signature,
					BlockTime: &blockTime,
				},
			}
			logger.Printf("[TEST] Mock: Set signatures result")
			return true
		}
		return false
	}), "getSignaturesForAddress", mock.Anything).Return(nil)

	// Create USDC mint public key
	usdcMint := solana.MustPublicKeyFromBase58(USDCMint)

	// Mock getTransaction response with exact parameter matching
	mockRPC.On("CallForInto", mock.MatchedBy(func(ctx interface{}) bool {
		_, ok := ctx.(context.Context)
		return ok
	}), mock.MatchedBy(func(result interface{}) bool {
		if _, ok := result.(**rpc.GetTransactionResult); ok {
			// Set the result
			r := result.(**rpc.GetTransactionResult)
			*r = &rpc.GetTransactionResult{
				Transaction: envelope,
				BlockTime:   &blockTime,
				Meta: &rpc.TransactionMeta{
					PostTokenBalances: []rpc.TokenBalance{
						{
							Mint: usdcMint,
							UiTokenAmount: &rpc.UiTokenAmount{
								Amount: "2000000000", // 2000 USDC with 6 decimals
							},
						},
					},
				},
			}
			logger.Printf("[TEST] Mock: Set transaction result")
			return true
		}
		return false
	}), "getTransaction", mock.MatchedBy(func(args []interface{}) bool {
		if len(args) != 2 {
			return false
		}
		// Check if first arg is a signature
		if _, ok := args[0].(solana.Signature); !ok {
			return false
		}
		// Check if second arg is a map with expected keys
		if opts, ok := args[1].(rpc.M); !ok {
			return false
		} else {
			_, hasCommitment := opts["commitment"]
			_, hasEncoding := opts["encoding"]
			_, hasMaxVersion := opts["maxSupportedTransactionVersion"]
			return hasCommitment && hasEncoding && hasMaxVersion
		}
	})).Return(nil)

	// Start monitoring
	logger.Printf("[TEST] Starting monitoring")
	txChan, err := client.MonitorTransactions(ctx, addresses, threshold)
	assert.NoError(t, err)
	assert.NotNil(t, txChan)

	// Wait for transaction
	logger.Printf("[TEST] Waiting for transaction")
	select {
	case tx := <-txChan:
		logger.Printf("[TEST] Received transaction")
		assert.Equal(t, "5wHu1qwD8ka3Z4CiGHh8dBjsb7tStV4nJgCz2Fi7CqYHRX6MXTEKQbjW2zQEGiRsHKc8DkwmNghZ4VKGJDvhDhYj", tx.Signature)
		assert.Equal(t, "DjuMPGThkGdyk2vDvDDYjTFSyxzTuJKK7LkGy9xCv3t7", tx.FromAddr)
		assert.Equal(t, "2xNweLHLqrbx4zo1waDvgWJHgsUpPj8Y8icbAFeR4a8i", tx.ToAddr)
		assert.Equal(t, float64(2000), tx.USDCAmount)
		assert.Equal(t, time.Unix(1234567890, 0), tx.Timestamp)
	case <-ctx.Done():
		t.Fatal("Timeout waiting for transaction")
	}
}

// NewTestLogger creates a logger for testing
func NewTestLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

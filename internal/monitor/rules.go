package monitor

import (
	"fmt"
	"solana-monitor/internal/solana"
)

// Rule defines the interface for transaction processing rules
type Rule interface {
	// Apply checks if the rule should be applied to the transaction
	Apply(tx *solana.Transaction) bool

	// BuildMessage generates the notification message for the transaction
	BuildMessage(tx *solana.Transaction) string
}

// USDCThresholdRule implements Rule for monitoring large USDC transfers
type USDCThresholdRule struct {
	threshold float64
}

// NewUSDCThresholdRule creates a new USDC threshold rule
func NewUSDCThresholdRule(threshold float64) *USDCThresholdRule {
	return &USDCThresholdRule{
		threshold: threshold,
	}
}

func (r *USDCThresholdRule) Apply(tx *solana.Transaction) bool {
	return tx.USDCAmount >= r.threshold
}

// BuildMessage generates a formatted alert message for the transaction
func (r *USDCThresholdRule) BuildMessage(tx *solana.Transaction) string {
	return fmt.Sprintf(
		"[ALERT] Large USDC Transfer Detected!\n\n"+
			"Amount: %.2f USDC\n"+
			"From: %s\n"+
			"To: %s\n\n"+
			"View on Solscan: https://solscan.io/tx/%s",
		tx.USDCAmount, tx.FromAddr, tx.ToAddr, tx.Signature,
	)
}

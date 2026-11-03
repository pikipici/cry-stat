// Phase 3 D2 (revisi V.1): ledger balance invariant tests.
// Pure unit untuk validateLegs (tanpa DB). DB tests (buy fee 0/nonzero,
// deposit, precision 18, missing-leg rollback) jalan di VPS via verify.
package ledger

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestValidateLegsBalance(t *testing.T) {
	// Buy fee nonzero: -IDR/+IDR, float -gross, payout +sent, fee +fee.
	legs := []leg{
		{owner: "user:1", asset: "IDR", amount: decimal.NewFromInt(50000).Neg()},
		{owner: "house:cash", asset: "IDR", amount: decimal.NewFromInt(50000)},
		{owner: "house:float", asset: "SOL", amount: decimal.RequireFromString("0.030000000000000000").Neg()},
		{owner: "external:payout", asset: "SOL", amount: decimal.RequireFromString("0.029990000000000000")},
		{owner: "house:fee-collect", asset: "SOL", amount: decimal.RequireFromString("0.000010000000000000")},
	}
	if err := validateLegs(legs); err != nil {
		t.Fatalf("buy fee nonzero harus balance: %v", err)
	}
}

func TestValidateLegsFeeZero(t *testing.T) {
	legs := []leg{
		{owner: "user:1", asset: "IDR", amount: decimal.NewFromInt(50000).Neg()},
		{owner: "house:cash", asset: "IDR", amount: decimal.NewFromInt(50000)},
		{owner: "house:float", asset: "SOL", amount: decimal.RequireFromString("0.030000000000000000").Neg()},
		{owner: "external:payout", asset: "SOL", amount: decimal.RequireFromString("0.030000000000000000")},
		{owner: "house:fee-collect", asset: "SOL", amount: decimal.Zero},
	}
	if err := validateLegs(legs); err != nil {
		t.Fatalf("buy fee zero harus balance: %v", err)
	}
}

func TestValidateLegsDeposit(t *testing.T) {
	legs := []leg{
		{owner: "external:deposit", asset: "SOL", amount: decimal.RequireFromString("0.030000000000000000").Neg()},
		{owner: "house:float", asset: "SOL", amount: decimal.RequireFromString("0.030000000000000000")},
		{owner: "house:cash", asset: "IDR", amount: decimal.NewFromInt(50000).Neg()},
		{owner: "user:2", asset: "IDR", amount: decimal.NewFromInt(50000)},
	}
	if err := validateLegs(legs); err != nil {
		t.Fatalf("deposit harus balance: %v", err)
	}
}

func TestValidateLegsRejectsImbalance(t *testing.T) {
	// Template lama A3: float -sent + fee (tanpa gross/external) = imbalance.
	legs := []leg{
		{owner: "house:float", asset: "SOL", amount: decimal.RequireFromString("0.029990000000000000").Neg()},
		{owner: "house:fee-collect", asset: "SOL", amount: decimal.RequireFromString("0.000010000000000000")},
	}
	if err := validateLegs(legs); err == nil {
		t.Fatal("template lama harus DITOLAK validateLegs")
	}
}

func TestValidateLegsRejectsMissingLeg(t *testing.T) {
	// Kaki IDR hilang sebelah.
	legs := []leg{
		{owner: "user:1", asset: "IDR", amount: decimal.NewFromInt(50000).Neg()},
	}
	if err := validateLegs(legs); err == nil {
		t.Fatal("missing leg harus DITOLAK")
	}
}

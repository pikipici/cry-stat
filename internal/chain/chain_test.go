package chain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestNetworkFor(t *testing.T) {
	for _, c := range []struct{ asset, net string }{
		{"BNB", "BSC"}, {"ETH", "ERC20"}, {"SOL", "SOL"},
		{"bnb", "BSC"}, {"sol", "SOL"},
	} {
		if net, ok := NetworkFor(c.asset); !ok || net != c.net {
			t.Fatalf("NetworkFor(%q) = (%q,%v), mau (%q,true)", c.asset, net, ok, c.net)
		}
	}
	if _, ok := NetworkFor("USDT"); ok {
		t.Fatal("USDT lolos NetworkFor!")
	}
}

func TestValidateAddress(t *testing.T) {
	goodBNB := "0x1234567890abcdef1234567890ABCDEF12345678"
	if err := ValidateAddress("BNB", goodBNB); err != nil {
		t.Fatalf("BNB valid ditolak: %v", err)
	}
	if err := ValidateAddress("ETH", goodBNB); err != nil {
		t.Fatalf("ETH valid ditolak: %v", err)
	}
	goodSOL := "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsHYdLVL9zreA"
	// 44 char base58 valid (tanpa 0/O/I/l).
	if err := ValidateAddress("SOL", goodSOL); err != nil {
		t.Fatalf("SOL valid ditolak: %v", err)
	}
	if err := ValidateAddress("BNB", "0x123"); err == nil {
		t.Fatal("BNB pendek lolos!")
	}
	if err := ValidateAddress("BNB", "1234567890abcdef1234567890ABCDEF12345678AA"); err == nil {
		t.Fatal("BNB tanpa 0x lolos!")
	}
	if err := ValidateAddress("ETH", "0xZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ"); err == nil {
		t.Fatal("ETH non-hex lolos!")
	}
	if err := ValidateAddress("SOL", "pendek"); err == nil {
		t.Fatal("SOL pendek lolos!")
	}
	// Base58 tidak punya char 0, O, I, l.
	if err := ValidateAddress("SOL", "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zrO0"); err == nil {
		t.Fatal("SOL dengan O/0 lolos!")
	}
	if err := ValidateAddress("USDT", "apapun"); err == nil {
		t.Fatal("coin asing lolos!")
	}
}

func TestFeeMath(t *testing.T) {
	units := decimal.NewFromFloat(0.028671264961)
	fee := decimal.NewFromFloat(0.00001)
	sent := units.Sub(fee)
	if !sent.GreaterThan(decimal.Zero) {
		t.Fatal("sent nol!")
	}
	// Fee > 20% units = tolak.
	bigFee := units.Mul(decimal.NewFromFloat(0.25))
	if bigFee.Div(units).LessThanOrEqual(decimal.NewFromFloat(0.2)) {
		t.Fatal("math reject salah!")
	}
}

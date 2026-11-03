// Phase 5 A18 (revisi V.1): SOL base58 vectors + no-panic fuzz-ish.
// Pure unit — jalan di mana saja.
package chain

import (
	"strings"
	"testing"
)

func TestSolBase58RejectsForbidden(t *testing.T) {
	// O, I, l bukan base58 — walau panjang 32-44, harus ditolak.
	// Basis: USDC mint (44-char 32-byte valid), ganti 1 char jadi O/I/l/0.
	base := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	if err := ValidateAddress("SOL", base); err != nil {
		t.Fatalf("basis valid ditolak: %v", err)
	}
	for _, bad := range []struct {
		name string
		addr string
	}{
		{"O-for-A", strings.Replace(base, "A", "O", 1)},
		{"I-for-H", strings.Replace(base, "H", "I", 1)},
		{"l-for-k", strings.Replace(base, "k", "l", 1)},
		{"zero-digit", strings.Replace(base, "9", "0", 1)},
	} {
		if err := ValidateAddress("SOL", bad.addr); err == nil {
			t.Fatalf("%s LOLOS: %q", bad.name, bad.addr)
		}
	}
}

func TestSolValidVectors(t *testing.T) {
	// Vektor 32-byte valid: system program + USDC mint + wrapped SOL.
	// (String mock lama "9WzD…" 41-char decode 30 byte = BUKAN pubkey valid,
	// sudah diganti mock baru di seluruh repo.)
	for _, v := range []string{
		"11111111111111111111111111111111",             // system program (32x '1')
		"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC mint
		"So11111111111111111111111111111111111111112",  // wrapped SOL
	} {
		if err := ValidateAddress("SOL", v); err != nil {
			t.Fatalf("vektor valid ditolak %q: %v", v, err)
		}
	}
}

func TestSolNoPanic(t *testing.T) {
	// Byte acak/extrem — tidak boleh panic, hanya error/nil yang waras.
	inputs := []string{
		"", " ", "111", strings.Repeat("1", 31), strings.Repeat("1", 45),
		strings.Repeat("0", 44), strings.Repeat("O", 44),
		strings.Repeat("!", 44), "4wBqpZM9xaSheZzJSMawUKKwhdpChKbZ5eu5ky4Vigw ",
		" 4wBqpZM9xaSheZzJSMawUKKwhdpChKbZ5eu5ky4Vigw",
		"\x00\x01\x02", "1111111111111111111111111111111\xff",
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic untuk %q: %v", in, r)
				}
			}()
			_ = ValidateAddress("SOL", in)
		}()
	}
}

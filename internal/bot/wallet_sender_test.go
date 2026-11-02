package bot

import (
	"errors"
	"strings"
	"testing"

	"github.com/pikipici/crypto-station/internal/ledger"
)

// R5: handler-level decision tests tanpa Telegram API.
// Menguji routing + allowlist + copy UX, bukan string keyboard semata.

func TestDecideWalletCallbackNonAdminDenied(t *testing.T) {
	for _, data := range []string{"a:wallet", "a:wallet:assets", "a:wallet:deposit", "a:wdep:SOL", "a:wdepok:1", "a:wdepno:1", "a:activity", "a:act:STOCK_IN:2"} {
		d := decideWalletCallback(data, false, true)
		if d.policy != "deny" {
			t.Fatalf("callback %q non-admin harus deny, dapat %+v", data, d)
		}
		if !strings.HasPrefix(d.denyReason, "panel:") {
			t.Fatalf("deny reason harus panel:*, dapat %q", d.denyReason)
		}
	}
}

func TestDecideWalletCallbackAdminRouting(t *testing.T) {
	cases := map[string]string{
		"a:wallet":         "menu",
		"a:wallet:assets":  "assets",
		"a:wallet:deposit": "deposit-picker",
		"a:wdep:SOL":       "deposit-coin",
		"a:wdepok:9":       "confirm",
		"a:wdepno:9":       "cancel",
		"a:activity":       "activity",
		"a:activity:25":    "activity",
		"a:act:STOCK_IN:2": "detail",
		"a:act:PAYOUT_OUT:3": "detail",
	}
	for data, want := range cases {
		d := decideWalletCallback(data, true, true)
		if d.policy != "ok" || d.action != want {
			t.Fatalf("callback %q harus ok/%s, dapat %+v", data, want, d)
		}
	}
}

func TestDecideWalletCallbackMalformedSilent(t *testing.T) {
	for _, data := range []string{"", "a:wdep:DOGE", "a:wdep:", "a:unknown:xyz"} {
		d := decideWalletCallback(data, true, true)
		if d.policy != "silent" {
			t.Fatalf("callback %q harus silent, dapat %+v", data, d)
		}
	}
}

func TestDecideWalletCallbackNotReady(t *testing.T) {
	d := decideWalletCallback("a:wallet", true, false)
	if d.policy != "ok" || d.action != "not-ready" {
		t.Fatalf("wallet belum siap harus not-ready, dapat %+v", d)
	}
	// Non-admin tetap deny walau wallet belum siap (allowlist dulu).
	d = decideWalletCallback("a:wallet", false, false)
	if d.policy != "deny" {
		t.Fatalf("non-admin harus deny walau not-ready, dapat %+v", d)
	}
}

func TestWalletConfirmCopyMapping(t *testing.T) {
	if got := walletConfirmCopy(ledger.ErrStockDepositRefConflict); !strings.Contains(got, "sudah pernah digunakan untuk data lain") {
		t.Fatalf("conflict copy salah: %q", got)
	}
	if got := walletConfirmCopy(ledger.StockDepositConflictError{Field: "units"}); !strings.Contains(got, "sudah pernah digunakan untuk data lain") {
		t.Fatalf("conflict struct copy salah: %q", got)
	}
	if got := walletConfirmCopy(ledger.ErrStockDepositCancelled); !strings.Contains(got, "sudah dibatalkan") {
		t.Fatalf("cancelled copy salah: %q", got)
	}
	if got := walletConfirmCopy(ledger.ErrStockDepositUnauthorized); got != "Deposit tidak ditemukan." {
		t.Fatalf("unauthorized copy bocor/salah: %q", got)
	}
	if got := walletConfirmCopy(errors.New("insert audit success: conn fail")); !strings.Contains(got, "jejak audit") {
		t.Fatalf("audit copy salah: %q", got)
	}
	if got := walletConfirmCopy(errors.New("coin harus BNB/ETH/SOL")); !strings.HasPrefix(got, "Gagal mencatat deposit: ") {
		t.Fatalf("generic copy salah: %q", got)
	}
	if got := walletConfirmCopy(nil); got != "" {
		t.Fatalf("nil harus kosong, dapat %q", got)
	}
}

func TestWalletConfirmCopyNoLeak(t *testing.T) {
	// Conflict error tidak boleh mengandung payload lama.
	err := ledger.StockDepositConflictError{Field: "to_address"}
	got := walletConfirmCopy(err)
	for _, secret := range []string{"to_address", "MOCK-", "0x", "journal"} {
		if strings.Contains(got, secret) {
			t.Fatalf("copy membocorkan %q: %q", secret, got)
		}
	}
}

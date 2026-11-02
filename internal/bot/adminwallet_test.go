package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/pikipici/crypto-station/internal/adminwallet"
	"github.com/shopspring/decimal"
)

func TestWalletKeyboardCallbacks(t *testing.T) {
	if got := AdminPanel().InlineKeyboard[2][0].CallbackData; got != "a:wallet" {
		t.Fatalf("wallet=%q", got)
	}
	if got := AdminPanel().InlineKeyboard[2][1].CallbackData; got != "a:activity" {
		t.Fatalf("activity=%q", got)
	}
	if got := AdminWalletDepositPicker().InlineKeyboard[0][0].CallbackData; got != "a:wdep:BNB" {
		t.Fatalf("deposit=%q", got)
	}
	if got := WalletDepositConfirm(12).InlineKeyboard[0][0].CallbackData; got != "a:wdepok:12" {
		t.Fatalf("confirm=%q", got)
	}
	if got := WalletDepositConfirm(12).InlineKeyboard[0][1].CallbackData; got != "a:wdepno:12" {
		t.Fatalf("cancel=%q", got)
	}
}

func TestWalletSessionStates(t *testing.T) {
	s := NewSessions()
	s.Set(7, Session{Step: StepAdminStockDepositRef, Coin: "SOL", Units: "1.25", HotAddr: "hot", TxRef: "MOCK-a", DepositID: 5})
	got := s.Get(7)
	if got.Step != StepAdminStockDepositRef || got.Units != "1.25" || got.DepositID != 5 {
		t.Fatalf("session=%+v", got)
	}
	s.Clear(7)
	if s.Get(7).Step != StepIdle {
		t.Fatal("cancel harus membersihkan session")
	}
}

func TestWalletDepositNeedsHot(t *testing.T) {
	if validateWalletDepositStart("BNB", "") == nil {
		t.Fatal("hot kosong harus ditolak")
	}
	if validateWalletDepositStart("DOGE", "hot") == nil {
		t.Fatal("asset invalid harus ditolak")
	}
	if err := validateWalletDepositStart("ETH", "0xhot"); err != nil {
		t.Fatal(err)
	}
}

func TestParseWalletUnits(t *testing.T) {
	got, err := parseWalletUnits("1.000000000000000001")
	if err != nil || !got.Equal(decimal.RequireFromString("1.000000000000000001")) {
		t.Fatalf("units=%s err=%v", got, err)
	}
	for _, value := range []string{"", "0", "-1", "abc"} {
		if _, err := parseWalletUnits(value); err == nil {
			t.Fatalf("%q harus invalid", value)
		}
	}
}

func TestValidateWalletTxRef(t *testing.T) {
	for _, value := range []string{"MOCK-ref-1", "0x1234567890abcdef1234567890abcdef12345678"} {
		if err := validateWalletTxRef("BNB", value); err != nil {
			t.Fatalf("%q: %v", value, err)
		}
	}
	for _, value := range []string{"MOCK-", "bad-ref"} {
		if err := validateWalletTxRef("BNB", value); err == nil {
			t.Fatalf("%q harus invalid", value)
		}
	}
}

func TestWalletShorteners(t *testing.T) {
	if got := shortAddress("0x1234567890abcdef"); got != "0x1234...cdef" {
		t.Fatalf("address=%q", got)
	}
	if got := shortTxRef("MOCK-123456789012345"); got != "MOCK-1234567..." {
		t.Fatalf("ref=%q", got)
	}
	if got := shortAddress("pendek"); got != "pendek" {
		t.Fatalf("short=%q", got)
	}
}

func TestFormatWIB(t *testing.T) {
	value := time.Date(2026, 11, 2, 0, 30, 0, 0, time.UTC)
	if got := formatWIB(value); got != "02-11-2026 07:30 WIB" {
		t.Fatalf("wib=%q", got)
	}
}

func TestWalletFormattersAreSimulation(t *testing.T) {
	assets := []adminwallet.AssetSummary{{Asset: "BNB", Network: "BSC", Balance: decimal.NewFromInt(2), Minimum: decimal.NewFromInt(1), Status: "AMAN", HotAddress: "", EstimateOK: false}}
	if got := formatAssetSummary(assets); !strings.Contains(got, "SIMULASI") || !strings.Contains(got, "Saldo jurnal") || !strings.Contains(got, "tidak tersedia (harga basi)") {
		t.Fatalf("asset format=%q", got)
	}
	items := []adminwallet.ActivityItem{{Kind: adminwallet.KindStockIn, ID: 1, Asset: "SOL", Units: decimal.NewFromInt(3), Status: "recorded_mock", TxRef: "MOCK-123456789012345", CreatedAt: time.Now()}}
	if got := formatActivityList(items, 10); !strings.Contains(got, "SIMULASI") || !strings.Contains(got, "➕") {
		t.Fatalf("activity format=%q", got)
	}
}

func TestGuardTelegramList(t *testing.T) {
	value := strings.Repeat("baris panjang\n", 500)
	got := guardTelegramList(value, 25)
	if len(got) > 4000 || !strings.Contains(got, "Daftar dipotong") {
		t.Fatalf("guard len=%d", len(got))
	}
}

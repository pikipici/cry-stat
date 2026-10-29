package price

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestMidFormula(t *testing.T) {
	// mid USDT = (bestBid + bestAsk) / 2  →  (100 + 102) / 2 = 101
	mid := decimal.NewFromFloat(100).Add(decimal.NewFromFloat(102)).Div(decimal.NewFromInt(2))
	if !mid.Equal(decimal.NewFromInt(101)) {
		t.Fatalf("mid salah: %s", mid)
	}
	midIDR := mid.Mul(decimal.NewFromInt(16000)) // 1.616.000
	if !midIDR.Equal(decimal.NewFromInt(1616000)) {
		t.Fatalf("midIDR salah: %s", midIDR)
	}
	one := decimal.NewFromInt(1)
	ask := midIDR.Mul(one.Add(decimal.NewFromFloat(0.01)))   // 1.632.160
	bid := midIDR.Mul(one.Sub(decimal.NewFromFloat(0.01)))   // 1.599.840
	if !ask.Equal(decimal.NewFromInt(1632160)) {
		t.Fatalf("ask salah: %s", ask)
	}
	if !bid.Equal(decimal.NewFromInt(1599840)) {
		t.Fatalf("bid salah: %s", bid)
	}
}

func TestBuyUnitsRounding(t *testing.T) {
	ask := decimal.NewFromInt(1632160)
	u := BuyUnits(195000, ask) // ≈ 0.11948...
	if u.Exponent() < -18 {
		t.Fatalf("skala > 18: %s", u)
	}
	// units * ask tidak boleh melebihi amount (user tidak dapat lebih dari yang dibayar)
	if u.Mul(ask).GreaterThan(decimal.NewFromInt(195001)) {
		t.Fatalf("kelebihan: %s", u.Mul(ask))
	}
	if u.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("units nol: %s", u)
	}
}

func TestTickFreshnessWindow(t *testing.T) {
	// Simulasi umur tick vs staleness_coin_ms default sandbox 15000.
	fresh := time.Now().Add(-5 * time.Second)
	stale := time.Now().Add(-30 * time.Second)
	limit := int64(15000)
	if time.Since(fresh).Milliseconds() > limit {
		t.Fatal("tick 5 detik dianggap basi, salah")
	}
	if time.Since(stale).Milliseconds() <= limit {
		t.Fatal("tick 30 detik dianggap segar, salah")
	}
}

func TestHandleBookTickerMessage(t *testing.T) {
	// Pesan asli Binance (gabungan): b=bid price, B=bid qty, a=ask price, A=ask qty.
	// Parser wajib ambil b/a (harga), bukan B/A (qty). Regresi bug Fase 1.
	svc := New(nil, nil)
	msg := []byte(`{"stream":"bnbusdt@bookTicker","data":{"u":21104839130,"s":"BNBUSDT","b":"705.67000000","B":"9.14700000","a":"705.68000000","A":"12.39700000"}}`)
	svc.handleMessage(msg)
	svc.mu.RLock()
	tk, ok := svc.ticks["BNB"]
	svc.mu.RUnlock()
	if !ok {
		t.Fatal("tick BNB tidak masuk")
	}
	if !tk.bestBid.Equal(decimal.NewFromFloat(705.67)) {
		t.Fatalf("bid salah: %s, mau 705.67 (kemungkinan baca qty B)", tk.bestBid)
	}
	if !tk.bestAsk.Equal(decimal.NewFromFloat(705.68)) {
		t.Fatalf("ask salah: %s, mau 705.68 (kemungkinan baca qty A)", tk.bestAsk)
	}
	// Pesan terbalik (bid > ask) wajib ditolak, tidak meracuni cache.
	svc.handleMessage([]byte(`{"stream":"ethusdt@bookTicker","data":{"s":"ETHUSDT","b":"999","a":"1"}}`))
	svc.mu.RLock()
	_, ok = svc.ticks["ETH"]
	svc.mu.RUnlock()
	if ok {
		t.Fatal("tick terbalik bid>ask masuk cache, harusnya ditolak")
	}
}

// Package price: SATU-SATUNYA pintu harga (bab 11.2 plan).
// Sumber: Binance spot WS bookTicker gabungan (bnbusdt/ethusdt/solusdt) + 1 sumber kurs REST.
// Aturan keras: basi/down/tanpa timestamp = tolak quote, jangan fallback ke harga lama.
// Putus WS (termasuk serverShutdown 24 jam) langsung menandai harga basi.
package price

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/coder/websocket"
	"github.com/shopspring/decimal"
)

var (
	ErrStaleCoin   = errors.New("harga coin basi")
	ErrStaleFX     = errors.New("kurs basi")
	ErrSourceDown  = errors.New("sumber harga mati")
	ErrAssetPaused = errors.New("coin di-pause")
	ErrBadAsset    = errors.New("coin tidak dikenal")
)

var Assets = []string{"BNB", "ETH", "SOL"}

// harga dari bookTicker: mid = (bestBid + bestAsk) / 2, sesuai plan 7.1 (harga_coin_usdt).
type tick struct {
	bestBid, bestAsk decimal.Decimal
	at              time.Time
}

type Service struct {
	set *settings.Service
	q   *db.Queries

	mu    sync.RWMutex
	ticks map[string]tick // key: BNB/ETH/SOL (upper)
	lastSnap map[string]time.Time // throttle audit price_snapshots

	fxMu     sync.RWMutex
	fxRate   decimal.Decimal
	fxSource string
	fxAt     time.Time
	fxOK     bool
}

func New(set *settings.Service, q *db.Queries) *Service {
	return &Service{set: set, q: q, ticks: map[string]tick{}, lastSnap: map[string]time.Time{}}
}

func ValidAsset(a string) bool {
	a = strings.ToUpper(a)
	return a == "BNB" || a == "ETH" || a == "SOL"
}

// SetFX dipanggil job kurs REST (nama sumber fix di Stage 0; Fase 1: placeholder manual/1 sumber).
func (s *Service) SetFX(rate decimal.Decimal, source string) {
	s.fxMu.Lock()
	s.fxRate, s.fxSource, s.fxAt, s.fxOK = rate, source, time.Now(), true
	s.fxMu.Unlock()
}

// Snapshot untuk quote: fail-closed kalau basi. Mengembalikan umur data (ms) untuk disimpan di Quotes.
type Snapshot struct {
	Mid         decimal.Decimal
	Bid         decimal.Decimal
	Ask         decimal.Decimal
	Rate        decimal.Decimal
	RateSource  string
	PriceAgeMs  int64
	RateAgeMs   int64
	BinanceUSDT decimal.Decimal
}

func (s *Service) Snapshot(asset string) (Snapshot, error) {
	asset = strings.ToUpper(asset)
	if !ValidAsset(asset) {
		return Snapshot{}, ErrBadAsset
	}
	if paused, _ := s.set.Get("pause_" + strings.ToLower(asset)); paused == "spike_stale" || paused == "exposure" {
		// Catatan: pause exposure = beli saja yang ditolak (ditangani di risk/Fase 2).
		// Di level price, exposure tidak menolak snapshot; spike_stale menolak semua.
		if paused == "spike_stale" {
			return Snapshot{}, ErrAssetPaused
		}
	}
	s.mu.RLock()
	t, ok := s.ticks[asset]
	s.mu.RUnlock()
	if !ok {
		return Snapshot{}, ErrSourceDown
	}
	priceAge := time.Since(t.at)
	// FIX debug #17: jangan panic kalau key hilang — tolak quote dengan pesan jelas.
	staleCoin, err := s.set.SafeInt64("staleness_coin_ms")
	if err != nil {
		return Snapshot{}, ErrSourceDown
	}
	if priceAge.Milliseconds() > staleCoin {
		return Snapshot{}, ErrStaleCoin
	}
	s.fxMu.RLock()
	fx, src, fxAt, fxOK := s.fxRate, s.fxSource, s.fxAt, s.fxOK
	s.fxMu.RUnlock()
	if !fxOK {
		return Snapshot{}, ErrSourceDown
	}
	rateAge := time.Since(fxAt)
	staleFX, err := s.set.SafeInt64("staleness_fx_ms")
	if err != nil {
		return Snapshot{}, ErrSourceDown
	}
	if rateAge.Milliseconds() > staleFX {
		return Snapshot{}, ErrStaleFX
	}
	midUSDT := t.bestBid.Add(t.bestAsk).Div(decimal.NewFromInt(2))
	midIDR := midUSDT.Mul(fx)
	spreadBuy, err := s.set.SafeSpread(strings.ToLower(asset), "buy")
	if err != nil {
		return Snapshot{}, ErrSourceDown
	}
	spreadSell, err := s.set.SafeSpread(strings.ToLower(asset), "sell")
	if err != nil {
		return Snapshot{}, ErrSourceDown
	}
	one := decimal.NewFromInt(1)
	return Snapshot{
		Mid:         midIDR,
		Ask:         midIDR.Mul(one.Add(spreadBuy)),
		Bid:         midIDR.Mul(one.Sub(spreadSell)),
		Rate:        fx,
		RateSource:  src,
		PriceAgeMs:  priceAge.Milliseconds(),
		RateAgeMs:   rateAge.Milliseconds(),
		BinanceUSDT: midUSDT,
	}, nil
}

// Units beli: amount / ask, bulatkan 18 desimal DI GO (aturan Lampiran A.5).
func BuyUnits(amountIDR int64, ask decimal.Decimal) decimal.Decimal {
	return decimal.NewFromInt(amountIDR).Div(ask).Round(18)
}

// RunWS konek ke Binance spot bookTicker gabungan. Blokir sampai ctx batal.
// Setiap pesan update tick + tulis price_snapshots (best effort, jangan gagalkan loop).
// Putus/error = return; pemanggil menandai basi dengan TIDAK update tick (fail-closed otomatis)
// dan reconnect pakai backoff.
func (s *Service) RunWS(ctx context.Context, wsURL string) error {
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial ws: %w", err)
	}
	defer c.CloseNow()
	for {
		_, msg, err := c.Read(ctx)
		if err != nil {
			return fmt.Errorf("read ws: %w", err)
		}
		s.handleMessage(msg)
	}
}

// Format bookTicker gabungan: {"stream":"bnbusdt@bookTicker","data":{"b":"..","a":"..",...}}
// atau bookTicker tunggal: {"b":"..","a":"..","s":"BNBUSDT"}.
func (s *Service) handleMessage(msg []byte) {
	var env struct {
		Stream string `json:"stream"`
		Data   *struct {
			BidPrice string `json:"b"`
			BidQty   string `json:"B"`
			AskPrice string `json:"a"`
			AskQty   string `json:"A"`
			Symbol   string `json:"s"`
		} `json:"data"`
		BidPrice string `json:"b"`
		BidQty   string `json:"B"`
		AskPrice string `json:"a"`
		AskQty   string `json:"A"`
		Symbol   string `json:"s"`
	}
	if err := json.Unmarshal(msg, &env); err != nil {
		return
	}
	var bStr, aStr, sym string
	if env.Data != nil {
		bStr, aStr = env.Data.BidPrice, env.Data.AskPrice
		sym = env.Data.Symbol
		if sym == "" && env.Stream != "" {
			sym = strings.ToUpper(strings.Split(env.Stream, "@")[0])
		}
	} else {
		bStr, aStr, sym = env.BidPrice, env.AskPrice, env.Symbol
	}
	asset := strings.TrimSuffix(strings.ToUpper(sym), "USDT")
	if !ValidAsset(asset) {
		return
	}
	bid, err1 := decimal.NewFromString(bStr)
	ask, err2 := decimal.NewFromString(aStr)
	if err1 != nil || err2 != nil || bid.IsZero() || ask.IsZero() {
		return
	}
	// Sanity: bid harus <= ask. Kalau kebalik = parsing salah, tolak (jangan racuni cache).
	if bid.GreaterThan(ask) {
		return
	}
	s.mu.Lock()
	s.ticks[asset] = tick{bestBid: bid, bestAsk: ask, at: time.Now()}
	s.mu.Unlock()

	// Audit harga: tulis price_snapshots, throttle 1/menit per coin (FIX debug #1:
	// sebelumnya snap dihitung lalu dibuang, tabel audit kosong permanen).
	s.throttledSnapshot(asset)
}

// throttledSnapshot: tulis snapshot audit maksimal 1x per menit per coin (best effort).
func (s *Service) throttledSnapshot(asset string) {
	if s.q == nil {
		return
	}
	s.mu.Lock()
	last, ok := s.lastSnap[asset]
	if ok && time.Since(last) < time.Minute {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	snap, err := s.Snapshot(asset)
	if err != nil {
		return
	}
	spreadBuy := s.set.Spread(strings.ToLower(asset), "buy")
	spreadSell := s.set.Spread(strings.ToLower(asset), "sell")
	_, err = s.q.InsertPriceSnapshot(context.Background(), db.InsertPriceSnapshotParams{
		Asset:         asset,
		BinancePrice:  snap.BinanceUSDT,
		SpreadBuy:     spreadBuy,
		SpreadSell:    spreadSell,
		Ask:           snap.Ask,
		Bid:           snap.Bid,
		RateUsdtIdr:   snap.Rate,
		RateSource:    snap.RateSource,
		RateAgeMs:     snap.RateAgeMs,
		PriceAgeMs:    snap.PriceAgeMs,
	})
	if err != nil {
		return
	}
	s.mu.Lock()
	s.lastSnap[asset] = time.Now()
	s.mu.Unlock()
}

// CombinedStreamURL default Binance spot (plan Lampiran A.6).
func CombinedStreamURL() string {
	return "wss://stream.binance.com:9443/stream?streams=bnbusdt@bookTicker/ethusdt@bookTicker/solusdt@bookTicker"
}

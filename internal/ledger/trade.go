package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/price"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

var (
	ErrQuoteExpired  = errors.New("quote expired, minta requote")
	ErrQuoteUsed     = errors.New("quote sudah dipakai")
	ErrNoBalance     = errors.New("saldo tidak cukup")
	ErrBelowMinimum  = errors.New("di bawah minimum")
	ErrBadSide       = errors.New("side harus buy/sell")
)

// QuoteResult: quote yang disimpan (id untuk confirm).
type QuoteResult struct {
	ID         int64
	Side       string
	Asset      string
	AmountIDR  int64
	Units      decimal.Decimal
	Price      decimal.Decimal // ask (buy) / bid (sell)
	ExpiresAt  time.Time
	FeeNote    string // "tanpa fee provider di sandbox"
}

// CreateQuote: cek minimum + snapshot fail-closed + simpan row active + expire quote lama user.
// Jantung validasi harga ada di price.Snapshot (basi = error, bukan fallback).
func (s *Service) CreateQuote(ctx context.Context, userID int64, side, asset string, amountIDR int64, snap price.Snapshot) (QuoteResult, error) {
	side = strings.ToLower(side)
	asset = strings.ToUpper(asset)
	if side != "buy" && side != "sell" {
		return QuoteResult{}, ErrBadSide
	}
	if !price.ValidAsset(asset) {
		return QuoteResult{}, fmt.Errorf("coin tidak dikenal")
	}
	if amountIDR < s.set.Int64("min_buy_idr") {
		return QuoteResult{}, fmt.Errorf("%w: minimum Rp%s", ErrBelowMinimum, s.set.MustString("min_buy_idr"))
	}
	var units, px, spread decimal.Decimal
	if side == "buy" {
		px = snap.Ask
		spread = s.set.Spread(strings.ToLower(asset), "buy")
		units = price.BuyUnits(amountIDR, px)
	} else {
		px = snap.Bid
		spread = s.set.Spread(strings.ToLower(asset), "sell")
		// Jual: units = amount / bid, bulatkan 18 desimal. Cek saldo di ExecuteTrade (butuh lock).
		units = decimal.NewFromInt(amountIDR).Div(px).Round(18)
	}
	if units.LessThanOrEqual(decimal.Zero) {
		return QuoteResult{}, ErrBelowMinimum
	}
	lockSecs := s.set.Int64("quote_lock_seconds")
	expires := time.Now().Add(time.Duration(lockSecs) * time.Second)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return QuoteResult{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)

	// Expire quote active lama user biar cuma 1 yang hidup (confirm selalu ke yang terbaru).
	if _, err := q.ActiveQuoteForUser(ctx, userID); err == nil {
		_ = err
	}
	// Tandai semua active lain jadi expired (best effort dalam tx yang sama).
	_, _ = tx.Exec(ctx, `UPDATE quotes SET status='expired' WHERE user_id=$1 AND status='active'`, userID)

	row, err := q.CreateQuote(ctx, db.CreateQuoteParams{
		UserID:      userID,
		Side:        side,
		Asset:       asset,
		AmountIdr:   amountIDR,
		QuotedUnits: units,
		Bid:         snap.Bid,
		Ask:         snap.Ask,
		Mid:         snap.Mid,
		SpreadUsed:  spread,
		RateUsdtIdr: snap.Rate,
		RateSource:  snap.RateSource,
		RateAgeMs:   snap.RateAgeMs,
		PriceAgeMs:  snap.PriceAgeMs,
		ExpiresAt:   pgtype.Timestamptz{Time: expires, Valid: true},
	})
	if err != nil {
		return QuoteResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QuoteResult{}, err
	}
	return QuoteResult{ID: row.ID, Side: side, Asset: asset, AmountIDR: amountIDR, Units: units, Price: px, ExpiresAt: expires}, nil
}

// TradeResult: hasil eksekusi.
type TradeResult struct {
	TradeID   int64
	Side      string
	Asset     string
	AmountIDR int64
	Units     decimal.Decimal
	PnL       decimal.Decimal
}

// ExecuteTrade: konsumsi quote atomik + jurnal + cache, semua 1 tx.
// Urutan dalam tx: lock user → consume quote → cek saldo → jurnal → cache → trades row.
func (s *Service) ExecuteTrade(ctx context.Context, userID, quoteID int64) (TradeResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TradeResult{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)

	u, err := q.GetUserForUpdate(ctx, userID)
	if err != nil {
		return TradeResult{}, err
	}
	if u.Status == "blocked" {
		return TradeResult{}, ErrUserBlocked
	}
	// Konsumsi atomik: 0 baris = dipakai/expired/bukan milik user.
	cq, err := q.ConsumeQuote(ctx, db.ConsumeQuoteParams{ID: quoteID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TradeResult{}, ErrQuoteUsed
		}
		return TradeResult{}, err
	}
	side := strings.ToLower(cq.Side)
	asset := strings.ToUpper(cq.Asset)
	units := cq.QuotedUnits
	amount := decimal.NewFromInt(cq.AmountIdr)

	var priceUsed, midUsed, pnl decimal.Decimal
	var legs []leg
	if side == "buy" {
		priceUsed, midUsed = cq.Ask, cq.Mid
		pnl = cq.Ask.Sub(cq.Mid).Mul(units).Round(18)
		// Cek saldo IDR cukup (baca cache dalam tx; validasi final = jurnal).
		w, err := q.GetWallet(ctx, db.GetWalletParams{UserID: userID, Asset: "IDR"})
		if err != nil || w.Balance.LessThan(amount) {
			return TradeResult{}, ErrNoBalance
		}
		legs = []leg{
			{owner: userOwner(userID), asset: "IDR", amount: amount.Neg()},
			{owner: houseOwner("cash"), asset: "IDR", amount: amount},
			{owner: houseOwner("inventory"), asset: asset, amount: units.Neg()},
			{owner: userOwner(userID), asset: asset, amount: units},
		}
	} else {
		priceUsed, midUsed = cq.Bid, cq.Mid
		pnl = cq.Mid.Sub(cq.Bid).Mul(units).Round(18)
		w, err := q.GetWallet(ctx, db.GetWalletParams{UserID: userID, Asset: asset})
		if err != nil || w.Balance.LessThan(units) {
			return TradeResult{}, ErrNoBalance
		}
		legs = []leg{
			{owner: userOwner(userID), asset: asset, amount: units.Neg()},
			{owner: houseOwner("inventory"), asset: asset, amount: units},
			{owner: houseOwner("cash"), asset: "IDR", amount: amount.Neg()},
			{owner: userOwner(userID), asset: "IDR", amount: amount},
		}
	}
	jtype := "buy"
	if side == "sell" {
		jtype = "sell"
	}
	j, err := q.CreateJournal(ctx, db.CreateJournalParams{
		Type:     jtype,
		RefTable: pgtype.Text{String: "quotes", Valid: true},
		RefID:    pgtype.Int8{Int64: quoteID, Valid: true},
	})
	if err != nil {
		return TradeResult{}, err
	}
	for _, l := range legs {
		acct, err := ensureAccount(ctx, q, l.owner, l.asset)
		if err != nil {
			return TradeResult{}, err
		}
		if _, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			JournalID: j.ID, AccountID: acct.ID, Asset: l.asset, Amount: l.amount,
		}); err != nil {
			return TradeResult{}, err
		}
	}
	// Refresh cache yang berubah.
	if err := refreshWalletCache(ctx, q, userID, "IDR"); err != nil {
		return TradeResult{}, err
	}
	if err := refreshWalletCache(ctx, q, userID, asset); err != nil {
		return TradeResult{}, err
	}
	tr, err := q.CreateTrade(ctx, db.CreateTradeParams{
		UserID:      userID,
		QuoteID:     quoteID,
		Side:        side,
		Asset:       asset,
		AmountIdr:   cq.AmountIdr,
		Units:       units,
		PriceUsed:   priceUsed,
		MidUsed:     midUsed,
		SpreadPnlIdr: pnl,
		JournalID:   j.ID,
	})
	if err != nil {
		return TradeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TradeResult{}, err
	}
	return TradeResult{TradeID: tr.ID, Side: side, Asset: asset, AmountIDR: cq.AmountIdr, Units: units, PnL: pnl}, nil
}

type leg struct {
	owner  string
	asset  string
	amount decimal.Decimal
}

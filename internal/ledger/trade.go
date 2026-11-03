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
	ErrQuoteExpired = errors.New("quote expired, minta requote")
	ErrQuoteUsed    = errors.New("quote sudah dipakai")
	ErrNoBalance    = errors.New("saldo tidak cukup")
	ErrBelowMinimum = errors.New("di bawah minimum")
	ErrBadSide      = errors.New("side harus buy/sell")
)

// QuoteResult: quote yang disimpan (id untuk confirm).
// Fase 8 (v0.12): buy membawa FeeUnits + ToAddress + SentUnits (quote 3 baris).
type QuoteResult struct {
	ID        int64
	Side      string
	Asset     string
	AmountIDR int64
	Units     decimal.Decimal // units bruto (sebelum fee)
	FeeUnits  decimal.Decimal // fee network (Fase 8: fee_mock_*)
	SentUnits decimal.Decimal // units − fee = yang sampai ke wallet user
	ToAddress string          // alamat tujuan (buy wajib, sell kosong)
	Price     decimal.Decimal // ask (buy) / bid (sell)
	ExpiresAt time.Time
	FeeNote   string // "tanpa fee di sandbox"
}

// CreateQuote: kompat buy-only (Phase 6). Sell via deposit (/jual).
func (s *Service) CreateQuote(ctx context.Context, userID int64, side, asset string, amountIDR int64, snap price.Snapshot) (QuoteResult, error) {
	side = strings.ToLower(side)
	if side != "buy" {
		return QuoteResult{}, fmt.Errorf("side harus buy (sell via deposit: /jual)")
	}
	return s.createQuoteInner(ctx, userID, side, asset, amountIDR, decimal.Zero, "", snap)
}

// CreateQuoteBuy: buy transit Fase 8 — fee + alamat wajib, tolak fee > 20%.
func (s *Service) CreateQuoteBuy(ctx context.Context, userID int64, asset string, amountIDR int64, feeUnits decimal.Decimal, toAddress string, snap price.Snapshot) (QuoteResult, error) {
	if strings.TrimSpace(toAddress) == "" {
		return QuoteResult{}, fmt.Errorf("alamat wallet belum ada — daftarin dulu via /alamat")
	}
	return s.createQuoteInner(ctx, userID, "buy", asset, amountIDR, feeUnits, toAddress, snap)
}

func (s *Service) createQuoteInner(ctx context.Context, userID int64, side, asset string, amountIDR int64, feeUnits decimal.Decimal, toAddress string, snap price.Snapshot) (QuoteResult, error) {
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
		// Fase 8: fee > 20% units = tolak (nominal terlalu kecil untuk fee).
		if !feeUnits.IsZero() {
			rejectPct := decimal.NewFromFloat(0.2)
			if v, err := s.set.SafeDecimal("fee_reject_pct"); err == nil {
				rejectPct = v
			}
			if feeUnits.Div(units).GreaterThan(rejectPct) {
				return QuoteResult{}, fmt.Errorf("nominal terlalu kecil untuk fee network")
			}
		}
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

	// FIX debug #7: cek cepat saldo beli di awal (non-lock, best effort UX) — gagal di sini
	// lebih jelas daripada gagal pas confirm. Lock beneran tetap di ExecuteTrade.
	// Jual tidak dicek di sini (units tergantung bid yang bisa geser; cek di ExecuteTrade).
	if side == "buy" {
		if w, err := q.GetWallet(ctx, db.GetWalletParams{UserID: userID, Asset: "IDR"}); err != nil || w.Balance.LessThan(decimal.NewFromInt(amountIDR)) {
			return QuoteResult{}, ErrNoBalance
		}
	}

	// FIX debug #9: expire quote active lama user biar cuma 1 yang hidup.
	// (Sebelumnya ada panggilan ActiveQuoteForUser yang hasilnya dibuang — dead code, dihapus.)
	_, _ = tx.Exec(ctx, `UPDATE quotes SET status='expired' WHERE user_id=$1 AND status='active'`, userID)

	// Fase 8: kolom fee_units + to_address (kolom baru migrasi 000006).
	// Tulis via SQL langsung biar tidak tergantung urutan regen sqlc.
	var rowID int64
	var rowExpires time.Time
	err = tx.QueryRow(ctx, `INSERT INTO quotes
	  (user_id, side, asset, amount_idr, quoted_units, fee_units, to_address, bid, ask, mid, spread_used,
	   rate_usdt_idr, rate_source, rate_age_ms, price_age_ms, expires_at)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	RETURNING id, expires_at`,
		userID, side, asset, amountIDR, units.String(), feeUnits.String(), toAddress,
		snap.Bid.String(), snap.Ask.String(), snap.Mid.String(), spread.String(),
		snap.Rate.String(), snap.RateSource, snap.RateAgeMs, snap.PriceAgeMs, expires,
	).Scan(&rowID, &rowExpires)
	if err != nil {
		return QuoteResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QuoteResult{}, err
	}
	sent := units.Sub(feeUnits)
	if side != "buy" {
		sent = decimal.Zero
	}
	return QuoteResult{ID: rowID, Side: side, Asset: asset, AmountIDR: amountIDR, Units: units, FeeUnits: feeUnits, SentUnits: sent, ToAddress: toAddress, Price: px, ExpiresAt: rowExpires, FeeNote: "fee network ditanggung user"}, nil
}

// TradeResult: hasil eksekusi.
// Fase 8: buy membawa FeeUnits + SentUnits + ToAddress (buat payout mock).
type TradeResult struct {
	TradeID   int64
	PayoutID  int64 // Phase 4: payout queued atomik (buy saja, 0 untuk sell)
	Side      string
	Asset     string
	AmountIDR int64
	Units     decimal.Decimal
	FeeUnits  decimal.Decimal
	SentUnits decimal.Decimal
	ToAddress string
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
	// Fase 8: baca fee_units + to_address sekalian (kolom baru migrasi 000006).
	var cq struct {
		Side      string
		Asset     string
		AmountIdr int64
		Units     decimal.Decimal
		FeeUnits  decimal.Decimal
		ToAddress string
		Bid, Ask  decimal.Decimal
		Mid       decimal.Decimal
	}
	var qUnits, qFee, qBid, qAsk, qMid string
	var qAddr string
	err = tx.QueryRow(ctx, `UPDATE quotes SET status='used'
	WHERE id=$1 AND user_id=$2 AND status='active' AND expires_at > now()
	RETURNING side, asset, amount_idr, quoted_units, fee_units, to_address, bid, ask, mid`,
		quoteID, userID,
	).Scan(&cq.Side, &cq.Asset, &cq.AmountIdr, &qUnits, &qFee, &qAddr, &qBid, &qAsk, &qMid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TradeResult{}, ErrQuoteUsed
		}
		return TradeResult{}, err
	}
	var perr error
	if cq.Units, perr = decimal.NewFromString(qUnits); perr != nil {
		return TradeResult{}, perr
	}
	if cq.FeeUnits, perr = decimal.NewFromString(qFee); perr != nil {
		return TradeResult{}, perr
	}
	if cq.Bid, perr = decimal.NewFromString(qBid); perr != nil {
		return TradeResult{}, perr
	}
	if cq.Ask, perr = decimal.NewFromString(qAsk); perr != nil {
		return TradeResult{}, perr
	}
	if cq.Mid, perr = decimal.NewFromString(qMid); perr != nil {
		return TradeResult{}, perr
	}
	cq.ToAddress = qAddr
	side := strings.ToLower(cq.Side)
	asset := strings.ToUpper(cq.Asset)
	units := cq.Units
	feeUnits := cq.FeeUnits
	sentUnits := units.Sub(feeUnits)
	amount := decimal.NewFromInt(cq.AmountIdr)

	var priceUsed, midUsed, pnl decimal.Decimal
	var legs []leg
	// Phase 6 (A10): satu-satunya side adalah buy. Sell-bandar mati total;
	// sell via deposit transit (MockDeposit). Quote sell lama ditolak di sini.
	if side != "buy" {
		return TradeResult{}, fmt.Errorf("side harus buy (sell via deposit: /jual)")
	}
	priceUsed, midUsed = cq.Ask, cq.Mid
	pnl = cq.Ask.Sub(cq.Mid).Mul(units).Round(18)
	// FIX debug #3: kunci row wallet (FOR UPDATE) sebelum cek — dua quote beda yang
	// dieksekusi paralel sekarang antre di lock yang sama, tidak bisa dua-duanya lolos.
	w, err := q.GetWalletForUpdate(ctx, db.GetWalletForUpdateParams{UserID: userID, Asset: "IDR"})
	if err != nil || w.Balance.LessThan(amount) {
		return TradeResult{}, ErrNoBalance
	}
	legs = []leg{
		{owner: userOwner(userID), asset: "IDR", amount: amount.Neg()},
		{owner: houseOwner("cash"), asset: "IDR", amount: amount},
		// Phase 3 transit balance: float keluar GROSS, sent ke boundary
		// external:payout, fee ke fee-collect. Sum coin = -gross+sent+fee = 0.
		{owner: houseOwner("float"), asset: asset, amount: units.Neg()},
		{owner: externalOwner("payout"), asset: asset, amount: sentUnits},
		{owner: houseOwner("fee-collect"), asset: asset, amount: feeUnits},
	}
	jtype := "buy"
	// Phase 3: kunci invariant sebelum tulis — corrupt/missing leg = rollback.
	if err := validateLegs(legs); err != nil {
		return TradeResult{}, err
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
	// Refresh cache user (semua asset sekaligus, 1 query targeted).
	if err := refreshWalletCache(ctx, q, userID); err != nil {
		return TradeResult{}, err
	}
	tr, err := q.CreateTrade(ctx, db.CreateTradeParams{
		UserID:       userID,
		QuoteID:      quoteID,
		Side:         side,
		Asset:        asset,
		AmountIdr:    cq.AmountIdr,
		Units:        units,
		PriceUsed:    priceUsed,
		MidUsed:      midUsed,
		SpreadPnlIdr: pnl,
		JournalID:    j.ID,
	})
	if err != nil {
		return TradeResult{}, err
	}
	// Phase 4 (A2): payout row queued ATOMIK dengan trade — tidak ada trade
	// buy tanpa kewajiban payout durable. Processor jalan sesudah commit.
	var payoutID int64
	if side == "buy" {
		// 1 coin = 1 network tetap (mirror chain.NetworkFor tanpa import
		// agar arah dependensi ledger->chain tidak muncul).
		net := map[string]string{"BNB": "BSC", "ETH": "ERC20", "SOL": "SOL"}[asset]
		payoutID, err = q.CreatePayoutQueued(ctx, db.CreatePayoutQueuedParams{
			UserID:    userID,
			TradeID:   tr.ID,
			Asset:     asset,
			Units:     units,
			FeeUnits:  feeUnits,
			ToAddress: cq.ToAddress,
			Network:   net,
		})
		if err != nil {
			return TradeResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return TradeResult{}, err
	}
	return TradeResult{TradeID: tr.ID, Side: side, Asset: asset, AmountIDR: cq.AmountIdr, Units: units, FeeUnits: feeUnits, SentUnits: sentUnits, ToAddress: cq.ToAddress, PnL: pnl, PayoutID: payoutID}, nil
}

type leg struct {
	owner  string
	asset  string
	amount decimal.Decimal
}

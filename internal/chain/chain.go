// Package chain: SATU-SATUNYA pintu on-chain (plan v0.12 bab 11).
// Fase 8 = mock: ValidateAddress (format), EstimateFee (fee_mock_*),
// Payout (txhash MOCK-xxx, selalu confirmed). Node beneran = Fase 10.
// Modul lain dilarang broadcast langsung.
package chain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func pgText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// NumericToDecimal: pgtype.Numeric (kolom GENERATED sqlc) → decimal.
// Invalid/NaN/nil = zero. Dipakai bot + adminwallet.
func NumericToDecimal(value pgtype.Numeric) decimal.Decimal {
	if !value.Valid || value.NaN || value.Int == nil {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(value.Int, -value.Exp)
}

type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	set  *settings.Service

	// Test-only fault seam: runs after confirmed UPDATE and before chain-event INSERT.
	// Both statements share one DB transaction, so an error must roll back both.
	beforeConfirmedEvent func() error
}

func New(pool *pgxpool.Pool, q *db.Queries, set *settings.Service) *Service {
	return &Service{pool: pool, q: q, set: set}
}

// NetworkFor: 1 coin = 1 network tetap (tanpa pilihan di MVP).
func NetworkFor(asset string) (string, bool) {
	switch strings.ToUpper(asset) {
	case "BNB":
		return "BSC", true
	case "ETH":
		return "ERC20", true
	case "SOL":
		return "SOL", true
	}
	return "", false
}

// ValidateAddress: format per network, tanpa node.
func ValidateAddress(asset, addr string) error {
	addr = strings.TrimSpace(addr)
	net, ok := NetworkFor(asset)
	if !ok {
		return fmt.Errorf("coin tidak dikenal")
	}
	switch net {
	case "BSC", "ERC20":
		if len(addr) != 42 || !strings.HasPrefix(addr, "0x") {
			return fmt.Errorf("alamat %s harus 0x + 40 hex", net)
		}
		for _, c := range addr[2:] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return fmt.Errorf("alamat %s mengandung char bukan hex", net)
			}
		}
		return nil
	case "SOL":
		if err := validateSolAddress(addr); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("network tidak dikenal")
}

// EstimateFee: Stage 1 = fee_mock_* dari Settings. Stage 2 = estimator node.
func (s *Service) EstimateFee(asset string) (decimal.Decimal, error) {
	asset = strings.ToUpper(asset)
	if _, ok := NetworkFor(asset); !ok {
		return decimal.Zero, fmt.Errorf("coin tidak dikenal")
	}
	return s.set.SafeDecimal("fee_mock_" + strings.ToLower(asset))
}

// FeeRejectPct: ambang tolak fee vs units (default 0.2 = 20%).
func (s *Service) FeeRejectPct() decimal.Decimal {
	if v, err := s.set.SafeDecimal("fee_reject_pct"); err == nil {
		return v
	}
	return decimal.NewFromFloat(0.2)
}

// PayoutMode: skenario mock processor (revisi V.1 G4.4, test-controlled).
// Production nanti = hasil broadcast nyata, bukan parameter.
type PayoutMode string

const (
	PayoutSuccess PayoutMode = "success" // queued→broadcasting→confirmed + txhash
	PayoutFailed  PayoutMode = "failed"  // →failed + refund journal tepat sekali
	PayoutUnknown PayoutMode = "unknown" // →unknown, tahan, JANGAN refund
	PayoutDelayed PayoutMode = "delayed" // tetap queued (worker tick berikutnya)
)

// Sentinel: caller (bot/worker) orkestrasi refund via ledger — chain TIDAK
// menulis jurnal (ledger satu-satunya pintu mutasi uang, bab 11.2).
var (
	ErrPayoutFailed  = errors.New("payout gagal")
	ErrPayoutUnknown = errors.New("payout perlu pengecekan — jangan ulang transaksi")
	ErrPayoutDelayed = errors.New("payout tertunda — diproses worker berikutnya")
)

// Payout: kompat Fase 8 — pastikan payout queued ada lalu proses success.
// Sejak Phase 4, ExecuteTrade sudah membuat payout queued atomik; fungsi ini
// hanya ensure (ON CONFLICT DO NOTHING) lalu memprosesnya.
func (s *Service) Payout(ctx context.Context, userID, tradeID int64, asset string, units, feeUnits decimal.Decimal, toAddr string) (string, error) {
	p, err := s.ensureQueued(ctx, userID, tradeID, asset, units, feeUnits, toAddr)
	if err != nil {
		return "", err
	}
	return s.ProcessPayout(ctx, p.ID, PayoutSuccess)
}

// ensureQueued: buat payout queued bila belum ada (idempotent by trade_id).
func (s *Service) ensureQueued(ctx context.Context, userID, tradeID int64, asset string, units, feeUnits decimal.Decimal, toAddr string) (db.GetPayoutRow, error) {
	asset = strings.ToUpper(asset)
	net, ok := NetworkFor(asset)
	if !ok {
		return db.GetPayoutRow{}, fmt.Errorf("coin tidak dikenal")
	}
	sent := units.Sub(feeUnits)
	if sent.LessThanOrEqual(decimal.Zero) {
		return db.GetPayoutRow{}, fmt.Errorf("fee menelan seluruh units")
	}
	// Trade sudah punya payout (jalur normal Phase 4) = pakai itu.
	if p, err := s.q.GetPayoutByTrade(ctx, tradeID); err == nil {
		return payoutFromTradeRow(p), nil
	}
	pid, err := s.q.CreatePayoutQueued(ctx, db.CreatePayoutQueuedParams{
		UserID: userID, TradeID: tradeID, Asset: asset,
		Units: units, FeeUnits: feeUnits,
		ToAddress: toAddr, Network: net,
	})
	if err != nil {
		// Race: orang lain bikin duluan → ambil yang ada.
		if p, err2 := s.q.GetPayoutByTrade(ctx, tradeID); err2 == nil {
			return payoutFromTradeRow(p), nil
		}
		return db.GetPayoutRow{}, err
	}
	return s.q.GetPayout(ctx, pid)
}

// ErrPayoutStaleTransition: state transition kena 0 row — payout sudah pindah
// state oleh worker lain (stale/racing transition). Bukan error operasional,
// tapi sinyal kalah race. Task 5 CF-04.
var ErrPayoutStaleTransition = errors.New("payout stale transition: state sudah berubah")

// mustWin: execrows harus menang tepat ≥1 row, else ErrPayoutStaleTransition.
func mustWin(n int64, err error) error {
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPayoutStaleTransition
	}
	return nil
}

// ProcessPayout: mock processor state machine (G4.4). Idempotent per payout ID.
// success: confirmed + MOCK txhash. failed: failed + refund once.
// unknown: unknown + hold. delayed: tetap queued (attempt+1 tanpa transisi).
func (s *Service) ProcessPayout(ctx context.Context, payoutID int64, mode PayoutMode) (string, error) {
	p, err := s.q.GetPayout(ctx, payoutID)
	if err != nil {
		return "", err
	}
	// Terminal sudah final = kembalikan hasil lama (exactly-once).
	if p.Status == "confirmed" {
		if p.TxHash.Valid {
			return p.TxHash.String, nil
		}
		return "", fmt.Errorf("payout confirmed tanpa txhash")
	}
	if p.Status == "failed" {
		return "", ErrPayoutFailed
	}
	if p.Status == "unknown" && mode != PayoutSuccess && mode != PayoutFailed {
		return "", ErrPayoutUnknown
	}
	n, err := s.q.MarkPayoutBroadcasting(ctx, p.ID)
	if err := mustWin(n, err); err != nil {
		return "", err
	}

	switch mode {
	case PayoutDelayed:
		return "", ErrPayoutDelayed
	case PayoutUnknown:
		n, err := s.q.MarkPayoutUnknown(ctx, db.MarkPayoutUnknownParams{ID: p.ID, LastError: "mock unknown (sandbox)"})
		if err := mustWin(n, err); err != nil {
			return "", err
		}
		return "", ErrPayoutUnknown
	case PayoutFailed:
		return s.failPayout(ctx, p)
	default: // PayoutSuccess
		return s.confirmPayoutWithEvent(ctx, p)
	}
}

// confirmPayoutWithEvent commits the terminal payout state and its audit event
// atomically. A failed event insert must not leave a confirmed payout without
// the corresponding chain event.
func (s *Service) confirmPayoutWithEvent(ctx context.Context, p db.GetPayoutRow) (string, error) {
	txHash := fmt.Sprintf("MOCK-%d-%d", p.TradeID, p.AttemptCount+1)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)

	n, err := q.MarkPayoutConfirmed(ctx, db.MarkPayoutConfirmedParams{
		ID: p.ID, TxHash: pgText(txHash), Confirms: 1,
	})
	if err := mustWin(n, err); err != nil {
		return "", err
	}
	if s.beforeConfirmedEvent != nil {
		if err := s.beforeConfirmedEvent(); err != nil {
			return "", err
		}
	}
	if _, err := q.InsertChainEvent(ctx, db.InsertChainEventParams{
		Asset: p.Asset, TxHash: txHash, EventType: "mock_confirmed",
		RawPayload: "sandbox mock, bukan chain beneran", Confirms: 1,
	}); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return txHash, nil
}

// failPayout: tandai failed. Zero-row → ErrPayoutStaleTransition (Task 5 CF-04/05).
// Refund DIORKESTRASI caller via ledger agar jurnal hanya ditulis pintu ledger.
// unknown TIDAK pernah memanggil ini.
func (s *Service) failPayout(ctx context.Context, p db.GetPayoutRow) (string, error) {
	// Sudah failed + refund = idempotent, tanpa jurnal kedua.
	if cur, err := s.q.GetPayout(ctx, p.ID); err == nil && cur.Status == "failed" && cur.RefundJournalID.Valid {
		return "", ErrPayoutFailed
	}
	n, err := s.q.MarkPayoutFailed(ctx, db.MarkPayoutFailedParams{ID: p.ID, LastError: "mock failed (sandbox)"})
	if err := mustWin(n, err); err != nil {
		return "", err
	}
	return "", ErrPayoutFailed
}

// SetRefundJournal: catat refund journal (ditulis ledger) ke payout.
// Guard IS NULL di SQL = exactly-once walau retry.
// Zero-row → ErrPayoutStaleTransition (Task 5). Deprecated: Task 3 RefundPayout
// atomic sudah linkage dalam tx yang sama; ini hanya untuk kompat caller lama.
func (s *Service) SetRefundJournal(ctx context.Context, payoutID, journalID int64) error {
	n, err := s.q.SetPayoutRefund(ctx, db.SetPayoutRefundParams{
		ID: payoutID, RefundJournalID: pgtype.Int8{Int64: journalID, Valid: true},
	})
	return mustWin(n, err)
}

func payoutFromTradeRow(p db.GetPayoutByTradeRow) db.GetPayoutRow {
	return db.GetPayoutRow{
		ID: p.ID, UserID: p.UserID, TradeID: p.TradeID, Asset: p.Asset,
		Units: p.Units, FeeUnits: p.FeeUnits, SentUnits: p.SentUnits,
		ToAddress: p.ToAddress, Network: p.Network, TxHash: p.TxHash,
		Status: p.Status, Confirms: p.Confirms,
	}
}

// solBase58Alphabet: Bitcoin/Base58 tanpa 0, O, I, l (revisi V.1 A18).
const solBase58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// validateSolAddress: base58 decode manual, harus tepat 32 byte public key.
// Tanpa network call, tanpa dep baru. Tidak panic untuk input apapun.
func validateSolAddress(addr string) error {
	if len(addr) < 32 || len(addr) > 44 {
		return fmt.Errorf("alamat SOL harus 32–44 char base58")
	}
	// Leading '1' = zero byte (base58 nol). Hitung dulu agar tidak hilang.
	leading := 0
	for leading < len(addr) && addr[leading] == '1' {
		leading++
	}
	decoded := make([]byte, 0, 32)
	for _, c := range addr {
		idx := -1
		for i := 0; i < len(solBase58Alphabet); i++ {
			if byte(c) == solBase58Alphabet[i] {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("alamat SOL mengandung char bukan base58: %q", string(c))
		}
		carry := idx
		for j := len(decoded) - 1; j >= 0; j-- {
			v := int(decoded[j])*58 + carry
			decoded[j] = byte(v & 0xff)
			carry = v >> 8
		}
		for carry > 0 {
			decoded = append([]byte{byte(carry & 0xff)}, decoded...)
			carry >>= 8
		}
	}
	// Total byte = leading zero ('1') + decoded value (minimal representation,
	// tanpa nol depan). Public key valid = tepat 32.
	if leading+len(decoded) == 32 {
		return nil
	}
	return fmt.Errorf("alamat SOL tidak 32 byte")
}

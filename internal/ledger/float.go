// Float topup Fase 8+ (plan v0.13 #43): isi ulang hot-wallet dari cold owner.
// Mock-ready: tanpa key asli — catat transfer manual cold→hot lewat jurnal float-in.
// Aturan: units > 0, asset BNB/ETH/SOL, jurnal 1 tx (house:float +units, house:cash IDR 0? TIDAK —
// float-in murni coin masuk, tanpa kaki IDR; sum per aset coin = +units satu kaki? DILARANG.
// Double-entry: float-in = house:cold −units, house:float +units (cold tercatat sebagai akun house).
package ledger

import (
	"context"
	"fmt"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/price"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// FloatTopupResult: hasil isi ulang float.
type FloatTopupResult struct {
	JournalID int64
	Asset     string
	Units     decimal.Decimal
}

// FloatTopup: owner transfer manual cold→hot, tercatat jurnal float-in.
// Kaki: house:cold coin −units, house:float coin +units. Sum per aset = 0.
func (s *Service) FloatTopup(ctx context.Context, asset string, units decimal.Decimal) (FloatTopupResult, error) {
	asset = strings.ToUpper(asset)
	if !price.ValidAsset(asset) {
		return FloatTopupResult{}, fmt.Errorf("coin harus BNB/ETH/SOL")
	}
	if units.LessThanOrEqual(decimal.Zero) {
		return FloatTopupResult{}, fmt.Errorf("units harus positif")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FloatTopupResult{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)

	j, err := q.CreateJournal(ctx, db.CreateJournalParams{
		Type:     "float-in",
		RefTable: pgtype.Text{String: "float", Valid: true},
		RefID:    pgtype.Int8{Valid: false},
	})
	if err != nil {
		return FloatTopupResult{}, err
	}
	for _, l := range []leg{
		{owner: houseOwner("cold"), asset: asset, amount: units.Neg()},
		{owner: houseOwner("float"), asset: asset, amount: units},
	} {
		acct, err := ensureAccount(ctx, q, l.owner, l.asset)
		if err != nil {
			return FloatTopupResult{}, err
		}
		if _, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			JournalID: j.ID, AccountID: acct.ID, Asset: l.asset, Amount: l.amount,
		}); err != nil {
			return FloatTopupResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return FloatTopupResult{}, err
	}
	return FloatTopupResult{JournalID: j.ID, Asset: asset, Units: units}, nil
}

// ReplaceWalletAddress: ganti alamat wallet user + audit actual before/after
// dalam SATU tx (revisi V.1 A17). First set: before "(kosong)".
// Failure di titik mana pun = rollback, tanpa audit sukses palsu.
package ledger

import (
	"context"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) ReplaceWalletAddress(ctx context.Context, userID int64, telegramID int64, asset, address, network string) (before, after string, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)

	if u, err := q.GetUserForUpdate(ctx, userID); err != nil || u.Status == "blocked" {
		if err == nil {
			return "", "", ErrUserBlocked
		}
		return "", "", err
	}
	before = "(kosong)"
	if old, err := q.GetWalletAddress(ctx, db.GetWalletAddressParams{UserID: userID, Asset: asset}); err == nil {
		before = asset + "=" + old.Address
	}
	row, err := q.UpsertWalletAddress(ctx, db.UpsertWalletAddressParams{
		UserID: userID, Asset: asset, Address: address, Network: network,
	})
	if err != nil {
		return "", "", err
	}
	after = asset + "=" + row.Address
	if _, err := q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{
		AdminTelegramID: telegramID,
		Action:          "alamat",
		TargetID:        pgtype.Int8{Int64: userID, Valid: true},
		Before:          before,
		After:           after,
	}); err != nil {
		return "", "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return before, after, nil
}

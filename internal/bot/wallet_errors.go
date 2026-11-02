package bot

import (
	"errors"
	"strings"

	"github.com/pikipici/crypto-station/internal/ledger"
)

// Predikat error ledger untuk copy UX wallet (§8 revisi).
// Dipisah agar decision logic dapat diuji tanpa Telegram API.

func isStockConflict(err error) bool {
	return errors.Is(err, ledger.ErrStockDepositRefConflict)
}

func isStockCancelled(err error) bool {
	return errors.Is(err, ledger.ErrStockDepositCancelled)
}

func isStockUnauthorized(err error) bool {
	return errors.Is(err, ledger.ErrStockDepositUnauthorized)
}

func containsAuditFailure(msg string) bool {
	return strings.Contains(msg, "audit")
}

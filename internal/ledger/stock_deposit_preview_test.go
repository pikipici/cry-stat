package ledger

import (
	"context"
	"errors"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/shopspring/decimal"
)

type previewQueriesFake struct {
	created   db.AdminStockDeposit
	existing  db.AdminStockDeposit
	createErr error
}

func (f previewQueriesFake) CreatePendingAdminStockDeposit(context.Context, db.CreatePendingAdminStockDepositParams) (db.AdminStockDeposit, error) {
	return f.created, f.createErr
}
func (f previewQueriesFake) GetAdminStockDepositByRef(context.Context, string) (db.AdminStockDeposit, error) {
	return f.existing, nil
}

func previewInput() RecordStockDepositInput {
	return RecordStockDepositInput{AdminTelegramID: 7, Asset: "SOL", Units: decimal.RequireFromString("1.25"), ToAddress: "hot", TxRef: "MOCK-one"}
}
func previewRow(status string) db.AdminStockDeposit {
	in := previewInput()
	return db.AdminStockDeposit{ID: 4, AdminTelegramID: in.AdminTelegramID, Asset: in.Asset, Network: "SOL", Units: in.Units, ToAddress: in.ToAddress, TxRef: in.TxRef, Status: status}
}

func TestCreateAdminStockDepositPreviewStates(t *testing.T) {
	in := previewInput()
	got, err := createAdminStockDepositPreview(context.Background(), previewQueriesFake{created: previewRow("pending_confirm")}, in)
	if err != nil || got.State != StockDepositPreviewCreated {
		t.Fatalf("created: %+v %v", got, err)
	}
	for _, tc := range []struct {
		status string
		want   StockDepositPreviewState
	}{{"pending_confirm", StockDepositPreviewIdenticalPending}, {"recorded_mock", StockDepositPreviewAlreadyRecorded}, {"cancelled", StockDepositPreviewCancelled}, {"other", StockDepositPreviewConflict}} {
		got, err = createAdminStockDepositPreview(context.Background(), previewQueriesFake{createErr: errors.New("duplicate"), existing: previewRow(tc.status)}, in)
		if err != nil || got.State != tc.want || got.Deposit.ID != 4 {
			t.Fatalf("%s: %+v %v", tc.status, got, err)
		}
	}
}

func TestCreateAdminStockDepositPreviewConflictAndUnauthorized(t *testing.T) {
	in := previewInput()
	row := previewRow("pending_confirm")
	row.Units = decimal.NewFromInt(9)
	got, err := createAdminStockDepositPreview(context.Background(), previewQueriesFake{createErr: errors.New("duplicate"), existing: row}, in)
	if err != nil || got.State != StockDepositPreviewConflict {
		t.Fatalf("conflict: %+v %v", got, err)
	}
	row = previewRow("pending_confirm")
	row.AdminTelegramID = 8
	got, err = createAdminStockDepositPreview(context.Background(), previewQueriesFake{createErr: errors.New("duplicate"), existing: row}, in)
	if err != nil || got.State != StockDepositPreviewUnauthorized || got.Deposit.ID != 0 {
		t.Fatalf("unauthorized leaked: %+v %v", got, err)
	}
}

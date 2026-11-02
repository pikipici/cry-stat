package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"
)

type fakeWalletSender struct {
	edits, denied int
	lastText      string
}

func (f *fakeWalletSender) editText(_ context.Context, _ int64, _ int, text string, _ *models.InlineKeyboardMarkup) {
	f.edits++
	f.lastText = text
}
func (f *fakeWalletSender) sendText(context.Context, int64, string)                                 {}
func (f *fakeWalletSender) sendInline(context.Context, int64, string, *models.InlineKeyboardMarkup) {}
func (f *fakeWalletSender) answerDenied(context.Context, string)                                    { f.denied++ }

type fakeWalletMutation struct{ confirms, cancels int }

func (f *fakeWalletMutation) ConfirmWalletDeposit(context.Context, int64, int64) (string, *models.InlineKeyboardMarkup) {
	f.confirms++
	return "confirmed", nil
}
func (f *fakeWalletMutation) CancelWalletDeposit(context.Context, int64, int64) (string, *models.InlineKeyboardMarkup) {
	f.cancels++
	return "cancelled", nil
}

func TestRouteWalletMutationCallbackBehavior(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name        string
		decision    walletCallbackDecision
		wantHandle  bool
		wantEdit    int
		wantDeny    int
		wantConfirm int
		wantCancel  int
	}{
		{name: "deny has no mutation", decision: walletCallbackDecision{policy: "deny"}, wantHandle: true, wantDeny: 1},
		{name: "not ready", decision: walletCallbackDecision{policy: "ok", action: "not-ready"}, wantHandle: true, wantEdit: 1},
		{name: "confirm once", decision: walletCallbackDecision{policy: "ok", action: "confirm", value: "a:wdepok:12"}, wantHandle: true, wantEdit: 1, wantConfirm: 1},
		{name: "cancel once", decision: walletCallbackDecision{policy: "ok", action: "cancel", value: "a:wdepno:12"}, wantHandle: true, wantEdit: 1, wantCancel: 1},
		{name: "malformed explicit stale", decision: walletCallbackDecision{policy: "ok", action: "confirm", value: "a:wdepok:nope"}, wantHandle: true, wantEdit: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sender := &fakeWalletSender{}
			svc := &fakeWalletMutation{}
			got := routeWalletMutationCallback(ctx, sender, tc.decision, "cb", 1, 2, 3, svc)
			if got != tc.wantHandle || sender.edits != tc.wantEdit || sender.denied != tc.wantDeny || svc.confirms != tc.wantConfirm || svc.cancels != tc.wantCancel {
				t.Fatalf("handled=%v edits=%d denied=%d confirms=%d cancels=%d", got, sender.edits, sender.denied, svc.confirms, svc.cancels)
			}
		})
	}
}

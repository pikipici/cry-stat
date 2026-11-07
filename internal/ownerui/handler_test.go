package ownerui

import "testing"

func TestResearchMenuContainsOnlyResearchCapabilities(t *testing.T) {
	kb := Keyboard()
	want := [][]string{{"Research", "Watchlist"}, {"Skill", "Runs"}, {"System", "Pause"}}
	if len(kb.Keyboard) != len(want) {
		t.Fatalf("rows=%d want=%d", len(kb.Keyboard), len(want))
	}
	for i := range want {
		for j := range want[i] {
			if kb.Keyboard[i][j].Text != want[i][j] {
				t.Fatalf("button[%d][%d]=%q want=%q", i, j, kb.Keyboard[i][j].Text, want[i][j])
			}
		}
	}
	for _, row := range kb.Keyboard {
		for _, button := range row {
			if button.Text == "Buy" || button.Text == "Sell" || button.Text == "Auto" || button.Text == "Beli" || button.Text == "Jual" {
				t.Fatalf("trading button exposed: %q", button.Text)
			}
		}
	}
}

func TestDecideOwnerPrivateResearchShell(t *testing.T) {
	cases := map[string]string{
		"/start":    HomeCopy,
		"Research":  ResearchCopy,
		"/research": ResearchCopy,
		"Watchlist": WatchlistCopy,
		"Skill":     SkillCopy,
		"Runs":      RunsCopy,
		"System":    SystemCopy,
		"Pause":     PauseCopy,
	}
	for input, want := range cases {
		decision := Decide(input, true, true)
		if !decision.Allowed || decision.Text != want || decision.Keyboard == nil {
			t.Fatalf("Decide(%q)=%+v want allowed text=%q keyboard", input, decision, want)
		}
		if decision.Mutates || decision.TradingEnabled {
			t.Fatalf("research shell must be read-only: %+v", decision)
		}
	}
}

func TestDecideDeniesNonOwnerAndGroup(t *testing.T) {
	if got := Decide("/start", false, true); got.Allowed || got.Text != OwnerOnlyCopy {
		t.Fatalf("non-owner=%+v", got)
	}
	if got := Decide("/start", true, false); got.Allowed || got.Text != "" {
		t.Fatalf("group must be silently denied: %+v", got)
	}
}

func TestDecideUnknownCommandIsHonest(t *testing.T) {
	got := Decide("/buy SOL 50000", true, true)
	if !got.Allowed || got.Text != UnsupportedCopy || got.Mutates || got.TradingEnabled {
		t.Fatalf("unknown legacy command=%+v", got)
	}
}

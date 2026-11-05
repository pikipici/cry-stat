package contracts

import (
	"os"
	"strings"
	"testing"
)

func TestDecodeNativeFixtures(t *testing.T) {
	tests := []struct {
		name, path string
		decode     func(*os.File) error
	}{
		{"snapshot", "testdata/research_snapshot.json", func(f *os.File) error {
			v, e := DecodeResearchSnapshot(f)
			if e == nil && v.Chain == nil {
				t.Fatal("fixture chain unexpectedly null")
			}
			return e
		}},
		{"phase2", "testdata/phase2_observation.json", func(f *os.File) error { _, e := DecodePhase2Observation(f); return e }},
		{"phase3", "testdata/phase3_run.json", func(f *os.File) error { _, e := DecodePhase3Run(f); return e }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := tt.decode(f); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestResearchSnapshotPreservesNullIdentity(t *testing.T) {
	raw := `{"schema_version":"1.0.0","snapshot_id":"snap-null","mode":"LEARN_VIDEO","synthetic":true,"as_of_utc":"2026-01-01T00:00:00Z","chain":null,"mint":null,"pool":null,"evidence":[],"gates":{"g0":{"state":"UNKNOWN","evidence_ids":[],"reason":"not observed"},"g1":{"state":"UNKNOWN","evidence_ids":[],"reason":"not observed"},"g2":{"state":"UNKNOWN","evidence_ids":[],"reason":"not observed"},"g3":{"state":"UNKNOWN","evidence_ids":[],"reason":"not observed"},"g4":{"state":"UNKNOWN","evidence_ids":[],"reason":"not observed"},"g5":{"state":"UNKNOWN","evidence_ids":[],"reason":"not observed"}}}`
	v, err := DecodeResearchSnapshot(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if v.Chain != nil || v.Mint != nil || v.Pool != nil {
		t.Fatal("null identity tidak dipertahankan")
	}
}

func TestPhase3RejectsBuySignalOrExecution(t *testing.T) {
	raw, err := os.ReadFile("testdata/phase3_run.json")
	if err != nil {
		t.Fatal(err)
	}
	base := string(raw)
	for _, bad := range []string{strings.Replace(base, `"is_buy_signal": false`, `"is_buy_signal": true`, 1), strings.Replace(base, `"executes_transactions": false`, `"executes_transactions": true`, 1)} {
		if _, err := DecodePhase3Run(strings.NewReader(bad)); err == nil {
			t.Fatal("unsafe phase3 accepted")
		}
	}
}

func TestNestedSchemaAndSizeAreStrict(t *testing.T) {
	raw, err := os.ReadFile("testdata/phase3_run.json")
	if err != nil {
		t.Fatal(err)
	}
	badNested := strings.Replace(string(raw), `"research_rank":`, `"unknown_nested":true,"research_rank":`, 1)
	if _, err := DecodePhase3Run(strings.NewReader(badNested)); err == nil {
		t.Fatal("nested unknown accepted")
	}
	oversized := string(raw) + strings.Repeat(" ", maxContractBytes)
	if _, err := DecodePhase3Run(strings.NewReader(oversized)); err == nil {
		t.Fatal("oversized contract accepted")
	}
}

func TestPhase4RequiresMetricsAndSafetyConstants(t *testing.T) {
	valid := `{"schema_version":"phase4.report.v1","version":"1.1.0","mode":"demo","mint":"SYNTH","source_type":"synthetic","is_synthetic":true,"provider":"fixture","data_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","execution_model":"delayed_next_open_with_depth_proxies","executes_real_orders":false,"is_buy_signal":false,"metrics":{"closed_trades":0,"net_realized_pnl_usd":0,"marked_return_frac":0,"max_marked_drawdown_frac":0,"open_position_count":0},"open_position_unliquidated":false,"assumptions":["synthetic data","no real orders"]}`
	if _, err := DecodePhase4Report(strings.NewReader(valid)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(valid, `"is_buy_signal":false`, `"is_buy_signal":true`, 1), strings.Replace(valid, `"metrics":{"closed_trades":0,"net_realized_pnl_usd":0,"marked_return_frac":0,"max_marked_drawdown_frac":0,"open_position_count":0}`, `"metrics":{}`, 1)} {
		if _, err := DecodePhase4Report(strings.NewReader(bad)); err == nil {
			t.Fatal("unsafe/invalid phase4 accepted")
		}
	}
}

package contracts

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type ResearchSnapshot struct {
	SchemaVersion string                     `json:"schema_version"`
	SnapshotID    string                     `json:"snapshot_id"`
	Mode          string                     `json:"mode"`
	Synthetic     bool                       `json:"synthetic"`
	AsOfUTC       string                     `json:"as_of_utc"`
	Chain         *string                    `json:"chain"`
	Mint          *string                    `json:"mint"`
	Pool          *string                    `json:"pool"`
	Venue         *string                    `json:"venue,omitempty"`
	QuoteSize     *json.Number               `json:"quote_size,omitempty"`
	QuoteUnit     *string                    `json:"quote_unit,omitempty"`
	Evidence      []json.RawMessage          `json:"evidence"`
	Gates         map[string]json.RawMessage `json:"gates"`
	AnalystNotes  string                     `json:"analyst_notes,omitempty"`
}
type Phase2Observation struct {
	ObservationSchema string                     `json:"observation_schema"`
	Synthetic         bool                       `json:"synthetic"`
	Chain             string                     `json:"chain"`
	Mint              string                     `json:"mint"`
	StartedAtUTC      string                     `json:"started_at_utc"`
	CompletedAtUTC    string                     `json:"completed_at_utc"`
	Providers         map[string]json.RawMessage `json:"providers"`
	Market            json.RawMessage            `json:"market"`
	MintState         json.RawMessage            `json:"mint_state"`
	TokenAccounts     json.RawMessage            `json:"token_accounts"`
	Flags             []json.RawMessage          `json:"flags"`
	Limits            []json.RawMessage          `json:"limits"`
}
type Phase3Run struct {
	SchemaVersion        string            `json:"schema_version"`
	RunID                string            `json:"run_id"`
	CreatedAtUTC         string            `json:"created_at_utc"`
	Mode                 string            `json:"mode"`
	Synthetic            bool              `json:"synthetic"`
	Status               string            `json:"status"`
	CandidateCount       int               `json:"candidate_count"`
	ProcessedCount       int               `json:"processed_count"`
	RequestJSONCalls     int               `json:"request_json_calls"`
	ResearchPriorityOnly *bool             `json:"research_priority_only"`
	IsBuySignal          *bool             `json:"is_buy_signal"`
	ExecutesTransactions *bool             `json:"executes_transactions"`
	RankedWatchlist      []json.RawMessage `json:"ranked_watchlist"`
	Caveats              []string          `json:"caveats"`
	OutstandingCount     *int              `json:"outstanding_count,omitempty"`
	Resumable            *bool             `json:"resumable,omitempty"`
}
type Phase4Report struct {
	SchemaVersion            string                     `json:"schema_version"`
	Version                  string                     `json:"version"`
	Mode                     string                     `json:"mode"`
	Mint                     string                     `json:"mint"`
	SourceType               string                     `json:"source_type"`
	IsSynthetic              bool                       `json:"is_synthetic"`
	Provider                 string                     `json:"provider"`
	DataSHA256               string                     `json:"data_sha256"`
	PeriodStartUTC           string                     `json:"period_start_utc,omitempty"`
	PeriodEndUTC             string                     `json:"period_end_utc,omitempty"`
	ExecutionModel           string                     `json:"execution_model"`
	ExecutesRealOrders       *bool                      `json:"executes_real_orders"`
	IsBuySignal              *bool                      `json:"is_buy_signal"`
	OpenPositionUnliquidated bool                       `json:"open_position_unliquidated"`
	Metrics                  map[string]json.RawMessage `json:"metrics"`
	Assumptions              []string                   `json:"assumptions"`
}

const maxContractBytes = 1 << 20

//go:embed schemas/*.json
var nativeSchemas embed.FS
var schemaCache sync.Map

func decode(r io.Reader, schemaName string, v any) error {
	data, err := io.ReadAll(io.LimitReader(r, maxContractBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxContractBytes {
		return errors.New("contract exceeds 1 MiB")
	}
	if err := validateSchema(schemaName, data); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	d.UseNumber()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("contract must contain one JSON document")
	}
	return nil
}

func validateSchema(name string, data []byte) error {
	var schema *jsonschema.Schema
	if cached, ok := schemaCache.Load(name); ok {
		schema = cached.(*jsonschema.Schema)
	} else {
		compiler := jsonschema.NewCompiler()
		raw, err := nativeSchemas.ReadFile("schemas/" + name + ".schema.json")
		if err != nil {
			return err
		}
		var schemaDocument any
		if err := json.Unmarshal(raw, &schemaDocument); err != nil {
			return err
		}
		if err := compiler.AddResource(name+".json", schemaDocument); err != nil {
			return err
		}
		schema, err = compiler.Compile(name + ".json")
		if err != nil {
			return err
		}
		schemaCache.Store(name, schema)
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if err := schema.Validate(value); err != nil {
		return fmt.Errorf("native schema validation: %w", err)
	}
	return nil
}
func validTime(s string) bool { _, err := time.Parse(time.RFC3339, s); return err == nil }
func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}

func DecodeResearchSnapshot(r io.Reader) (ResearchSnapshot, error) {
	var v ResearchSnapshot
	if err := decode(r, "research_snapshot", &v); err != nil {
		return v, err
	}
	if v.SchemaVersion != "1.0.0" || v.SnapshotID == "" || !validTime(v.AsOfUTC) || !oneOf(v.Mode, "ANALYZE_TOKEN", "COMPARE_CANDIDATES", "AUDIT_WALLET", "DESIGN_EXPERIMENT", "LEARN_VIDEO") || v.Evidence == nil || v.Gates == nil {
		return v, errors.New("invalid research snapshot contract")
	}
	return v, nil
}
func DecodePhase2Observation(r io.Reader) (Phase2Observation, error) {
	var v Phase2Observation
	if err := decode(r, "phase2_observation", &v); err != nil {
		return v, err
	}
	if v.ObservationSchema != "phase2.observation.v1" || v.Chain != "solana" || v.Mint == "" || !validTime(v.StartedAtUTC) || !validTime(v.CompletedAtUTC) || v.Providers == nil || v.Flags == nil || v.Limits == nil {
		return v, errors.New("invalid phase2 observation contract")
	}
	return v, nil
}
func DecodePhase3Run(r io.Reader) (Phase3Run, error) {
	var v Phase3Run
	if err := decode(r, "phase3_run", &v); err != nil {
		return v, err
	}
	if v.SchemaVersion != "phase3.run.v1" || v.RunID == "" || !validTime(v.CreatedAtUTC) || !oneOf(v.Mode, "demo", "live") || !oneOf(v.Status, "RUNNING", "COMPLETED", "COMPLETED_WITH_GAPS", "PARTIAL", "PARTIAL_CALL_BUDGET", "PARTIAL_TIME_BUDGET", "PARTIAL_CANDIDATE_FAILURE", "FAILED_DISCOVERY", "NO_CANDIDATES") || v.ResearchPriorityOnly == nil || !*v.ResearchPriorityOnly || v.IsBuySignal == nil || *v.IsBuySignal || v.ExecutesTransactions == nil || *v.ExecutesTransactions || v.RankedWatchlist == nil || v.Caveats == nil || v.CandidateCount < 0 || v.ProcessedCount < 0 || v.RequestJSONCalls < 0 {
		return v, errors.New("invalid or unsafe phase3 run contract")
	}
	return v, nil
}
func DecodePhase4Report(r io.Reader) (Phase4Report, error) {
	var v Phase4Report
	if err := decode(r, "phase4_report", &v); err != nil {
		return v, err
	}
	if v.SchemaVersion != "phase4.report.v1" || v.Version != "1.1.0" || !oneOf(v.Mode, "demo", "backtest", "walkforward") || !oneOf(v.SourceType, "synthetic", "historical_external") || v.Mint == "" || v.Provider == "" || len(v.DataSHA256) != 64 || v.ExecutionModel != "delayed_next_open_with_depth_proxies" || v.ExecutesRealOrders == nil || *v.ExecutesRealOrders || v.IsBuySignal == nil || *v.IsBuySignal || v.Metrics == nil || v.Assumptions == nil {
		return v, fmt.Errorf("invalid or unsafe phase4 report contract")
	}
	return v, nil
}

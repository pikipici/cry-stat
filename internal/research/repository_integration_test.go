package research

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	tradingdb "github.com/pikipici/crypto-station/internal/tradingdb"
)

func TestResearchFoundationPersistsImmutableRunGraph(t *testing.T) {
	dsn := os.Getenv("TRADING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TRADING_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := tradingdb.New(tx)

	nowTime := time.Now().UTC()
	now := pgtype.Timestamptz{Time: nowTime, Valid: true}
	hashBytes := sha256.Sum256([]byte(t.Name() + nowTime.String()))
	hash := hex.EncodeToString(hashBytes[:])
	pkg, err := q.CreateSkillPackage(ctx, tradingdb.CreateSkillPackageParams{
		Name: "meme-token-research", Version: "1.1.0-test", ArtifactSha256: hash,
		StrategyContractVersion: "meme-research.v1", ResearchOnly: true,
		ExecutesTransactions: false, IsBuySignal: false, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := q.CreateResearchRun(ctx, tradingdb.CreateResearchRunParams{
		SkillPackageID: pkg.ID, SkillArtifactSha256: hash, ConfigSha256: hash, Status: "queued", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := q.CreateMarketSnapshot(ctx, tradingdb.CreateMarketSnapshotParams{
		ResearchRunID: run.ID, Venue: "SOLANA", AssetID: "TEST_" + hash[:12],
		ObservedAt: now, PayloadSha256: hash, Payload: []byte(`{"synthetic":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := q.CreateEvidenceItem(ctx, tradingdb.CreateEvidenceItemParams{
		ResearchRunID: run.ID, MarketSnapshotID: snapshot.ID, Kind: "identity",
		ObservedAt: now, PayloadSha256: hash, Payload: []byte(`{"status":"UNKNOWN"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	gate, err := q.CreateGateResult(ctx, tradingdb.CreateGateResultParams{
		ResearchRunID: run.ID, Gate: "G0", Status: "UNKNOWN", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	audit, err := q.CreateAuditEvent(ctx, tradingdb.CreateAuditEventParams{
		Action: "research.run.created", TargetType: "research_run", TargetID: run.ID,
		PayloadSha256: hash, Payload: []byte(`{"source":"integration-test"}`), CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	var persistedSkillHash, persistedConfigHash, persistedPayloadHash string
	if err := tx.QueryRow(ctx, `SELECT skill_artifact_sha256, config_sha256 FROM research_runs WHERE id=$1`, run.ID).Scan(&persistedSkillHash, &persistedConfigHash); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT payload_sha256 FROM market_snapshots WHERE id=$1 AND research_run_id=$2`, snapshot.ID, run.ID).Scan(&persistedPayloadHash); err != nil {
		t.Fatal(err)
	}
	if persistedSkillHash != hash || persistedConfigHash != hash || persistedPayloadHash != hash {
		t.Fatal("persisted hashes tidak sesuai")
	}
	for table, id := range map[string]int64{"skill_packages": pkg.ID, "market_snapshots": snapshot.ID, "evidence_items": evidence.ID, "gate_results": gate.ID, "audit_events": audit.ID} {
		execExpectSQLState(t, ctx, tx, "55000", `UPDATE `+pgx.Identifier{table}.Sanitize()+` SET created_at=created_at WHERE id=$1`, id)
	}
	execExpectSQLState(t, ctx, tx, "55000", `UPDATE research_runs SET config_sha256=$2 WHERE id=$1`, run.ID, strings64("f"))
	execExpectSQLState(t, ctx, tx, "23503", `INSERT INTO research_runs (skill_package_id,skill_artifact_sha256,config_sha256,status,created_at) VALUES ($1,$2,$3,'queued',$4)`, pkg.ID, strings64("e"), hash, now)
	execExpectSQLState(t, ctx, tx, "23505", `INSERT INTO skill_packages (name,version,artifact_sha256,strategy_contract_version,research_only,executes_transactions,is_buy_signal,created_at) VALUES ('duplicate','duplicate',$1,'meme-research.v1',true,false,false,$2)`, hash, now)
}

func strings64(char string) string {
	out := ""
	for len(out) < 64 {
		out += char
	}
	return out
}

func execExpectSQLState(t *testing.T, ctx context.Context, tx pgx.Tx, state, query string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(ctx, "SAVEPOINT expected_error"); err != nil {
		t.Fatal(err)
	}
	_, err := tx.Exec(ctx, query, args...)
	var pgErr *pgconn.PgError
	if err == nil || !errors.As(err, &pgErr) || pgErr.Code != state {
		t.Fatalf("error=%v, mau SQLSTATE %s", err, state)
	}
	if _, rollbackErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT expected_error"); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if _, releaseErr := tx.Exec(ctx, "RELEASE SAVEPOINT expected_error"); releaseErr != nil {
		t.Fatal(releaseErr)
	}
}

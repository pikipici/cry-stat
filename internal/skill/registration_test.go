package skill

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRegistrarRegistersValidatedBaselineInactiveWithAudit(t *testing.T) {
	dsn := os.Getenv("TRADING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TRADING_TEST_DATABASE_URL is required")
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	f, _ := zw.Create("meme-token-skill/SKILL.md")
	_, _ = f.Write([]byte("---\nname: meme-token-research\nversion: 1.1.0\n---\n" + time.Now().UTC().String()))
	_ = zw.Close()
	sum := sha256.Sum256(archive.Bytes())
	sha := hex.EncodeToString(sum[:])
	falseValue := false
	manifest := Manifest{SchemaVersion: ManifestSchemaVersion, Name: "meme-token-research", Version: "1.1.0", ArtifactSHA256: sha, StrategyContractVersion: SupportedStrategyContract, Capabilities: Capabilities{ResearchOnly: true, OfflineSimulation: &falseValue, LiveReadOnlyConnectors: &falseValue}, Prohibitions: Prohibitions{ExecutesTransactions: &falseValue, IsBuySignal: &falseValue, StoresPrivateKeys: &falseValue}}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	registrar := NewRegistrar(pool, DefaultArchivePolicy())
	result, err := registrar.Register(context.Background(), bytes.NewReader(archive.Bytes()), manifest, filepath.Join(t.TempDir(), "q"), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.Active || result.ValidationStatus != "validated" || result.ArtifactSHA256 != sha {
		t.Fatalf("result=%+v", result)
	}
	var active bool
	var status string
	var auditCount int
	if err := pool.QueryRow(context.Background(), `SELECT active, validation_status FROM skill_packages WHERE id=$1`, result.ID).Scan(&active, &status); err != nil {
		t.Fatal(err)
	}
	if active || status != "validated" {
		t.Fatalf("db active=%v status=%s", active, status)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE action='skill.package.registered' AND target_id=$1`, result.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("audit count=%d", auditCount)
	}
	if _, err := os.Stat(result.QuarantinePath); err != nil {
		t.Fatalf("committed quarantine missing: %v", err)
	}
	second, err := registrar.Register(context.Background(), bytes.NewReader(archive.Bytes()), manifest, result.QuarantinePath, time.Now().UTC())
	if err != nil || second.ID != result.ID {
		t.Fatalf("idempotent retry=(%+v,%v), want id=%d", second, err, result.ID)
	}
	var packageCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM skill_packages WHERE artifact_sha256=$1`, sha).Scan(&packageCount); err != nil {
		t.Fatal(err)
	}
	if packageCount != 1 {
		t.Fatalf("package count=%d", packageCount)
	}
}

package research

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBoundedJobClaimAndFinalize(t *testing.T) {
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
	svc := NewService(pool)
	h := sha256.Sum256([]byte(t.Name() + time.Now().String()))
	hash := hex.EncodeToString(h[:])
	var packageID int64
	err = pool.QueryRow(ctx, `INSERT INTO skill_packages(name,version,artifact_sha256,strategy_contract_version,research_only,executes_transactions,is_buy_signal,validation_status,active,quarantine_path) VALUES('job-test','1', $1,'meme-research.v1',true,false,false,'validated',false,$2) RETURNING id`, hash, "/tmp/"+hash).Scan(&packageID)
	if err != nil {
		t.Fatal(err)
	}
	job, err := svc.Queue(ctx, QueueRequest{SkillPackageID: packageID, SkillArtifactSHA256: hash, ConfigSHA256: hash, MaxCandidates: 3, MaxCalls: 60, MaxSeconds: 240})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "queued" || job.MaxCandidates != 3 {
		t.Fatalf("job=%+v", job)
	}
	var wg sync.WaitGroup
	results := make(chan Job, 2)
	errs := make(chan error, 2)
	for _, token := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(tok string) {
			defer wg.Done()
			j, e := svc.Claim(ctx, job.ID, tok, time.Minute)
			if e != nil {
				errs <- e
				return
			}
			results <- j
		}(token)
	}
	wg.Wait()
	close(results)
	close(errs)
	if len(results) != 1 || len(errs) != 1 {
		t.Fatalf("claims success=%d errors=%d", len(results), len(errs))
	}
	claimed := <-results
	if _, err := svc.Finalize(ctx, job.ID, "wrong-token", StatusCompleted, 1, 1, 10, ""); err == nil {
		t.Fatal("wrong token finalized")
	}
	if _, err := svc.Start(ctx, job.ID, claimed.ClaimToken); err != nil {
		t.Fatal(err)
	}
	final, err := svc.Finalize(ctx, job.ID, claimed.ClaimToken, StatusCompletedWithGaps, 3, 2, 60, "")
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "completed_with_gaps" || final.ProcessedCount != 2 {
		t.Fatalf("final=%+v", final)
	}
	if _, err := svc.Resume(ctx, job.ID, "worker-c", time.Minute); !errors.Is(err, ErrTerminalJob) {
		t.Fatalf("terminal resume error=%v, mau ErrTerminalJob", err)
	}
}

func TestJobBoundsAndResumeDoNotRefreshCompletedEvidence(t *testing.T) {
	if err := ValidateBounds(QueueRequest{MaxCandidates: 0, MaxCalls: 1, MaxSeconds: 1}); err == nil {
		t.Fatal("zero candidate accepted")
	}
	if err := ValidateBounds(QueueRequest{MaxCandidates: 11, MaxCalls: 1, MaxSeconds: 1}); err == nil {
		t.Fatal("candidate over cap accepted")
	}
	if !IsTerminal(StatusCompleted) || IsTerminal(StatusRunning) == true {
		t.Fatal("terminal classification wrong")
	}
}

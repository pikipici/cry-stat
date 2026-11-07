package skill

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Registrar struct {
	pool   *pgxpool.Pool
	policy ArchivePolicy
}
type RegistrationResult struct {
	ID               int64
	ArtifactSHA256   string
	ValidationStatus string
	Active           bool
	QuarantinePath   string
}

func NewRegistrar(pool *pgxpool.Pool, policy ArchivePolicy) *Registrar {
	return &Registrar{pool: pool, policy: policy}
}

func (r *Registrar) Register(ctx context.Context, artifact io.Reader, manifest Manifest, destination string, now time.Time) (RegistrationResult, error) {
	if r == nil || r.pool == nil {
		return RegistrationResult{}, errors.New("registrar database is required")
	}
	if err := manifest.Validate(); err != nil {
		return RegistrationResult{}, err
	}
	data, err := io.ReadAll(io.LimitReader(artifact, r.policy.MaxArchiveBytes+1))
	if err != nil {
		return RegistrationResult{}, err
	}
	if int64(len(data)) > r.policy.MaxArchiveBytes {
		return RegistrationResult{}, errors.New("archive exceeds size cap")
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	if sha != manifest.ArtifactSHA256 {
		return RegistrationResult{}, errors.New("artifact sha256 mismatch")
	}
	if existing, found, err := r.lookup(ctx, sha); err != nil {
		return RegistrationResult{}, err
	} else if found {
		if existing.QuarantinePath != destination {
			return RegistrationResult{}, errors.New("artifact already registered at a different quarantine path")
		}
		return existing, nil
	}
	if _, err := InspectAndQuarantine(bytes.NewReader(data), manifest, destination, r.policy); err != nil {
		return RegistrationResult{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(destination)
		}
	}()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return RegistrationResult{}, r.cleanupError(destination, err)
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO skill_packages (name,version,artifact_sha256,strategy_contract_version,research_only,executes_transactions,is_buy_signal,created_at,validation_status,active,quarantine_path) VALUES ($1,$2,$3,$4,true,false,false,$5,'validated',false,$6) RETURNING id`, manifest.Name, manifest.Version, sha, manifest.StrategyContractVersion, now, destination).Scan(&id)
	if err != nil {
		_ = tx.Rollback(ctx)
		if existing, found, lookupErr := r.lookup(ctx, sha); lookupErr == nil && found && existing.QuarantinePath == destination {
			if cleanupErr := os.RemoveAll(destination); cleanupErr != nil {
				return RegistrationResult{}, fmt.Errorf("duplicate registration cleanup failed: %w", cleanupErr)
			}
			cleanup = false
			return existing, nil
		}
		return RegistrationResult{}, r.cleanupError(destination, fmt.Errorf("insert skill package: %w", err))
	}
	payload, _ := json.Marshal(map[string]any{"artifact_sha256": sha, "validation_status": "validated", "active": false})
	payloadHash := sha256.Sum256(payload)
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (action,target_type,target_id,payload_sha256,payload,created_at) VALUES ('skill.package.registered','skill_package',$1,$2,$3,$4)`, id, hex.EncodeToString(payloadHash[:]), payload, now); err != nil {
		_ = tx.Rollback(ctx)
		return RegistrationResult{}, r.cleanupError(destination, fmt.Errorf("insert registration audit: %w", err))
	}
	if err = tx.Commit(ctx); err != nil {
		// Commit errors can have an ambiguous outcome. Reconcile by immutable SHA
		// before deleting quarantine; a committed row must retain its artifact.
		if existing, found, lookupErr := r.lookup(ctx, sha); lookupErr == nil && found {
			if existing.QuarantinePath != destination {
				return RegistrationResult{}, errors.New("commit outcome conflicts with registered quarantine path")
			}
			cleanup = false
			return existing, nil
		} else if lookupErr != nil {
			cleanup = false
			return RegistrationResult{}, fmt.Errorf("commit outcome unknown; quarantine retained for reconciliation: commit=%v lookup=%w", err, lookupErr)
		}
		return RegistrationResult{}, r.cleanupError(destination, err)
	}
	cleanup = false
	return RegistrationResult{ID: id, ArtifactSHA256: sha, ValidationStatus: "validated", Active: false, QuarantinePath: destination}, nil
}

func (r *Registrar) lookup(ctx context.Context, sha string) (RegistrationResult, bool, error) {
	var result RegistrationResult
	err := r.pool.QueryRow(ctx, `SELECT id,artifact_sha256,validation_status,active,quarantine_path FROM skill_packages WHERE artifact_sha256=$1`, sha).Scan(&result.ID, &result.ArtifactSHA256, &result.ValidationStatus, &result.Active, &result.QuarantinePath)
	if errors.Is(err, pgx.ErrNoRows) {
		return RegistrationResult{}, false, nil
	}
	return result, err == nil, err
}

func (r *Registrar) cleanupError(destination string, cause error) error {
	if err := os.RemoveAll(destination); err != nil {
		return fmt.Errorf("%v; quarantine cleanup failed: %w", cause, err)
	}
	return cause
}

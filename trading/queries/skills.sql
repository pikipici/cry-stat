-- name: CreateSkillPackage :one
INSERT INTO skill_packages (name, version, artifact_sha256, strategy_contract_version, research_only, executes_transactions, is_buy_signal, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING *;

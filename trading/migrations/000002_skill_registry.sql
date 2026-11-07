-- +goose Up
ALTER TABLE skill_packages ADD COLUMN validation_status TEXT NOT NULL DEFAULT 'pending' CHECK (validation_status IN ('pending','validated','rejected'));
ALTER TABLE skill_packages ADD COLUMN active BOOLEAN NOT NULL DEFAULT false CHECK (NOT active);
ALTER TABLE skill_packages ADD COLUMN quarantine_path TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX idx_skill_packages_quarantine_path_unique ON skill_packages(quarantine_path) WHERE quarantine_path <> '';

-- +goose Down
DROP INDEX IF EXISTS idx_skill_packages_quarantine_path_unique;
ALTER TABLE skill_packages DROP COLUMN IF EXISTS quarantine_path;
ALTER TABLE skill_packages DROP COLUMN IF EXISTS active;
ALTER TABLE skill_packages DROP COLUMN IF EXISTS validation_status;

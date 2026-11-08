-- +goose Up
-- Crypto Station Fase 9: credited_idr di deposits (IDR yang dibayar ke user).

ALTER TABLE deposits ADD COLUMN IF NOT EXISTS credited_idr BIGINT NOT NULL DEFAULT 0;

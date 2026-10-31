-- +goose Up
-- Crypto Station Fase 9: journals.type + payout/deposit/float-in.

ALTER TABLE journals DROP CONSTRAINT IF EXISTS journals_type_check;
ALTER TABLE journals ADD CONSTRAINT journals_type_check
  CHECK (type IN ('grant','topup','buy','sell','payout','float-in','deposit','withdraw','refund','adjust'));

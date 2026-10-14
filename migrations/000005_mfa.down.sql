-- Reverts 000005_mfa. MFA enrolments and recovery codes are lost.
ALTER TABLE sessions DROP COLUMN IF EXISTS mfa_verified_at;
DROP TABLE IF EXISTS mfa_recovery_codes;
DROP TABLE IF EXISTS user_mfa;

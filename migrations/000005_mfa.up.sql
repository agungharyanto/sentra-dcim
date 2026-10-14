-- 000005_mfa: TOTP MFA for every account (v0.3.3, D-062..D-064).
-- Backward compatible: new tables and a nullable column; v0.3.2 binaries ignore them.
CREATE TABLE user_mfa (
    user_id        uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    -- AES-256-GCM(nonce || ciphertext) of the TOTP secret, AAD = user id; key from MFA_ENC_KEY. Never the secret.
    secret_enc     bytea       NOT NULL CHECK (octet_length(secret_enc) BETWEEN 28 AND 128),
    -- NULL while the enrolment waits for its first valid code.
    enabled_at     timestamptz NULL,
    -- Highest TOTP time step accepted so far: codes of this or earlier steps are refused (anti-replay).
    last_used_step bigint      NOT NULL DEFAULT 0 CHECK (last_used_step >= 0),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE user_mfa IS 'TOTP MFA per account (v0.3.3). Secrets only encrypted (MFA_ENC_KEY).';

CREATE TABLE mfa_recovery_codes (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- HMAC-SHA256 of the normalised code (key derived from MFA_ENC_KEY). Never the code.
    code_hash  bytea       NOT NULL CHECK (octet_length(code_hash) = 32),
    used_at    timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, code_hash)
);
COMMENT ON TABLE mfa_recovery_codes IS 'One-time MFA recovery codes (v0.3.3), stored as keyed hashes.';

-- A session is usable only after its MFA step (NULL = mfa_pending). Existing sessions become pending: every
-- signed-in user passes (or enrols) MFA on the next request.
ALTER TABLE sessions ADD COLUMN mfa_verified_at timestamptz NULL;

-- 000007_audit_chain: tamper-evident audit log (v0.3.4, D-051/D-073). Every row carries
--   hash = sha256(prev_hash || canonical(row)), prev_hash = hash of the previous row (32 zero bytes for the first,
-- or the last pruned row's hash from audit_checkpoints). A BEFORE INSERT trigger assigns chain_seq and both hashes
-- under an advisory transaction lock, so concurrent inserts can never fork the chain and clients cannot supply
-- their own values. internal/audit/chain re-computes the same canonical form independently from the raw columns.

ALTER TABLE audit_events ADD COLUMN chain_seq bigint;

-- canonical(row): every field as <octet length>:<value>, in a fixed order; NULL → empty. Timestamps in UTC with
-- microseconds, IPs without prefix length, metadata as jsonb text (deterministic).
CREATE FUNCTION audit_field(v text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT octet_length(coalesce(v, ''))::text || ':' || coalesce(v, '')
$$;

CREATE FUNCTION audit_canonical(e audit_events) RETURNS bytea LANGUAGE sql STABLE AS $$
    SELECT convert_to(
        audit_field(e.chain_seq::text) || audit_field(e.id::text) ||
        audit_field(to_char(e.occurred_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')) ||
        audit_field(e.actor_type) || audit_field(e.actor_id::text) || audit_field(e.actor_username) ||
        audit_field(e.action) || audit_field(e.target_type) || audit_field(e.target_id) || audit_field(e.result) ||
        audit_field(host(e.ip)) || audit_field(e.user_agent) || audit_field(e.request_id) || audit_field(e.metadata::text),
        'UTF8')
$$;

-- Checkpoints left by pruning (retention): the chain continues from last_hash after through_seq.
CREATE TABLE audit_checkpoints (
    through_seq    bigint      PRIMARY KEY,
    last_hash      bytea       NOT NULL CHECK (octet_length(last_hash) = 32),
    rows_pruned    bigint      NOT NULL CHECK (rows_pruned > 0),
    oldest_at      timestamptz NOT NULL,
    newest_at      timestamptz NOT NULL,
    archive_name   text        NOT NULL CHECK (length(archive_name) BETWEEN 1 AND 200),
    archive_sha256 text        NOT NULL CHECK (archive_sha256 ~ '^[0-9a-f]{64}$'),
    created_at     timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE audit_checkpoints IS 'Hash of the last pruned audit row per pruning run (v0.3.4); append-only.';
CREATE TRIGGER audit_checkpoints_no_update_delete BEFORE UPDATE OR DELETE ON audit_checkpoints
    FOR EACH ROW EXECUTE FUNCTION audit_events_append_only();
GRANT SELECT ON audit_checkpoints TO sentra_app;

-- Backfill the existing rows in id order (owner only; the append-only trigger is paused for this statement).
ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_update_delete;
DO $$
DECLARE
    r    audit_events;
    prev bytea := decode(repeat('00', 32), 'hex');
    seq  bigint := 0;
BEGIN
    FOR r IN SELECT * FROM audit_events ORDER BY id LOOP
        seq := seq + 1;
        r.chain_seq := seq;
        r.prev_hash := prev;
        r.hash := sha256(prev || audit_canonical(r));
        UPDATE audit_events SET chain_seq = r.chain_seq, prev_hash = r.prev_hash, hash = r.hash WHERE id = r.id;
        prev := r.hash;
    END LOOP;
END
$$;
ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_update_delete;

ALTER TABLE audit_events ALTER COLUMN chain_seq SET NOT NULL,
                         ALTER COLUMN prev_hash SET NOT NULL,
                         ALTER COLUMN hash SET NOT NULL,
                         ADD CONSTRAINT audit_events_chain_seq_key UNIQUE (chain_seq),
                         ADD CONSTRAINT audit_events_hash_len CHECK (octet_length(hash) = 32 AND octet_length(prev_hash) = 32);

CREATE FUNCTION audit_events_chain() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    last_seq  bigint;
    last_hash bytea;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtext('sentra.audit_chain'));
    SELECT chain_seq, hash INTO last_seq, last_hash FROM audit_events ORDER BY chain_seq DESC LIMIT 1;
    IF last_seq IS NULL THEN
        SELECT through_seq, c.last_hash INTO last_seq, last_hash FROM audit_checkpoints c ORDER BY through_seq DESC LIMIT 1;
    END IF;
    NEW.chain_seq := coalesce(last_seq, 0) + 1;
    NEW.prev_hash := coalesce(last_hash, decode(repeat('00', 32), 'hex'));
    NEW.hash := sha256(NEW.prev_hash || audit_canonical(NEW));
    RETURN NEW;
END
$$;
CREATE TRIGGER audit_events_chain BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION audit_events_chain();

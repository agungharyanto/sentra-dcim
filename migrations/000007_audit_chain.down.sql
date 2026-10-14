-- Reverts 000007_audit_chain: removes the chain trigger, checkpoints and chain_seq; hashes are cleared (the columns
-- existed before, nullable). Pruned rows are NOT restored (their archives stay in the backup directory).
DROP TRIGGER IF EXISTS audit_events_chain ON audit_events;
DROP FUNCTION IF EXISTS audit_events_chain();
ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS audit_events_hash_len,
                         DROP CONSTRAINT IF EXISTS audit_events_chain_seq_key,
                         ALTER COLUMN prev_hash DROP NOT NULL,
                         ALTER COLUMN hash DROP NOT NULL;
ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_update_delete;
UPDATE audit_events SET prev_hash = NULL, hash = NULL;
ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_update_delete;
ALTER TABLE audit_events DROP COLUMN IF EXISTS chain_seq;
DROP TABLE IF EXISTS audit_checkpoints;
DROP FUNCTION IF EXISTS audit_canonical(audit_events);
DROP FUNCTION IF EXISTS audit_field(text);

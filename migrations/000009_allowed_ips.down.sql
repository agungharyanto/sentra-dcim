-- Reverts 000009_allowed_ips. The mode keys in `settings` are removed too, so a re-up starts in mode off.
DELETE FROM settings WHERE key IN ('allowlist.mode', 'allowlist.pending_until');
DROP TABLE IF EXISTS allowed_ips;

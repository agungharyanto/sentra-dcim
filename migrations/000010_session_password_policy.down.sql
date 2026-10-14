-- Reverts 000010: kiosk flags and password history are dropped; the policy settings rows are removed (defaults apply).
DELETE FROM settings WHERE key LIKE 'session.%' OR key LIKE 'password.%';
DROP TABLE IF EXISTS password_history;
ALTER TABLE sessions DROP COLUMN IF EXISTS kiosk;
ALTER TABLE users DROP COLUMN IF EXISTS kiosk;

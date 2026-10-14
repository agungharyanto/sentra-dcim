-- Reverts 000008_settings: settings fall back to their defaults (.env), user preferences are dropped.
DROP TABLE IF EXISTS user_preferences;
DROP TABLE IF EXISTS settings;

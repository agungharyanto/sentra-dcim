-- 000001_init (down). The timescaledb extension is intentionally kept: it is provided by the image
-- and dropping it would affect objects owned by later migrations.
DROP TABLE IF EXISTS system_info;

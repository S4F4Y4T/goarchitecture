-- Change id from SERIAL (int4) to BIGSERIAL (int8) for large-scale growth.
-- Change timestamp columns from TIMESTAMP (no timezone) to TIMESTAMPTZ
-- to match the user service schema and avoid timezone-related bugs.
ALTER TABLE products
    ALTER COLUMN id TYPE BIGINT,
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

-- Re-create the sequence as bigint-backed.
ALTER SEQUENCE products_id_seq AS BIGINT;

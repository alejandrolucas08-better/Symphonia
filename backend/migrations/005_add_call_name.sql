ALTER TABLE calls ADD COLUMN IF NOT EXISTS name VARCHAR(80);

UPDATE calls
SET name = 'Meeting'
WHERE name IS NULL OR BTRIM(name) = '';

ALTER TABLE calls ALTER COLUMN name SET NOT NULL;
ALTER TABLE calls ALTER COLUMN name SET DEFAULT 'Meeting';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'calls_name_length_check'
          AND conrelid = 'calls'::regclass
    ) THEN
        ALTER TABLE calls
            ADD CONSTRAINT calls_name_length_check
            CHECK (CHAR_LENGTH(BTRIM(name)) BETWEEN 1 AND 80);
    END IF;
END $$;

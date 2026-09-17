-- A legacy Symphonia release used UUID identifiers and a different calls
-- schema. The Go backend uses BIGINT identifiers, so values from that schema
-- cannot be scanned into its int64 fields. Rebuild the related tables only
-- when that legacy UUID layout is detected, preserving compatible data.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'id'
          AND udt_name = 'uuid'
    ) THEN
        RETURN;
    END IF;

    -- Do not silently alter historical records whose values cannot be
    -- represented by the current API contract.
    IF EXISTS (SELECT 1 FROM users WHERE char_length(name) > 60) THEN
        RAISE EXCEPTION 'cannot migrate legacy users: a name exceeds 60 characters';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM calls
        WHERE code !~ '^[a-z0-9]{8}$'
           OR status NOT IN ('waiting', 'active', 'ended')
           OR (status = 'ended' AND ended_at IS NULL)
           OR (status <> 'ended' AND ended_at IS NOT NULL)
    ) THEN
        RAISE EXCEPTION 'cannot migrate legacy calls: unsupported code or status';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM call_participants
        WHERE language NOT IN ('PT-BR', 'EN-US', 'ES-ES', 'FR-FR')
    ) THEN
        RAISE EXCEPTION 'cannot migrate legacy call participants: unsupported language';
    END IF;

    ALTER TABLE call_participants RENAME TO call_participants_legacy;
    ALTER TABLE calls RENAME TO calls_legacy;
    ALTER TABLE users RENAME TO users_legacy;

    CREATE TEMPORARY TABLE user_id_map (
        legacy_id UUID PRIMARY KEY,
        new_id BIGINT NOT NULL
    ) ON COMMIT DROP;

    INSERT INTO user_id_map (legacy_id, new_id)
    SELECT id, row_number() OVER (ORDER BY created_at, id)::BIGINT
    FROM users_legacy;

    CREATE TABLE users (
        id BIGSERIAL CONSTRAINT symphonia_users_pkey PRIMARY KEY,
        name VARCHAR(60) NOT NULL,
        email VARCHAR(254) NOT NULL CONSTRAINT symphonia_users_email_key UNIQUE,
        password_hash TEXT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    INSERT INTO users (id, name, email, password_hash, created_at, updated_at)
    SELECT map.new_id, legacy.name, LOWER(legacy.email), legacy.password_hash,
           legacy.created_at, legacy.updated_at
    FROM users_legacy AS legacy
    JOIN user_id_map AS map ON map.legacy_id = legacy.id
    ORDER BY map.new_id;

    PERFORM setval(
        'users_id_seq'::regclass,
        COALESCE((SELECT MAX(id) FROM users), 1),
        EXISTS (SELECT 1 FROM users)
    );

    CREATE TEMPORARY TABLE call_id_map (
        legacy_id UUID PRIMARY KEY,
        new_id BIGINT NOT NULL
    ) ON COMMIT DROP;

    INSERT INTO call_id_map (legacy_id, new_id)
    SELECT id, row_number() OVER (ORDER BY created_at, id)::BIGINT
    FROM calls_legacy;

    CREATE TABLE calls (
        id BIGSERIAL CONSTRAINT symphonia_calls_pkey PRIMARY KEY,
        code VARCHAR(8) NOT NULL CONSTRAINT symphonia_calls_code_key UNIQUE
            CHECK (code ~ '^[a-z0-9]{8}$'),
        host_user_id BIGINT NOT NULL REFERENCES users(id),
        status VARCHAR(7) NOT NULL DEFAULT 'waiting'
            CHECK (status IN ('waiting', 'active', 'ended')),
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        ended_at TIMESTAMPTZ,
        CHECK ((status = 'ended' AND ended_at IS NOT NULL) OR
               (status <> 'ended' AND ended_at IS NULL))
    );

    INSERT INTO calls (id, code, host_user_id, status, created_at, updated_at, ended_at)
    SELECT call_map.new_id, legacy.code, user_map.new_id, legacy.status,
           legacy.created_at, COALESCE(legacy.ended_at, legacy.created_at),
           legacy.ended_at
    FROM calls_legacy AS legacy
    JOIN call_id_map AS call_map ON call_map.legacy_id = legacy.id
    JOIN user_id_map AS user_map ON user_map.legacy_id = legacy.created_by
    ORDER BY call_map.new_id;

    PERFORM setval(
        'calls_id_seq'::regclass,
        COALESCE((SELECT MAX(id) FROM calls), 1),
        EXISTS (SELECT 1 FROM calls)
    );

    CREATE TABLE call_participants (
        call_id BIGINT NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
        user_id BIGINT NOT NULL REFERENCES users(id),
        spoken_language VARCHAR(5) NOT NULL
            CHECK (spoken_language IN ('PT-BR', 'EN-US', 'ES-ES', 'FR-FR')),
        heard_language VARCHAR(5) NOT NULL
            CHECK (heard_language IN ('PT-BR', 'EN-US', 'ES-ES', 'FR-FR')),
        joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        left_at TIMESTAMPTZ,
        CONSTRAINT symphonia_call_participants_pkey PRIMARY KEY (call_id, user_id)
    );

    INSERT INTO call_participants (
        call_id, user_id, spoken_language, heard_language, joined_at, left_at
    )
    SELECT call_map.new_id, user_map.new_id, legacy.language, legacy.language,
           legacy.joined_at, legacy.left_at
    FROM call_participants_legacy AS legacy
    JOIN call_id_map AS call_map ON call_map.legacy_id = legacy.call_id
    JOIN user_id_map AS user_map ON user_map.legacy_id = legacy.user_id;

    CREATE INDEX idx_symphonia_users_email ON users (email);
    CREATE INDEX idx_symphonia_calls_code ON calls (code);
    CREATE INDEX idx_symphonia_call_participants_active
        ON call_participants (call_id) WHERE left_at IS NULL;
    CREATE INDEX idx_symphonia_call_participants_user
        ON call_participants (user_id);

    DROP TABLE call_participants_legacy;
    DROP TABLE calls_legacy;
    DROP TABLE users_legacy;
END $$;

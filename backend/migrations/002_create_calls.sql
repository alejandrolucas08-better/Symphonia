CREATE TABLE IF NOT EXISTS calls (
    id           BIGSERIAL PRIMARY KEY,
    code         VARCHAR(8) NOT NULL UNIQUE CHECK (code ~ '^[a-z0-9]{8}$'),
    host_user_id BIGINT NOT NULL REFERENCES users(id),
    status       VARCHAR(7) NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'active', 'ended')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at     TIMESTAMPTZ,
    CHECK ((status = 'ended' AND ended_at IS NOT NULL) OR (status <> 'ended' AND ended_at IS NULL))
);

CREATE TABLE IF NOT EXISTS call_participants (
    call_id         BIGINT NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    spoken_language VARCHAR(5) NOT NULL CHECK (spoken_language IN ('PT-BR', 'EN-US', 'ES-ES', 'FR-FR')),
    heard_language  VARCHAR(5) NOT NULL CHECK (heard_language IN ('PT-BR', 'EN-US', 'ES-ES', 'FR-FR')),
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at         TIMESTAMPTZ,
    PRIMARY KEY (call_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_calls_code ON calls (code);
CREATE INDEX IF NOT EXISTS idx_call_participants_active
    ON call_participants (call_id) WHERE left_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_call_participants_user
    ON call_participants (user_id);

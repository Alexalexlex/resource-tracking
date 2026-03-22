package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS llm_events (
    id BIGSERIAL PRIMARY KEY,
    team TEXT NOT NULL,
    service TEXT NOT NULL,
    environment TEXT NOT NULL DEFAULT 'unknown',
    endpoint TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    request_type TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    prompt_tokens INTEGER NOT NULL DEFAULT 0 CHECK (prompt_tokens >= 0),
    completion_tokens INTEGER NOT NULL DEFAULT 0 CHECK (completion_tokens >= 0),
    total_tokens INTEGER NOT NULL DEFAULT 0 CHECK (total_tokens >= 0),
    estimated_cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (estimated_cost_usd >= 0),
    latency_ms INTEGER NOT NULL DEFAULT 0 CHECK (latency_ms >= 0),
    status_code INTEGER NOT NULL DEFAULT 200,
    success BOOLEAN NOT NULL DEFAULT TRUE,
    request_summary TEXT NOT NULL DEFAULT '',
    request_meta JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_meta JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_llm_events_created_at ON llm_events (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_llm_events_team_created_at ON llm_events (team, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_llm_events_service_created_at ON llm_events (service, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_llm_events_model_created_at ON llm_events (model, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_llm_events_team_service_created_at ON llm_events (team, service, created_at DESC);

CREATE TABLE IF NOT EXISTS llm_quotas (
    id BIGSERIAL PRIMARY KEY,
    team TEXT NOT NULL,
    service TEXT NOT NULL DEFAULT '',
    daily_request_limit BIGINT NOT NULL DEFAULT 0 CHECK (daily_request_limit >= 0),
    daily_token_limit BIGINT NOT NULL DEFAULT 0 CHECK (daily_token_limit >= 0),
    daily_cost_limit_usd DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (daily_cost_limit_usd >= 0),
    hard_limit BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(team, service)
);
`

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schemaSQL)
	return err
}

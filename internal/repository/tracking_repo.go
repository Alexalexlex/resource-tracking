package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"example.com/fiber-hello/internal/entity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TrackingRepository interface {
	CreateEvent(ctx context.Context, event entity.LLMEvent) (entity.LLMEvent, error)
	ListEvents(ctx context.Context, filter entity.EventFilter) ([]entity.LLMEvent, error)
	GetOverview(ctx context.Context, filter entity.AnalyticsFilter) (entity.OverviewStats, error)
	GetTeamBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error)
	GetServiceBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error)
	GetModelBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error)
	GetHourlyTimeseries(ctx context.Context, filter entity.AnalyticsFilter, hours int) ([]entity.TimeseriesPoint, error)
	ListApplicableQuotas(ctx context.Context, team, service string) ([]entity.Quota, error)
	GetQuotaUsage(ctx context.Context, quota entity.Quota, from, to time.Time) (entity.QuotaUsage, error)
	UpsertQuota(ctx context.Context, quota entity.Quota) (entity.Quota, error)
	ListQuotas(ctx context.Context, team, service string) ([]entity.Quota, error)
}

type trackingRepo struct {
	pool *pgxpool.Pool
}

func NewTrackingRepository(pool *pgxpool.Pool) TrackingRepository {
	return &trackingRepo{pool: pool}
}

func (r *trackingRepo) CreateEvent(ctx context.Context, event entity.LLMEvent) (entity.LLMEvent, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	event.RequestMeta = normalizeJSON(event.RequestMeta)
	event.ResponseMeta = normalizeJSON(event.ResponseMeta)

	query := `
		INSERT INTO llm_events (
			team, service, environment, endpoint, provider, model, request_type,
			user_id, trace_id, prompt_tokens, completion_tokens, total_tokens,
			estimated_cost_usd, latency_ms, status_code, success,
			request_summary, request_meta, response_meta, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16,
			$17, $18, $19, $20
		)
		RETURNING id, created_at
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		event.Team,
		event.Service,
		event.Environment,
		event.Endpoint,
		event.Provider,
		event.Model,
		event.RequestType,
		event.UserID,
		event.TraceID,
		event.PromptTokens,
		event.CompletionTokens,
		event.TotalTokens,
		event.EstimatedCostUSD,
		event.LatencyMs,
		event.StatusCode,
		event.Success,
		event.RequestSummary,
		event.RequestMeta,
		event.ResponseMeta,
		event.CreatedAt,
	).Scan(&event.ID, &event.CreatedAt)
	if err != nil {
		return entity.LLMEvent{}, err
	}

	return event, nil
}

func (r *trackingRepo) ListEvents(ctx context.Context, filter entity.EventFilter) ([]entity.LLMEvent, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	whereClause, args, nextArg := buildEventWhere(filter.AnalyticsFilter, 1)
	query := fmt.Sprintf(`
		SELECT
			id, team, service, environment, endpoint, provider, model, request_type,
			user_id, trace_id, prompt_tokens, completion_tokens, total_tokens,
			estimated_cost_usd, latency_ms, status_code, success,
			request_summary, request_meta, response_meta, created_at
		FROM llm_events
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, nextArg, nextArg+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]entity.LLMEvent, 0, filter.Limit)
	for rows.Next() {
		event, scanErr := scanEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

func (r *trackingRepo) GetOverview(ctx context.Context, filter entity.AnalyticsFilter) (entity.OverviewStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	whereClause, args, _ := buildEventWhere(filter, 1)
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) AS total_requests,
			COUNT(*) FILTER (WHERE success) AS successful_requests,
			COUNT(*) FILTER (WHERE NOT success) AS failed_requests,
			COUNT(DISTINCT team) AS teams,
			COUNT(DISTINCT CONCAT(team, ':', service)) AS services,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(estimated_cost_usd), 0) AS total_cost_usd,
			COALESCE(AVG(latency_ms), 0) AS average_latency_ms,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms), 0) AS p95_latency_ms
		FROM llm_events
		%s
	`, whereClause)

	var overview entity.OverviewStats
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&overview.TotalRequests,
		&overview.SuccessfulRequests,
		&overview.FailedRequests,
		&overview.Teams,
		&overview.Services,
		&overview.TotalTokens,
		&overview.TotalCostUSD,
		&overview.AverageLatencyMs,
		&overview.P95LatencyMs,
	)
	if err != nil {
		return entity.OverviewStats{}, err
	}

	return overview, nil
}

func (r *trackingRepo) GetTeamBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error) {
	return r.getBreakdown(ctx, filter, limit, `
		SELECT
			team,
			'' AS service,
			'' AS provider,
			'' AS model,
			COUNT(*) AS requests,
			COUNT(*) FILTER (WHERE NOT success) AS failed_requests,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(estimated_cost_usd), 0) AS total_cost_usd,
			COALESCE(AVG(latency_ms), 0) AS average_latency_ms
		FROM llm_events
		%s
		GROUP BY team
		ORDER BY requests DESC, team ASC
		LIMIT $%d
	`)
}

func (r *trackingRepo) GetServiceBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error) {
	return r.getBreakdown(ctx, filter, limit, `
		SELECT
			team,
			service,
			'' AS provider,
			'' AS model,
			COUNT(*) AS requests,
			COUNT(*) FILTER (WHERE NOT success) AS failed_requests,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(estimated_cost_usd), 0) AS total_cost_usd,
			COALESCE(AVG(latency_ms), 0) AS average_latency_ms
		FROM llm_events
		%s
		GROUP BY team, service
		ORDER BY requests DESC, team ASC, service ASC
		LIMIT $%d
	`)
}

func (r *trackingRepo) GetModelBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error) {
	return r.getBreakdown(ctx, filter, limit, `
		SELECT
			'' AS team,
			'' AS service,
			provider,
			model,
			COUNT(*) AS requests,
			COUNT(*) FILTER (WHERE NOT success) AS failed_requests,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(estimated_cost_usd), 0) AS total_cost_usd,
			COALESCE(AVG(latency_ms), 0) AS average_latency_ms
		FROM llm_events
		%s
		GROUP BY provider, model
		ORDER BY requests DESC, provider ASC, model ASC
		LIMIT $%d
	`)
}

func (r *trackingRepo) GetHourlyTimeseries(ctx context.Context, filter entity.AnalyticsFilter, hours int) ([]entity.TimeseriesPoint, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if hours < 1 {
		hours = 24
	}

	to := time.Now().UTC()
	if filter.To != nil {
		to = filter.To.UTC()
	}

	from := to.Add(-time.Duration(hours) * time.Hour)
	if filter.From != nil {
		from = filter.From.UTC()
	}

	filter.From = &from
	filter.To = &to

	whereClause, args, _ := buildEventWhere(filter, 1)
	query := fmt.Sprintf(`
		SELECT
			DATE_TRUNC('hour', created_at) AS bucket,
			COUNT(*) AS requests,
			COUNT(*) FILTER (WHERE NOT success) AS failed_requests,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(estimated_cost_usd), 0) AS total_cost_usd,
			COALESCE(AVG(latency_ms), 0) AS average_latency_ms
		FROM llm_events
		%s
		GROUP BY bucket
		ORDER BY bucket ASC
	`, whereClause)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]entity.TimeseriesPoint, 0, hours)
	for rows.Next() {
		var point entity.TimeseriesPoint
		if err := rows.Scan(
			&point.Bucket,
			&point.Requests,
			&point.FailedRequests,
			&point.TotalTokens,
			&point.TotalCostUSD,
			&point.AverageLatencyMs,
		); err != nil {
			return nil, err
		}
		points = append(points, point)
	}

	return points, rows.Err()
}

func (r *trackingRepo) ListApplicableQuotas(ctx context.Context, team, service string) ([]entity.Quota, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT
			id, team, service, daily_request_limit, daily_token_limit,
			daily_cost_limit_usd, hard_limit, enabled, created_at, updated_at
		FROM llm_quotas
		WHERE enabled = TRUE AND team = $1 AND (service = '' OR service = $2)
		ORDER BY service ASC
	`, team, service)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	quotas := make([]entity.Quota, 0)
	for rows.Next() {
		quota, scanErr := scanQuota(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		quotas = append(quotas, quota)
	}

	return quotas, rows.Err()
}

func (r *trackingRepo) GetQuotaUsage(ctx context.Context, quota entity.Quota, from, to time.Time) (entity.QuotaUsage, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var query string
	var args []any
	if quota.Service == "" {
		query = `
			SELECT
				COUNT(*) AS requests,
				COALESCE(SUM(total_tokens), 0) AS total_tokens,
				COALESCE(SUM(estimated_cost_usd), 0) AS total_cost_usd
			FROM llm_events
			WHERE team = $1 AND created_at >= $2 AND created_at < $3
		`
		args = []any{quota.Team, from, to}
	} else {
		query = `
			SELECT
				COUNT(*) AS requests,
				COALESCE(SUM(total_tokens), 0) AS total_tokens,
				COALESCE(SUM(estimated_cost_usd), 0) AS total_cost_usd
			FROM llm_events
			WHERE team = $1 AND service = $2 AND created_at >= $3 AND created_at < $4
		`
		args = []any{quota.Team, quota.Service, from, to}
	}

	var usage entity.QuotaUsage
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&usage.Requests,
		&usage.TotalTokens,
		&usage.TotalCostUSD,
	)
	if err != nil {
		return entity.QuotaUsage{}, err
	}

	return usage, nil
}

func (r *trackingRepo) UpsertQuota(ctx context.Context, quota entity.Quota) (entity.Quota, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO llm_quotas (
			team, service, daily_request_limit, daily_token_limit,
			daily_cost_limit_usd, hard_limit, enabled
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (team, service)
		DO UPDATE SET
			daily_request_limit = EXCLUDED.daily_request_limit,
			daily_token_limit = EXCLUDED.daily_token_limit,
			daily_cost_limit_usd = EXCLUDED.daily_cost_limit_usd,
			hard_limit = EXCLUDED.hard_limit,
			enabled = EXCLUDED.enabled,
			updated_at = NOW()
		RETURNING
			id, team, service, daily_request_limit, daily_token_limit,
			daily_cost_limit_usd, hard_limit, enabled, created_at, updated_at
	`

	var stored entity.Quota
	err := r.pool.QueryRow(
		ctx,
		query,
		quota.Team,
		quota.Service,
		quota.DailyRequestLimit,
		quota.DailyTokenLimit,
		quota.DailyCostLimitUSD,
		quota.HardLimit,
		quota.Enabled,
	).Scan(
		&stored.ID,
		&stored.Team,
		&stored.Service,
		&stored.DailyRequestLimit,
		&stored.DailyTokenLimit,
		&stored.DailyCostLimitUSD,
		&stored.HardLimit,
		&stored.Enabled,
		&stored.CreatedAt,
		&stored.UpdatedAt,
	)
	if err != nil {
		return entity.Quota{}, err
	}

	return stored, nil
}

func (r *trackingRepo) ListQuotas(ctx context.Context, team, service string) ([]entity.Quota, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	clauses := []string{"1 = 1"}
	args := make([]any, 0, 2)
	argIndex := 1
	if strings.TrimSpace(team) != "" {
		clauses = append(clauses, fmt.Sprintf("team = $%d", argIndex))
		args = append(args, team)
		argIndex++
	}
	if strings.TrimSpace(service) != "" {
		clauses = append(clauses, fmt.Sprintf("service = $%d", argIndex))
		args = append(args, service)
		argIndex++
	}

	query := fmt.Sprintf(`
		SELECT
			id, team, service, daily_request_limit, daily_token_limit,
			daily_cost_limit_usd, hard_limit, enabled, created_at, updated_at
		FROM llm_quotas
		WHERE %s
		ORDER BY team ASC, service ASC
	`, strings.Join(clauses, " AND "))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	quotas := make([]entity.Quota, 0)
	for rows.Next() {
		quota, scanErr := scanQuota(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		quotas = append(quotas, quota)
	}

	return quotas, rows.Err()
}

func (r *trackingRepo) getBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int, queryTemplate string) ([]entity.BreakdownRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	whereClause, args, nextArg := buildEventWhere(filter, 1)
	query := fmt.Sprintf(queryTemplate, whereClause, nextArg)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	breakdown := make([]entity.BreakdownRow, 0, limit)
	for rows.Next() {
		var row entity.BreakdownRow
		if err := rows.Scan(
			&row.Team,
			&row.Service,
			&row.Provider,
			&row.Model,
			&row.Requests,
			&row.FailedRequests,
			&row.TotalTokens,
			&row.TotalCostUSD,
			&row.AverageLatencyMs,
		); err != nil {
			return nil, err
		}
		breakdown = append(breakdown, row)
	}

	return breakdown, rows.Err()
}

func buildEventWhere(filter entity.AnalyticsFilter, startArg int) (string, []any, int) {
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 6)
	argIndex := startArg

	addTextFilter := func(column, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		clauses = append(clauses, fmt.Sprintf("%s = $%d", column, argIndex))
		args = append(args, value)
		argIndex++
	}

	addTextFilter("team", filter.Team)
	addTextFilter("service", filter.Service)
	addTextFilter("provider", filter.Provider)
	addTextFilter("model", filter.Model)
	addTextFilter("environment", filter.Environment)

	if filter.From != nil {
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filter.From)
		argIndex++
	}
	if filter.To != nil {
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filter.To)
		argIndex++
	}

	if len(clauses) == 0 {
		return "", args, argIndex
	}

	return "WHERE " + strings.Join(clauses, " AND "), args, argIndex
}

func scanEvent(row interface {
	Scan(dest ...any) error
}) (entity.LLMEvent, error) {
	var (
		event        entity.LLMEvent
		requestMeta  []byte
		responseMeta []byte
	)

	err := row.Scan(
		&event.ID,
		&event.Team,
		&event.Service,
		&event.Environment,
		&event.Endpoint,
		&event.Provider,
		&event.Model,
		&event.RequestType,
		&event.UserID,
		&event.TraceID,
		&event.PromptTokens,
		&event.CompletionTokens,
		&event.TotalTokens,
		&event.EstimatedCostUSD,
		&event.LatencyMs,
		&event.StatusCode,
		&event.Success,
		&event.RequestSummary,
		&requestMeta,
		&responseMeta,
		&event.CreatedAt,
	)
	if err != nil {
		return entity.LLMEvent{}, err
	}

	event.RequestMeta = requestMeta
	event.ResponseMeta = responseMeta
	return event, nil
}

func scanQuota(row interface {
	Scan(dest ...any) error
}) (entity.Quota, error) {
	var quota entity.Quota
	err := row.Scan(
		&quota.ID,
		&quota.Team,
		&quota.Service,
		&quota.DailyRequestLimit,
		&quota.DailyTokenLimit,
		&quota.DailyCostLimitUSD,
		&quota.HardLimit,
		&quota.Enabled,
		&quota.CreatedAt,
		&quota.UpdatedAt,
	)
	if err != nil {
		return entity.Quota{}, err
	}
	return quota, nil
}

func normalizeJSON(value []byte) []byte {
	if len(strings.TrimSpace(string(value))) == 0 {
		return []byte("{}")
	}
	return value
}

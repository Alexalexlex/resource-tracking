package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"example.com/fiber-hello/internal/entity"
	"example.com/fiber-hello/internal/repository"
)

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type TrackingService interface {
	CreateEvent(ctx context.Context, event entity.LLMEvent) (entity.IngestResult, error)
	ListEvents(ctx context.Context, filter entity.EventFilter) ([]entity.LLMEvent, error)
	GetOverview(ctx context.Context, filter entity.AnalyticsFilter) (entity.OverviewStats, error)
	GetTeamBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error)
	GetServiceBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error)
	GetModelBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error)
	GetHourlyTimeseries(ctx context.Context, filter entity.AnalyticsFilter, hours int) ([]entity.TimeseriesPoint, error)
	UpsertQuota(ctx context.Context, quota entity.Quota) (entity.Quota, error)
	ListQuotas(ctx context.Context, team, service string) ([]entity.Quota, error)
	BuildPrometheusMetrics(ctx context.Context) (string, error)
}

type trackingService struct {
	repo          repository.TrackingRepository
	quotaLocation *time.Location
}

func NewTrackingService(repo repository.TrackingRepository, quotaLocation *time.Location) TrackingService {
	if quotaLocation == nil {
		quotaLocation = time.UTC
	}
	return &trackingService{
		repo:          repo,
		quotaLocation: quotaLocation,
	}
}

func (s *trackingService) CreateEvent(ctx context.Context, event entity.LLMEvent) (entity.IngestResult, error) {
	event = sanitizeEvent(event)
	if err := validateEvent(event); err != nil {
		return entity.IngestResult{}, err
	}

	quotaStatus, err := s.evaluateQuota(ctx, event)
	if err != nil {
		return entity.IngestResult{}, err
	}
	if quotaStatus.Blocked {
		return entity.IngestResult{
			Accepted:    false,
			QuotaStatus: quotaStatus,
		}, nil
	}

	stored, err := s.repo.CreateEvent(ctx, event)
	if err != nil {
		return entity.IngestResult{}, err
	}

	return entity.IngestResult{
		Accepted:    true,
		Event:       &stored,
		QuotaStatus: quotaStatus,
	}, nil
}

func (s *trackingService) ListEvents(ctx context.Context, filter entity.EventFilter) ([]entity.LLMEvent, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.repo.ListEvents(ctx, filter)
}

func (s *trackingService) GetOverview(ctx context.Context, filter entity.AnalyticsFilter) (entity.OverviewStats, error) {
	return s.repo.GetOverview(ctx, filter)
}

func (s *trackingService) GetTeamBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error) {
	return s.repo.GetTeamBreakdown(ctx, filter, clampLimit(limit))
}

func (s *trackingService) GetServiceBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error) {
	return s.repo.GetServiceBreakdown(ctx, filter, clampLimit(limit))
}

func (s *trackingService) GetModelBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error) {
	return s.repo.GetModelBreakdown(ctx, filter, clampLimit(limit))
}

func (s *trackingService) GetHourlyTimeseries(ctx context.Context, filter entity.AnalyticsFilter, hours int) ([]entity.TimeseriesPoint, error) {
	if hours <= 0 || hours > 24*30 {
		hours = 24
	}
	return s.repo.GetHourlyTimeseries(ctx, filter, hours)
}

func (s *trackingService) UpsertQuota(ctx context.Context, quota entity.Quota) (entity.Quota, error) {
	quota.Team = strings.TrimSpace(quota.Team)
	quota.Service = strings.TrimSpace(quota.Service)

	if quota.Team == "" {
		return entity.Quota{}, ValidationError{Message: "team is required"}
	}
	if quota.DailyRequestLimit <= 0 && quota.DailyTokenLimit <= 0 && quota.DailyCostLimitUSD <= 0 {
		return entity.Quota{}, ValidationError{Message: "at least one limit must be greater than zero"}
	}

	return s.repo.UpsertQuota(ctx, quota)
}

func (s *trackingService) ListQuotas(ctx context.Context, team, service string) ([]entity.Quota, error) {
	return s.repo.ListQuotas(ctx, strings.TrimSpace(team), strings.TrimSpace(service))
}

func (s *trackingService) BuildPrometheusMetrics(ctx context.Context) (string, error) {
	overview, err := s.repo.GetOverview(ctx, entity.AnalyticsFilter{})
	if err != nil {
		return "", err
	}

	teams, err := s.repo.GetTeamBreakdown(ctx, entity.AnalyticsFilter{}, 100)
	if err != nil {
		return "", err
	}

	services, err := s.repo.GetServiceBreakdown(ctx, entity.AnalyticsFilter{}, 100)
	if err != nil {
		return "", err
	}

	quotas, err := s.repo.ListQuotas(ctx, "", "")
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("# HELP resource_tracking_requests_total Total number of tracked LLM requests.\n")
	builder.WriteString("# TYPE resource_tracking_requests_total counter\n")
	builder.WriteString(fmt.Sprintf("resource_tracking_requests_total %d\n", overview.TotalRequests))

	builder.WriteString("# HELP resource_tracking_failed_requests_total Total number of failed tracked LLM requests.\n")
	builder.WriteString("# TYPE resource_tracking_failed_requests_total counter\n")
	builder.WriteString(fmt.Sprintf("resource_tracking_failed_requests_total %d\n", overview.FailedRequests))

	builder.WriteString("# HELP resource_tracking_tokens_total Total number of tracked LLM tokens.\n")
	builder.WriteString("# TYPE resource_tracking_tokens_total counter\n")
	builder.WriteString(fmt.Sprintf("resource_tracking_tokens_total %d\n", overview.TotalTokens))

	builder.WriteString("# HELP resource_tracking_cost_usd_total Estimated accumulated LLM cost in USD.\n")
	builder.WriteString("# TYPE resource_tracking_cost_usd_total counter\n")
	builder.WriteString(fmt.Sprintf("resource_tracking_cost_usd_total %.6f\n", overview.TotalCostUSD))

	builder.WriteString("# HELP resource_tracking_latency_ms_average Average LLM latency in milliseconds.\n")
	builder.WriteString("# TYPE resource_tracking_latency_ms_average gauge\n")
	builder.WriteString(fmt.Sprintf("resource_tracking_latency_ms_average %.2f\n", overview.AverageLatencyMs))

	builder.WriteString("# HELP resource_tracking_configured_quotas Total number of configured quotas.\n")
	builder.WriteString("# TYPE resource_tracking_configured_quotas gauge\n")
	builder.WriteString(fmt.Sprintf("resource_tracking_configured_quotas %d\n", len(quotas)))

	builder.WriteString("# HELP resource_tracking_team_requests_total Tracked LLM requests aggregated by team.\n")
	builder.WriteString("# TYPE resource_tracking_team_requests_total gauge\n")
	for _, row := range teams {
		builder.WriteString(fmt.Sprintf(
			"resource_tracking_team_requests_total{team=\"%s\"} %d\n",
			escapeLabelValue(row.Team),
			row.Requests,
		))
	}

	builder.WriteString("# HELP resource_tracking_service_requests_total Tracked LLM requests aggregated by team and service.\n")
	builder.WriteString("# TYPE resource_tracking_service_requests_total gauge\n")
	for _, row := range services {
		builder.WriteString(fmt.Sprintf(
			"resource_tracking_service_requests_total{team=\"%s\",service=\"%s\"} %d\n",
			escapeLabelValue(row.Team),
			escapeLabelValue(row.Service),
			row.Requests,
		))
	}

	return builder.String(), nil
}

func (s *trackingService) evaluateQuota(ctx context.Context, event entity.LLMEvent) (entity.QuotaEvaluation, error) {
	quotas, err := s.repo.ListApplicableQuotas(ctx, event.Team, event.Service)
	if err != nil {
		return entity.QuotaEvaluation{}, err
	}

	windowStart, windowEnd := s.dayWindow(event.CreatedAt)
	evaluation := entity.QuotaEvaluation{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Decisions:   make([]entity.QuotaDecision, 0, len(quotas)),
		Warnings:    make([]string, 0),
	}

	for _, quota := range quotas {
		usage, err := s.repo.GetQuotaUsage(ctx, quota, windowStart, windowEnd)
		if err != nil {
			return entity.QuotaEvaluation{}, err
		}

		projected := entity.QuotaUsage{
			Requests:     usage.Requests + 1,
			TotalTokens:  usage.TotalTokens + int64(event.TotalTokens),
			TotalCostUSD: usage.TotalCostUSD + event.EstimatedCostUSD,
		}

		exceededFields := exceededQuotaFields(quota, projected)
		decision := entity.QuotaDecision{
			Quota:          quota,
			CurrentUsage:   usage,
			ProjectedUsage: projected,
			ExceededFields: exceededFields,
		}
		evaluation.Decisions = append(evaluation.Decisions, decision)

		if len(exceededFields) == 0 {
			continue
		}

		scope := quota.Team
		if quota.Service != "" {
			scope = fmt.Sprintf("%s/%s", quota.Team, quota.Service)
		}

		if quota.HardLimit {
			evaluation.Blocked = true
			evaluation.Warnings = append(
				evaluation.Warnings,
				fmt.Sprintf("hard quota exceeded for %s: %s", scope, strings.Join(exceededFields, ", ")),
			)
			continue
		}

		evaluation.Warnings = append(
			evaluation.Warnings,
			fmt.Sprintf("soft quota exceeded for %s: %s", scope, strings.Join(exceededFields, ", ")),
		)
	}

	return evaluation, nil
}

func (s *trackingService) dayWindow(base time.Time) (time.Time, time.Time) {
	if base.IsZero() {
		base = time.Now().UTC()
	}

	local := base.In(s.quotaLocation)
	windowStartLocal := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.quotaLocation)
	windowEndLocal := windowStartLocal.Add(24 * time.Hour)

	return windowStartLocal.UTC(), windowEndLocal.UTC()
}

func sanitizeEvent(event entity.LLMEvent) entity.LLMEvent {
	event.Team = strings.TrimSpace(event.Team)
	event.Service = strings.TrimSpace(event.Service)
	event.Environment = strings.TrimSpace(event.Environment)
	event.Endpoint = strings.TrimSpace(event.Endpoint)
	event.Provider = strings.TrimSpace(event.Provider)
	event.Model = strings.TrimSpace(event.Model)
	event.RequestType = strings.TrimSpace(event.RequestType)
	event.UserID = strings.TrimSpace(event.UserID)
	event.TraceID = strings.TrimSpace(event.TraceID)
	event.RequestSummary = strings.TrimSpace(event.RequestSummary)

	if event.Environment == "" {
		event.Environment = "unknown"
	}
	if event.StatusCode == 0 {
		if event.Success {
			event.StatusCode = 200
		} else {
			event.StatusCode = 500
		}
	}
	if event.TotalTokens == 0 {
		event.TotalTokens = event.PromptTokens + event.CompletionTokens
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}
	if event.RequestMeta == nil {
		event.RequestMeta = []byte("{}")
	}
	if event.ResponseMeta == nil {
		event.ResponseMeta = []byte("{}")
	}

	return event
}

func validateEvent(event entity.LLMEvent) error {
	switch {
	case event.Team == "":
		return ValidationError{Message: "team is required"}
	case event.Service == "":
		return ValidationError{Message: "service is required"}
	case event.Provider == "":
		return ValidationError{Message: "provider is required"}
	case event.Model == "":
		return ValidationError{Message: "model is required"}
	case event.PromptTokens < 0:
		return ValidationError{Message: "promptTokens cannot be negative"}
	case event.CompletionTokens < 0:
		return ValidationError{Message: "completionTokens cannot be negative"}
	case event.TotalTokens < 0:
		return ValidationError{Message: "totalTokens cannot be negative"}
	case event.EstimatedCostUSD < 0:
		return ValidationError{Message: "estimatedCostUsd cannot be negative"}
	case event.LatencyMs < 0:
		return ValidationError{Message: "latencyMs cannot be negative"}
	}

	return nil
}

func exceededQuotaFields(quota entity.Quota, projected entity.QuotaUsage) []string {
	fields := make([]string, 0, 3)

	if quota.DailyRequestLimit > 0 && projected.Requests > quota.DailyRequestLimit {
		fields = append(fields, "dailyRequestLimit")
	}
	if quota.DailyTokenLimit > 0 && projected.TotalTokens > quota.DailyTokenLimit {
		fields = append(fields, "dailyTokenLimit")
	}
	if quota.DailyCostLimitUSD > 0 && projected.TotalCostUSD > quota.DailyCostLimitUSD {
		fields = append(fields, "dailyCostLimitUsd")
	}

	return fields
}

func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return 10
	case limit > 100:
		return 100
	default:
		return limit
	}
}

func escapeLabelValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`, `"`, `\"`)
	return replacer.Replace(value)
}

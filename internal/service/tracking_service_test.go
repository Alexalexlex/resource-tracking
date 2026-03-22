package service

import (
	"context"
	"testing"
	"time"

	"example.com/fiber-hello/internal/entity"
)

type fakeTrackingRepo struct {
	createEventFn    func(ctx context.Context, event entity.LLMEvent) (entity.LLMEvent, error)
	listApplicableFn func(ctx context.Context, team, service string) ([]entity.Quota, error)
	getQuotaUsageFn  func(ctx context.Context, quota entity.Quota, from, to time.Time) (entity.QuotaUsage, error)
	getOverviewFn    func(ctx context.Context, filter entity.AnalyticsFilter) (entity.OverviewStats, error)
	getTeamFn        func(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error)
	getServiceFn     func(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error)
	getModelFn       func(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error)
	getTimeseriesFn  func(ctx context.Context, filter entity.AnalyticsFilter, hours int) ([]entity.TimeseriesPoint, error)
	upsertQuotaFn    func(ctx context.Context, quota entity.Quota) (entity.Quota, error)
	listQuotasFn     func(ctx context.Context, team, service string) ([]entity.Quota, error)
}

func (f fakeTrackingRepo) CreateEvent(ctx context.Context, event entity.LLMEvent) (entity.LLMEvent, error) {
	return f.createEventFn(ctx, event)
}

func (f fakeTrackingRepo) ListEvents(ctx context.Context, filter entity.EventFilter) ([]entity.LLMEvent, error) {
	return nil, nil
}

func (f fakeTrackingRepo) GetOverview(ctx context.Context, filter entity.AnalyticsFilter) (entity.OverviewStats, error) {
	if f.getOverviewFn == nil {
		return entity.OverviewStats{}, nil
	}
	return f.getOverviewFn(ctx, filter)
}

func (f fakeTrackingRepo) GetTeamBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error) {
	if f.getTeamFn == nil {
		return nil, nil
	}
	return f.getTeamFn(ctx, filter, limit)
}

func (f fakeTrackingRepo) GetServiceBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error) {
	if f.getServiceFn == nil {
		return nil, nil
	}
	return f.getServiceFn(ctx, filter, limit)
}

func (f fakeTrackingRepo) GetModelBreakdown(ctx context.Context, filter entity.AnalyticsFilter, limit int) ([]entity.BreakdownRow, error) {
	if f.getModelFn == nil {
		return nil, nil
	}
	return f.getModelFn(ctx, filter, limit)
}

func (f fakeTrackingRepo) GetHourlyTimeseries(ctx context.Context, filter entity.AnalyticsFilter, hours int) ([]entity.TimeseriesPoint, error) {
	if f.getTimeseriesFn == nil {
		return nil, nil
	}
	return f.getTimeseriesFn(ctx, filter, hours)
}

func (f fakeTrackingRepo) ListApplicableQuotas(ctx context.Context, team, service string) ([]entity.Quota, error) {
	if f.listApplicableFn == nil {
		return nil, nil
	}
	return f.listApplicableFn(ctx, team, service)
}

func (f fakeTrackingRepo) GetQuotaUsage(ctx context.Context, quota entity.Quota, from, to time.Time) (entity.QuotaUsage, error) {
	if f.getQuotaUsageFn == nil {
		return entity.QuotaUsage{}, nil
	}
	return f.getQuotaUsageFn(ctx, quota, from, to)
}

func (f fakeTrackingRepo) UpsertQuota(ctx context.Context, quota entity.Quota) (entity.Quota, error) {
	if f.upsertQuotaFn == nil {
		return quota, nil
	}
	return f.upsertQuotaFn(ctx, quota)
}

func (f fakeTrackingRepo) ListQuotas(ctx context.Context, team, service string) ([]entity.Quota, error) {
	if f.listQuotasFn == nil {
		return nil, nil
	}
	return f.listQuotasFn(ctx, team, service)
}

func TestCreateEventBlocksHardQuota(t *testing.T) {
	repo := fakeTrackingRepo{
		createEventFn: func(ctx context.Context, event entity.LLMEvent) (entity.LLMEvent, error) {
			t.Fatal("event must not be stored when hard quota is exceeded")
			return entity.LLMEvent{}, nil
		},
		listApplicableFn: func(ctx context.Context, team, service string) ([]entity.Quota, error) {
			return []entity.Quota{
				{
					Team:              "team-a",
					Service:           "svc-a",
					DailyRequestLimit: 10,
					HardLimit:         true,
					Enabled:           true,
				},
			}, nil
		},
		getQuotaUsageFn: func(ctx context.Context, quota entity.Quota, from, to time.Time) (entity.QuotaUsage, error) {
			return entity.QuotaUsage{Requests: 10}, nil
		},
	}

	svc := NewTrackingService(repo, time.UTC)
	result, err := svc.CreateEvent(context.Background(), entity.LLMEvent{
		Team:     "team-a",
		Service:  "svc-a",
		Provider: "openai",
		Model:    "gpt-4o-mini",
		Success:  true,
	})
	if err != nil {
		t.Fatalf("CreateEvent returned error: %v", err)
	}
	if result.Accepted {
		t.Fatalf("expected event to be rejected")
	}
	if !result.QuotaStatus.Blocked {
		t.Fatalf("expected quota status to be blocked")
	}
	if len(result.QuotaStatus.Decisions) != 1 {
		t.Fatalf("expected one quota decision, got %d", len(result.QuotaStatus.Decisions))
	}
}

func TestCreateEventStoresSoftQuotaWarning(t *testing.T) {
	var stored entity.LLMEvent
	repo := fakeTrackingRepo{
		createEventFn: func(ctx context.Context, event entity.LLMEvent) (entity.LLMEvent, error) {
			stored = event
			event.ID = 42
			return event, nil
		},
		listApplicableFn: func(ctx context.Context, team, service string) ([]entity.Quota, error) {
			return []entity.Quota{
				{
					Team:            "team-a",
					DailyTokenLimit: 100,
					HardLimit:       false,
					Enabled:         true,
				},
			}, nil
		},
		getQuotaUsageFn: func(ctx context.Context, quota entity.Quota, from, to time.Time) (entity.QuotaUsage, error) {
			return entity.QuotaUsage{TotalTokens: 90}, nil
		},
	}

	svc := NewTrackingService(repo, time.UTC)
	result, err := svc.CreateEvent(context.Background(), entity.LLMEvent{
		Team:             "team-a",
		Service:          "svc-a",
		Provider:         "openai",
		Model:            "gpt-4o-mini",
		PromptTokens:     10,
		CompletionTokens: 5,
		Success:          true,
	})
	if err != nil {
		t.Fatalf("CreateEvent returned error: %v", err)
	}
	if !result.Accepted {
		t.Fatalf("expected event to be accepted")
	}
	if result.Event == nil || result.Event.ID != 42 {
		t.Fatalf("expected stored event with id 42")
	}
	if stored.TotalTokens != 15 {
		t.Fatalf("expected total tokens to be auto-calculated, got %d", stored.TotalTokens)
	}
	if len(result.QuotaStatus.Warnings) == 0 {
		t.Fatalf("expected a soft quota warning")
	}
}

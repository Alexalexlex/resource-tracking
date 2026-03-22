package entity

import (
	"encoding/json"
	"time"
)

type AnalyticsFilter struct {
	Team        string
	Service     string
	Provider    string
	Model       string
	Environment string
	From        *time.Time
	To          *time.Time
}

type EventFilter struct {
	AnalyticsFilter
	Limit  int
	Offset int
}

type LLMEvent struct {
	ID               int64           `json:"id"`
	Team             string          `json:"team"`
	Service          string          `json:"service"`
	Environment      string          `json:"environment"`
	Endpoint         string          `json:"endpoint"`
	Provider         string          `json:"provider"`
	Model            string          `json:"model"`
	RequestType      string          `json:"requestType"`
	UserID           string          `json:"userId"`
	TraceID          string          `json:"traceId"`
	PromptTokens     int             `json:"promptTokens"`
	CompletionTokens int             `json:"completionTokens"`
	TotalTokens      int             `json:"totalTokens"`
	EstimatedCostUSD float64         `json:"estimatedCostUsd"`
	LatencyMs        int             `json:"latencyMs"`
	StatusCode       int             `json:"statusCode"`
	Success          bool            `json:"success"`
	RequestSummary   string          `json:"requestSummary"`
	RequestMeta      json.RawMessage `json:"requestMeta"`
	ResponseMeta     json.RawMessage `json:"responseMeta"`
	CreatedAt        time.Time       `json:"createdAt"`
}

type OverviewStats struct {
	TotalRequests      int64   `json:"totalRequests"`
	SuccessfulRequests int64   `json:"successfulRequests"`
	FailedRequests     int64   `json:"failedRequests"`
	Teams              int64   `json:"teams"`
	Services           int64   `json:"services"`
	TotalTokens        int64   `json:"totalTokens"`
	TotalCostUSD       float64 `json:"totalCostUsd"`
	AverageLatencyMs   float64 `json:"averageLatencyMs"`
	P95LatencyMs       float64 `json:"p95LatencyMs"`
}

type BreakdownRow struct {
	Team             string  `json:"team,omitempty"`
	Service          string  `json:"service,omitempty"`
	Provider         string  `json:"provider,omitempty"`
	Model            string  `json:"model,omitempty"`
	Requests         int64   `json:"requests"`
	FailedRequests   int64   `json:"failedRequests"`
	TotalTokens      int64   `json:"totalTokens"`
	TotalCostUSD     float64 `json:"totalCostUsd"`
	AverageLatencyMs float64 `json:"averageLatencyMs"`
}

type TimeseriesPoint struct {
	Bucket           time.Time `json:"bucket"`
	Requests         int64     `json:"requests"`
	FailedRequests   int64     `json:"failedRequests"`
	TotalTokens      int64     `json:"totalTokens"`
	TotalCostUSD     float64   `json:"totalCostUsd"`
	AverageLatencyMs float64   `json:"averageLatencyMs"`
}

type Quota struct {
	ID                int64     `json:"id"`
	Team              string    `json:"team"`
	Service           string    `json:"service"`
	DailyRequestLimit int64     `json:"dailyRequestLimit"`
	DailyTokenLimit   int64     `json:"dailyTokenLimit"`
	DailyCostLimitUSD float64   `json:"dailyCostLimitUsd"`
	HardLimit         bool      `json:"hardLimit"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type QuotaUsage struct {
	Requests     int64   `json:"requests"`
	TotalTokens  int64   `json:"totalTokens"`
	TotalCostUSD float64 `json:"totalCostUsd"`
}

type QuotaDecision struct {
	Quota          Quota      `json:"quota"`
	CurrentUsage   QuotaUsage `json:"currentUsage"`
	ProjectedUsage QuotaUsage `json:"projectedUsage"`
	ExceededFields []string   `json:"exceededFields,omitempty"`
}

type QuotaEvaluation struct {
	WindowStart time.Time       `json:"windowStart"`
	WindowEnd   time.Time       `json:"windowEnd"`
	Blocked     bool            `json:"blocked"`
	Warnings    []string        `json:"warnings,omitempty"`
	Decisions   []QuotaDecision `json:"decisions"`
}

type IngestResult struct {
	Accepted    bool            `json:"accepted"`
	Event       *LLMEvent       `json:"event,omitempty"`
	QuotaStatus QuotaEvaluation `json:"quotaStatus"`
}

<img width="837" height="757" alt="Resource Tracking System" src="https://github.com/user-attachments/assets/cb2ad767-0960-492b-8d6a-3e065046d3bd" />

# Resource Tracking System for LLM Integration

MVP backend for collecting, analyzing, and limiting LLM traffic across microservices.

## Why use it

This service is useful when several teams or microservices call LLM providers independently and you need one place to answer:

- which team uses the most tokens
- which service creates the most expensive traffic
- which model is used most often
- how latency and failure rate change over time
- whether a team or service is close to its daily quota

In practice, this service is used by:

- platform teams to monitor shared LLM usage
- engineering managers to understand cost by team
- service owners to debug expensive or slow prompts
- ops teams to expose metrics into Grafana

## What this service does

- Accepts structured telemetry events from any microservice that calls an LLM provider.
- Stores request metadata in PostgreSQL.
- Builds dashboard-ready aggregates by team, service, model, and time window.
- Applies daily quotas at team or service level.
- Exposes Prometheus-compatible `/metrics` for Grafana dashboards.

## Recommended architecture

For the first production iteration, use a lightweight integration in every microservice instead of a mandatory central proxy:

1. A microservice calls OpenAI, Anthropic, or another LLM provider.
2. Right after the call finishes, the service sends a small event to this tracking backend.
3. This backend stores the event, evaluates quota rules, and exposes analytics APIs plus `/metrics`.
4. Prometheus scrapes `/metrics`, and Grafana becomes the UI for team-level dashboards.

This approach is simpler than an inline proxy, avoids becoming a latency bottleneck, and still gives enough visibility for cost and usage control.

## How it is used

The normal flow looks like this:

1. A business microservice sends a request to an LLM provider.
2. The microservice receives the provider response.
3. The microservice sends a small telemetry event to this tracking service.
4. The tracking service stores the event in PostgreSQL.
5. The tracking service evaluates quotas and returns warnings or a block decision.
6. Prometheus scrapes `/metrics`, and Grafana visualizes aggregate usage.
7. Engineers use dashboard APIs for drill-down into specific teams, services, or models.

Important operational rule: this service should collect metadata, not become part of the critical path of the LLM request itself. The simplest integration is to send tracking data after the provider call completes.

## Data model

Each tracked event can include:

- `team`
- `service`
- `environment`
- `endpoint`
- `provider`
- `model`
- `requestType`
- `userId`
- `traceId`
- `promptTokens`
- `completionTokens`
- `totalTokens`
- `estimatedCostUsd`
- `latencyMs`
- `statusCode`
- `success`
- `requestSummary`
- `requestMeta`
- `responseMeta`
- `createdAt`

Recommended minimum fields for every integration:

- `team`
- `service`
- `environment`
- `provider`
- `model`
- `promptTokens`
- `completionTokens`
- `estimatedCostUsd`
- `latencyMs`
- `statusCode`
- `success`

Recommended privacy approach:

- store a short `requestSummary` instead of full prompts when possible
- put technical metadata into `requestMeta` and `responseMeta`
- do not store secrets, raw access tokens, or sensitive personal data

## API

### Ingest event

`POST /api/v1/events`

```json
{
  "team": "team-1",
  "service": "search-api",
  "environment": "prod",
  "endpoint": "/v1/chat/completions",
  "provider": "openai",
  "model": "gpt-4o-mini",
  "requestType": "support-answer",
  "userId": "user-42",
  "traceId": "trace-9f4d",
  "promptTokens": 1200,
  "completionTokens": 320,
  "estimatedCostUsd": 0.0234,
  "latencyMs": 1850,
  "statusCode": 200,
  "success": true,
  "requestSummary": "Answer a support request about billing",
  "requestMeta": {
    "region": "eu-central-1",
    "feature": "support-bot"
  },
  "responseMeta": {
    "finishReason": "stop"
  },
  "createdAt": "2026-03-22T12:00:00Z"
}
```

Response behavior:

- `201 Created` if the event is accepted and stored
- `429 Too Many Requests` if a hard quota is exceeded
- `400 Bad Request` if required fields are invalid

### Dashboard APIs

- `GET /api/v1/dashboard/overview`
- `GET /api/v1/dashboard/teams?limit=10`
- `GET /api/v1/dashboard/services?limit=10`
- `GET /api/v1/dashboard/models?limit=10`
- `GET /api/v1/dashboard/timeseries?hours=24`
- `GET /api/v1/events?team=team-1&service=search-api&limit=50`

Supported filters:

- `team`
- `service`
- `provider`
- `model`
- `environment`
- `from` in RFC3339
- `to` in RFC3339

### Quotas

- `POST /api/v1/quotas`
- `GET /api/v1/quotas`

Example quota:

```json
{
  "team": "team-1",
  "service": "search-api",
  "dailyRequestLimit": 5000,
  "dailyTokenLimit": 2000000,
  "dailyCostLimitUsd": 50,
  "hardLimit": true,
  "enabled": true
}
```

If a hard quota is exceeded, ingestion returns `429 Too Many Requests`. Soft quotas store the event but add warnings in the response.

## Local run

1. Create PostgreSQL database.
2. Copy `.env.example` values into your environment.
3. Start the app:

```bash
go run ./cmd/server
```

The service auto-creates the required tables on startup.

## Quick start

### 1. Start the service

```bash
go run ./cmd/server
```

### 2. Create a quota

```bash
curl -X POST http://localhost:8080/api/v1/quotas \
  -H 'Content-Type: application/json' \
  -d '{
    "team": "team-1",
    "service": "search-api",
    "dailyRequestLimit": 5000,
    "dailyTokenLimit": 2000000,
    "dailyCostLimitUsd": 50,
    "hardLimit": false,
    "enabled": true
  }'
```

### 3. Send one tracking event

```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H 'Content-Type: application/json' \
  -d '{
    "team": "team-1",
    "service": "search-api",
    "environment": "prod",
    "provider": "openai",
    "model": "gpt-4o-mini",
    "promptTokens": 1200,
    "completionTokens": 320,
    "estimatedCostUsd": 0.0234,
    "latencyMs": 1850,
    "statusCode": 200,
    "success": true,
    "requestSummary": "Answer a support request about billing"
  }'
```

### 4. Check the dashboard APIs

```bash
curl http://localhost:8080/api/v1/dashboard/overview
curl http://localhost:8080/api/v1/dashboard/teams?limit=10
curl http://localhost:8080/api/v1/dashboard/services?limit=10
curl http://localhost:8080/api/v1/dashboard/timeseries?hours=24
```

### 5. Check Prometheus metrics

```bash
curl http://localhost:8080/metrics
```

## Simple integration guide

The easiest way to integrate a microservice is:

1. Call the LLM provider as usual.
2. Extract usage and technical metadata from the provider response.
3. Build a tracking event.
4. Send it to `POST /api/v1/events`.
5. Do it with a short timeout and do not fail the main business request if tracking is unavailable.

### What to send from a microservice

Each service should set stable identifiers:

- `team`: owning team, for example `team-1`
- `service`: microservice name, for example `search-api`
- `environment`: `dev`, `stage`, or `prod`
- `provider`: `openai`, `anthropic`, and so on
- `model`: actual model name used in the request

Each request should add dynamic data:

- latency
- token usage
- estimated cost
- success or failure
- HTTP status or provider status
- short business summary such as `rag-answer`, `support-reply`, `classification`

### Integration rules

- Send tracking data after the LLM response is received.
- Use asynchronous sending if the service is latency-sensitive.
- Keep payloads small. Prefer summaries and metadata over full prompts.
- Use one consistent `team` and `service` name so dashboards stay clean.
- If provider usage data is missing, send `latencyMs`, `success`, and `statusCode` anyway.

### Minimal Go example

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type TrackingEvent struct {
	Team             string  `json:"team"`
	Service          string  `json:"service"`
	Environment      string  `json:"environment"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	EstimatedCostUSD float64 `json:"estimatedCostUsd"`
	LatencyMs        int     `json:"latencyMs"`
	StatusCode       int     `json:"statusCode"`
	Success          bool    `json:"success"`
	RequestSummary   string  `json:"requestSummary"`
}

func sendTrackingEvent(event TrackingEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"http://localhost:8080/api/v1/events",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
```

### Where to place integration in service code

The best place is right after the LLM provider call, when you already know:

- model used
- request latency
- token counts
- success or failure
- cost estimate

Typical placement:

- in a shared LLM client wrapper
- in provider adapter code
- in middleware around AI-specific business operations

If multiple services use the same internal SDK for LLM calls, put tracking there once instead of duplicating it everywhere.

## How to use the data

Use `overview` when you want a high-level health and cost snapshot.

Use `teams` when you want to compare adoption and cost across teams.

Use `services` when you want to find which service is producing most traffic or failures.

Use `models` when you want to understand which models drive cost and latency.

Use `timeseries` when you want to see spikes, regressions, or daily usage patterns.

Use `events` when you need a detailed recent activity feed for debugging.

Use `quotas` when you need soft warnings or hard daily caps for a team or service.

## Grafana setup

Recommended UI stack:

- This service as collector and analytics API
- Prometheus for scraping `/metrics`
- Grafana for dashboards

Sample Prometheus config is in `docs/prometheus.yml`.

Useful dashboard panels:

- total requests by team
- total tokens by service
- estimated cost by model
- failure rate by provider
- average and p95 latency
- quota violations by team/service

Use JSON APIs for event drill-down and Grafana for aggregate monitoring.

## Recommended rollout

For the first rollout:

1. Integrate 1 or 2 services only.
2. Verify that `team` and `service` naming is consistent.
3. Check dashboard aggregates and `/metrics`.
4. Add quotas in soft mode first.
5. Switch selected quotas to hard mode only after you trust the data quality.

This keeps adoption simple and avoids breaking production traffic because of bad or incomplete telemetry.

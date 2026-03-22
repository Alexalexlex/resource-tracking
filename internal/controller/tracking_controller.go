package controller

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"example.com/fiber-hello/internal/entity"
	"example.com/fiber-hello/internal/service"
	"github.com/gofiber/fiber/v2"
)

type TrackingController struct {
	service service.TrackingService
}

type ingestEventDTO struct {
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
	Success          *bool           `json:"success"`
	RequestSummary   string          `json:"requestSummary"`
	RequestMeta      json.RawMessage `json:"requestMeta"`
	ResponseMeta     json.RawMessage `json:"responseMeta"`
	CreatedAt        string          `json:"createdAt"`
}

type upsertQuotaDTO struct {
	Team              string  `json:"team"`
	Service           string  `json:"service"`
	DailyRequestLimit int64   `json:"dailyRequestLimit"`
	DailyTokenLimit   int64   `json:"dailyTokenLimit"`
	DailyCostLimitUSD float64 `json:"dailyCostLimitUsd"`
	HardLimit         bool    `json:"hardLimit"`
	Enabled           *bool   `json:"enabled"`
}

func NewTrackingController(service service.TrackingService) *TrackingController {
	return &TrackingController{service: service}
}

func (c *TrackingController) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	api.Post("/events", c.IngestEvent)
	api.Get("/events", c.ListEvents)
	api.Get("/quotas", c.ListQuotas)
	api.Post("/quotas", c.UpsertQuota)

	dashboard := api.Group("/dashboard")
	dashboard.Get("/overview", c.GetOverview)
	dashboard.Get("/teams", c.GetTeamBreakdown)
	dashboard.Get("/services", c.GetServiceBreakdown)
	dashboard.Get("/models", c.GetModelBreakdown)
	dashboard.Get("/timeseries", c.GetTimeseries)

	app.Get("/metrics", c.GetMetrics)
}

func (c *TrackingController) IngestEvent(ctx *fiber.Ctx) error {
	var req ingestEventDTO
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body: "+err.Error())
	}

	event, err := mapEventDTO(req)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	result, err := c.service.CreateEvent(ctx.Context(), event)
	if err != nil {
		return mapHTTPError(err)
	}
	if !result.Accepted {
		return ctx.Status(fiber.StatusTooManyRequests).JSON(result)
	}

	return ctx.Status(fiber.StatusCreated).JSON(result)
}

func (c *TrackingController) ListEvents(ctx *fiber.Ctx) error {
	events, err := c.service.ListEvents(ctx.Context(), entity.EventFilter{
		AnalyticsFilter: parseAnalyticsFilter(ctx),
		Limit:           parseBoundedInt(ctx.Query("limit"), 50, 1, 500),
		Offset:          parseBoundedInt(ctx.Query("offset"), 0, 0, 100000),
	})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(fiber.Map{"data": events})
}

func (c *TrackingController) GetOverview(ctx *fiber.Ctx) error {
	overview, err := c.service.GetOverview(ctx.Context(), parseAnalyticsFilter(ctx))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(overview)
}

func (c *TrackingController) GetTeamBreakdown(ctx *fiber.Ctx) error {
	breakdown, err := c.service.GetTeamBreakdown(
		ctx.Context(),
		parseAnalyticsFilter(ctx),
		parseBoundedInt(ctx.Query("limit"), 10, 1, 100),
	)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(fiber.Map{"data": breakdown})
}

func (c *TrackingController) GetServiceBreakdown(ctx *fiber.Ctx) error {
	breakdown, err := c.service.GetServiceBreakdown(
		ctx.Context(),
		parseAnalyticsFilter(ctx),
		parseBoundedInt(ctx.Query("limit"), 10, 1, 100),
	)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(fiber.Map{"data": breakdown})
}

func (c *TrackingController) GetModelBreakdown(ctx *fiber.Ctx) error {
	breakdown, err := c.service.GetModelBreakdown(
		ctx.Context(),
		parseAnalyticsFilter(ctx),
		parseBoundedInt(ctx.Query("limit"), 10, 1, 100),
	)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(fiber.Map{"data": breakdown})
}

func (c *TrackingController) GetTimeseries(ctx *fiber.Ctx) error {
	points, err := c.service.GetHourlyTimeseries(
		ctx.Context(),
		parseAnalyticsFilter(ctx),
		parseBoundedInt(ctx.Query("hours"), 24, 1, 24*30),
	)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(fiber.Map{"data": points})
}

func (c *TrackingController) UpsertQuota(ctx *fiber.Ctx) error {
	var req upsertQuotaDTO
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body: "+err.Error())
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	quota, err := c.service.UpsertQuota(ctx.Context(), entity.Quota{
		Team:              req.Team,
		Service:           req.Service,
		DailyRequestLimit: req.DailyRequestLimit,
		DailyTokenLimit:   req.DailyTokenLimit,
		DailyCostLimitUSD: req.DailyCostLimitUSD,
		HardLimit:         req.HardLimit,
		Enabled:           enabled,
	})
	if err != nil {
		return mapHTTPError(err)
	}

	return ctx.Status(fiber.StatusCreated).JSON(quota)
}

func (c *TrackingController) ListQuotas(ctx *fiber.Ctx) error {
	quotas, err := c.service.ListQuotas(
		ctx.Context(),
		ctx.Query("team"),
		ctx.Query("service"),
	)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(fiber.Map{"data": quotas})
}

func (c *TrackingController) GetMetrics(ctx *fiber.Ctx) error {
	metrics, err := c.service.BuildPrometheusMetrics(ctx.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	ctx.Set(fiber.HeaderContentType, "text/plain; version=0.0.4; charset=utf-8")
	return ctx.SendString(metrics)
}

func mapEventDTO(req ingestEventDTO) (entity.LLMEvent, error) {
	success := true
	if req.Success != nil {
		success = *req.Success
	}

	var createdAt time.Time
	if req.CreatedAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.CreatedAt)
		if err != nil {
			return entity.LLMEvent{}, fiber.NewError(fiber.StatusBadRequest, "createdAt must be RFC3339")
		}
		createdAt = parsed
	}

	return entity.LLMEvent{
		Team:             req.Team,
		Service:          req.Service,
		Environment:      req.Environment,
		Endpoint:         req.Endpoint,
		Provider:         req.Provider,
		Model:            req.Model,
		RequestType:      req.RequestType,
		UserID:           req.UserID,
		TraceID:          req.TraceID,
		PromptTokens:     req.PromptTokens,
		CompletionTokens: req.CompletionTokens,
		TotalTokens:      req.TotalTokens,
		EstimatedCostUSD: req.EstimatedCostUSD,
		LatencyMs:        req.LatencyMs,
		StatusCode:       req.StatusCode,
		Success:          success,
		RequestSummary:   req.RequestSummary,
		RequestMeta:      req.RequestMeta,
		ResponseMeta:     req.ResponseMeta,
		CreatedAt:        createdAt,
	}, nil
}

func parseAnalyticsFilter(ctx *fiber.Ctx) entity.AnalyticsFilter {
	return entity.AnalyticsFilter{
		Team:        ctx.Query("team"),
		Service:     ctx.Query("service"),
		Provider:    ctx.Query("provider"),
		Model:       ctx.Query("model"),
		Environment: ctx.Query("environment"),
		From:        parseTimeQuery(ctx.Query("from")),
		To:          parseTimeQuery(ctx.Query("to")),
	}
}

func parseBoundedInt(raw string, fallback, min, max int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func parseTimeQuery(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func mapHTTPError(err error) error {
	var validationErr service.ValidationError
	if errors.As(err, &validationErr) {
		return fiber.NewError(fiber.StatusBadRequest, validationErr.Error())
	}
	return fiber.NewError(fiber.StatusInternalServerError, err.Error())
}

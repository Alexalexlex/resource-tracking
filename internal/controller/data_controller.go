package controller

import (
	"encoding/json"

	"example.com/fiber-hello/internal/entity"
	"example.com/fiber-hello/internal/service"
	"github.com/gofiber/fiber/v2"
)

type DataController struct {
	service service.DataService
}

func NewDataController(service service.DataService) *DataController {
	return &DataController{service: service}
}

func (c *DataController) RegisterRoutes(app *fiber.App) {
	app.Post("/send_data", c.SendData)
}

type createDataDTO struct {
	Path   string          `json:"path"`
	Source string          `json:"source"`
	Meta   json.RawMessage `json:"meta"`
}

func (c *DataController) SendData(ctx *fiber.Ctx) error {
	var req createDataDTO

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body: "+err.Error())
	}

	id := c.service.SendData(ctx.Context(), entity.Data{
		Path:   req.Path,
		Source: req.Source,
		Meta:   req.Meta,
	})

	return ctx.JSON(id)
}

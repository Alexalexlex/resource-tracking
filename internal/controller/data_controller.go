package controller

import (
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
	app.Get("/send_data", c.SendData)
}

func (c *DataController) SendData(ctx *fiber.Ctx) error {
	users := c.service.SendData()
	return ctx.JSON(users)
}

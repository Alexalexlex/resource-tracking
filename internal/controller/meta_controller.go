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
	app.Get("/users", c.GetUsers)
}

func (c *DataController) GetUsers(ctx *fiber.Ctx) error {
	users := c.service.GetAllUsers()
	return ctx.JSON(users)
}

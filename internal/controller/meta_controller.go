package controller

import (
	"example.com/fiber-hello/internal/service"
	"github.com/gofiber/fiber/v2"
)

type MetaController struct {
	service service.MetaService
}

func NewMetaController(service service.MetaService) *MetaController {
	return &MetaController{service: service}
}

func (c *MetaController) RegisterRoutes(app *fiber.App) {
	app.Get("/users", c.GetUsers)
}

func (c *MetaController) GetUsers(ctx *fiber.Ctx) error {
	users := c.service.GetAllUsers()
	return ctx.JSON(users)
}

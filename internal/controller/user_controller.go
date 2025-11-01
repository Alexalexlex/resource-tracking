package controller

import (
	"example.com/fiber-hello/internal/service"
	"github.com/gofiber/fiber/v2"
)

type UserController struct {
	service service.UserService
}

func NewUserController(service service.UserService) *UserController {
	return &UserController{service: service}
}

func (c *UserController) RegisterRoutes(app *fiber.App) {
	app.Get("/users", c.GetUsers)
}

func (c *UserController) GetUsers(ctx *fiber.Ctx) error {
	users := c.service.GetAllUsers()
	return ctx.JSON(users)
}

package main

import (
	"log"
	"os"

	"example.com/fiber-hello/internal/controller"
	"example.com/fiber-hello/internal/repository"
	"example.com/fiber-hello/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	app := fiber.New()

	// Инициализация слоёв
	metaRepo := repository.NewMetaRepository()
	metaService := service.NewMetaService(metaRepo)
	metaController := controller.NewMetaController(metaService)

	// Register user routes
	metaController.RegisterRoutes(app)

	app.Use(func(c *fiber.Ctx) error { // простой логер
		log.Printf("%s %s", c.Method(), c.Path())
		return c.Next()
	})
	app.Use(recover.New())

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Content-Type, Authorization",
	}))

	// Простые роуты
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Hello, Fiber!"})
	})
	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.SendString("pong")
	})

	// Порт из ENV (по умолчанию 8080)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(app.Listen(":" + port))
}

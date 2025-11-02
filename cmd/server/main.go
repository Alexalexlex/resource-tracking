package main

import (
	"context"
	"log"
	"os"
	"time"

	"example.com/fiber-hello/internal/controller"
	"example.com/fiber-hello/internal/db"
	"example.com/fiber-hello/internal/repository"
	"example.com/fiber-hello/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// Контекст для старта БД
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	app := fiber.New()

	// Инициализация слоёв
	dataRepo := repository.NewDataRepository(pool)
	dataService := service.NewDataService(dataRepo)
	dataController := controller.NewDataController(dataService)

	// Register user routes
	dataController.RegisterRoutes(app)

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

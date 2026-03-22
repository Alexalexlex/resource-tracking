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

	if err := db.EnsureSchema(ctx, pool); err != nil {
		log.Fatalf("failed to initialize schema: %v", err)
	}

	app := fiber.New()

	app.Use(func(c *fiber.Ctx) error {
		log.Printf("%s %s", c.Method(), c.Path())
		return c.Next()
	})
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Content-Type, Authorization",
	}))

	quotaTZ := loadLocation(os.Getenv("QUOTA_TZ"))

	trackingRepo := repository.NewTrackingRepository(pool)
	trackingService := service.NewTrackingService(trackingRepo, quotaTZ)
	trackingController := controller.NewTrackingController(trackingService)
	trackingController.RegisterRoutes(app)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service": "resource-tracking-system",
			"version": "mvp",
			"docs": fiber.Map{
				"events":     "/api/v1/events",
				"overview":   "/api/v1/dashboard/overview",
				"timeseries": "/api/v1/dashboard/timeseries",
				"quotas":     "/api/v1/quotas",
				"metrics":    "/metrics",
			},
		})
	})
	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.SendString("pong")
	})
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(app.Listen(":" + port))
}

func loadLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		log.Printf("failed to load QUOTA_TZ=%q, falling back to UTC: %v", name, err)
		return time.UTC
	}
	return location
}

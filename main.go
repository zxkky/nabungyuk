package main

import (
	"log"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/nabungyuk/nabungyuk/config"
	"github.com/nabungyuk/nabungyuk/middleware"
	"github.com/nabungyuk/nabungyuk/routes"
	"github.com/nabungyuk/nabungyuk/services"
)

// allowedOrigins builds the CORS whitelist from the CORS_ALLOWED_ORIGINS env var.
// If not set, falls back to a safe default for local development.
func allowedOrigins() []string {
	origins := config.GetEnv("CORS_ALLOWED_ORIGINS", "")
	if origins == "" {
		// Safe default for local development
		return []string{"http://localhost:3000", "http://localhost:8081", "http://127.0.0.1:3000", "http://127.0.0.1:8081"}
	}
	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func main() {
	// Load environment variables (must happen before middleware init reads env)
	config.LoadEnv()

	log.Println("=== NabungYuk Personal Finance App ===")
	log.Println("Kelola uang, capai impian.")
	log.Println()

	// Load and validate JWT secret (must be after LoadEnv so .env values are available)
	middleware.LoadJWTSecret()

	// Setup database
	config.SetupDatabase()
	log.Println("Database setup completed")

	// Run migrations
	config.MigrateDB()
	log.Println("Database migration completed")

	// Create Gin router
	router := gin.Default()
	if proxies := strings.TrimSpace(config.GetEnv("TRUSTED_PROXIES", "")); proxies != "" {
		var trusted []string
		for _, proxy := range strings.Split(proxies, ",") {
			if proxy = strings.TrimSpace(proxy); proxy != "" {
				trusted = append(trusted, proxy)
			}
		}
		if err := router.SetTrustedProxies(trusted); err != nil {
			log.Fatalf("Invalid TRUSTED_PROXIES: %v", err)
		}
	} else {
		// Do not trust X-Forwarded-For unless a reverse proxy is explicitly configured.
		_ = router.SetTrustedProxies(nil)
	}

	// Configure CORS with restricted origin whitelist
	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Serve static files
	router.Static("/static", "./static")

	// Root path serves index.html (login page)
	router.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	// Serve all HTML pages from static folder
	for _, html := range []string{"index.html", "register.html", "dashboard.html", "transactions.html", "savings.html", "reminders.html", "reports.html"} {
		router.GET("/"+html, func(c *gin.Context) {
			c.File("./static/" + html)
		})
	}

	// Setup routes
	routes.SetupRoutes(router)

	// Start reminder scheduler (background worker)
	if smtpInterval := config.GetEnv("REMINDER_CHECK_INTERVAL", "30s"); smtpInterval != "disabled" {
		interval, err := time.ParseDuration(smtpInterval)
		if err != nil {
			interval = 30 * time.Second
		}
		emailService := services.NewEmailService()
		scheduler := services.NewScheduler(emailService)
		scheduler.Start(interval)
		defer scheduler.Stop()
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "NabungYuk API",
		})
	})

	// Get server port
	port := config.GetEnv("SERVER_PORT", "8081")

	log.Printf("Server starting on http://localhost:%s\n", port)
	log.Println("=================================")

	// Start server
	log.Fatal(router.Run(":" + port))
}

package router

import (
	"log"
	"os"
	"time"

	"github.com/Nero7991/devlm/internal/api/handlers"
	"github.com/Nero7991/devlm/internal/api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"golang.org/x/time/rate"
)

// SetupRouter initializes and configures the Gin router
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// Add middleware
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.Use(middleware.Authentication())
	r.Use(rateLimiter())
	r.Use(errorHandler())

	// Health check route
	r.GET("/health", handlers.HealthCheck)

	// API routes with versioning
	v1 := r.Group("/api/v1")
	{
		// Project routes
		projects := v1.Group("/projects")
		{
			projects.POST("", handlers.CreateProject)
			projects.GET("/:id", handlers.GetProject)
			projects.PUT("/:id", handlers.UpdateProject)
			projects.DELETE("/:id", handlers.DeleteProject)
			projects.GET("", handlers.ListProjects)
		}

		// Task routes
		tasks := v1.Group("/tasks")
		{
			tasks.POST("", handlers.CreateTask)
			tasks.GET("/:id", handlers.GetTask)
			tasks.PUT("/:id", handlers.UpdateTask)
			tasks.DELETE("/:id", handlers.DeleteTask)
			tasks.GET("", handlers.ListTasks)
		}

		// Code execution routes
		v1.POST("/execute", handlers.ExecuteCode)

		// File operation routes
		files := v1.Group("/files")
		{
			files.POST("", handlers.CreateFile)
			files.GET("/:id", handlers.GetFile)
			files.PUT("/:id", handlers.UpdateFile)
			files.DELETE("/:id", handlers.DeleteFile)
			files.GET("", handlers.ListFiles)
		}

		// Web search routes
		v1.POST("/search", handlers.PerformWebSearch)

		// LLM routes
		llm := v1.Group("/llm")
		{
			llm.POST("/analyze", handlers.AnalyzeRequirements)
			llm.POST("/generate", handlers.GenerateCode)
			llm.POST("/review", handlers.ReviewCode)
		}

		// Dev.txt file routes
		devtxt := v1.Group("/devtxt")
		{
			devtxt.POST("", handlers.UploadDevTxt)
			devtxt.GET("", handlers.GetDevTxt)
			devtxt.PUT("", handlers.UpdateDevTxt)
		}

		// Sandbox environment routes
		sandbox := v1.Group("/sandbox")
		{
			sandbox.POST("/create", handlers.CreateSandbox)
			sandbox.DELETE("/:id", handlers.DeleteSandbox)
			sandbox.POST("/:id/execute", handlers.ExecuteInSandbox)
		}

		// Documentation routes
		v1.GET("/docs", handlers.GetAPIDocumentation)
	}

	return r
}

// rateLimiter middleware implements rate limiting
func rateLimiter() gin.HandlerFunc {
	limit := viper.GetInt("RATE_LIMIT_PER_SECOND")
	if limit == 0 {
		limit = 100 // Default to 100 requests per second if not configured
	}
	limiter := rate.NewLimiter(rate.Every(time.Second), limit)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatus(429) // Too Many Requests
			return
		}
		c.Next()
	}
}

// errorHandler middleware handles and logs errors
func errorHandler() gin.HandlerFunc {
	// Create a logger
	logger := log.New(os.Stdout, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			// Log the errors
			for _, e := range c.Errors {
				logger.Println(e.Error())
			}

			// Return an error response
			c.JSON(c.Writer.Status(), gin.H{
				"errors": c.Errors.Errors(),
			})
		}
	}
}
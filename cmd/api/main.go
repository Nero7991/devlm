package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nero7991/devlm/internal/config"
	"github.com/Nero7991/devlm/internal/database"
	"github.com/Nero7991/devlm/internal/handler"
	"github.com/Nero7991/devlm/internal/service"
	"github.com/Nero7991/devlm/internal/executor"
	"github.com/Nero7991/devlm/internal/llm"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func main() {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.DatabaseDSN)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify database connection
	if err := db.Ping(); err != nil {
		logger.Fatalf("Failed to ping database: %v", err)
	}
	logger.Println("Successfully connected to database")

	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	defer rdb.Close()

	// Verify Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		logger.Printf("Warning: Failed to connect to Redis: %v", err)
		// Continue without Redis, as it's not critical for core functionality
	} else {
		logger.Println("Successfully connected to Redis")
	}

	// Initialize services
	dbService := database.NewService(db)
	cacheService := service.NewRedisService(rdb)
	llmClient := llm.NewClient(cfg.LLMServiceAddr)
	defer llmClient.Close()

	executorService := executor.NewService()
	apiService := service.NewAPIService(dbService, cacheService, llmClient, executorService)

	// Initialize handlers
	router := mux.NewRouter()
	handler.NewAPIHandler(router, apiService)

	// Add health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check database connection
		if err := db.Ping(); err != nil {
			logger.Printf("Database health check failed: %v", err)
			http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
			return
		}

		// Check Redis connection
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, err := rdb.Ping(ctx).Result(); err != nil {
			logger.Printf("Redis health check failed: %v", err)
			// Don't fail the health check for Redis, just log the error
		}

		// Check LLM service
		if err := llmClient.Ping(ctx); err != nil {
			logger.Printf("LLM service health check failed: %v", err)
			http.Error(w, "LLM service unavailable", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Create server
	srv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      router,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server
	go func() {
		logger.Printf("Starting server on %s", cfg.ServerAddr)
		maxRetries := 5
		for i := 0; i < maxRetries; i++ {
			err := srv.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				logger.Printf("Failed to start server (attempt %d/%d): %v", i+1, maxRetries, err)
				time.Sleep(time.Second * 5)
			} else {
				break
			}
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server after %d attempts: %v", maxRetries, err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Println("Shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal(fmt.Sprintf("Server forced to shutdown: %v", err))
	}

	logger.Println("Server exiting")
}
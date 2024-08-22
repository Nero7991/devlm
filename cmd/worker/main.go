package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Nero7991/devlm/internal/worker"
	"github.com/Nero7991/devlm/pkg/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	w, err := worker.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startWorker(ctx, w)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down worker...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	cancel()

	if err := gracefulShutdown(shutdownCtx, w); err != nil {
		log.Printf("Worker shutdown error: %v", err)
		if err := w.ForceShutdown(); err != nil {
			log.Printf("Forced shutdown error: %v", err)
		}
	} else {
		log.Println("Worker shutdown completed successfully")
	}

	wg.Wait()
	log.Println("Worker stopped")
}

func startWorker(ctx context.Context, w *worker.Worker) {
	const maxRetries = 3
	retryDelay := time.Second

	for i := 0; i < maxRetries; i++ {
		if err := w.Start(ctx); err != nil {
			if ctx.Err() != nil {
				log.Println("Worker stopped due to context cancellation")
				return
			}
			log.Printf("Worker stopped with error: %v", err)
			if i < maxRetries-1 {
				log.Printf("Retrying in %v...", retryDelay)
				select {
				case <-time.After(retryDelay):
					retryDelay *= 2 // Exponential backoff
				case <-ctx.Done():
					log.Println("Worker retry cancelled due to context cancellation")
					return
				}
			}
		} else {
			return
		}
	}
	log.Println("Max retries reached. Worker failed to start.")
}

func gracefulShutdown(ctx context.Context, w *worker.Worker) error {
	shutdownCh := make(chan error, 1)
	go func() {
		shutdownCh <- w.Shutdown()
	}()

	select {
	case err := <-shutdownCh:
		if err != nil {
			log.Printf("Graceful shutdown error: %v", err)
		}
		return err
	case <-ctx.Done():
		log.Printf("Graceful shutdown timed out: %v", ctx.Err())
		return ctx.Err()
	}
}
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/Nero7991/devlm/internal/cache"
	"github.com/Nero7991/devlm/internal/config"
	"github.com/Nero7991/devlm/internal/database"
)

type HealthHandler struct {
	startTime      time.Time
	config         *config.Config
	db             *database.Database
	cache          *cache.Cache
	isShuttingDown int32
}

func NewHealthHandler(cfg *config.Config, db *database.Database, cache *cache.Cache) (*HealthHandler, error) {
	if cfg == nil || db == nil || cache == nil {
		return nil, fmt.Errorf("invalid input: config, database, and cache must not be nil")
	}
	return &HealthHandler{
		startTime: time.Now(),
		config:    cfg,
		db:        db,
		cache:     cache,
	}, nil
}

func (h *HealthHandler) CheckHealth(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	health := map[string]interface{}{
		"status":     "healthy",
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"uptime":     time.Since(h.startTime).String(),
		"version":    h.config.Version,
		"go_version": runtime.Version(),
		"goroutines": runtime.NumGoroutine(),
		"memory": map[string]interface{}{
			"alloc":       ByteCountIEC(m.Alloc),
			"total_alloc": ByteCountIEC(m.TotalAlloc),
			"sys":         ByteCountIEC(m.Sys),
			"num_gc":      m.NumGC,
		},
	}

	h.addComponentChecks(health)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(health); err != nil {
		http.Error(w, "Failed to encode health check response", http.StatusInternalServerError)
		return
	}
}

func (h *HealthHandler) CheckReadiness(w http.ResponseWriter, r *http.Request) {
	ready := true
	status := http.StatusOK
	details := make(map[string]string)

	dbCtx, dbCancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer dbCancel()
	if err := h.db.PingContext(dbCtx); err != nil {
		ready = false
		details["database"] = fmt.Sprintf("unavailable: %v", err)
	} else {
		details["database"] = "available"
	}

	cacheCtx, cacheCancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cacheCancel()
	if err := h.cache.PingContext(cacheCtx); err != nil {
		ready = false
		details["cache"] = fmt.Sprintf("unavailable: %v", err)
	} else {
		details["cache"] = "available"
	}

	if !ready {
		status = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"ready":     ready,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"details":   details,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode readiness check response", http.StatusInternalServerError)
		return
	}
}

func (h *HealthHandler) CheckLiveness(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&h.isShuttingDown) == 1 {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *HealthHandler) addComponentChecks(health map[string]interface{}) {
	components := make(map[string]string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		components["database"] = fmt.Sprintf("error: %v", err)
	} else {
		components["database"] = "OK"
	}

	if err := h.cache.PingContext(ctx); err != nil {
		components["cache"] = fmt.Sprintf("error: %v", err)
	} else {
		components["cache"] = "OK"
	}

	health["components"] = components
}

func ByteCountIEC(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (h *HealthHandler) StartShutdown() {
	atomic.StoreInt32(&h.isShuttingDown, 1)
}

func (h *HealthHandler) IsShuttingDown() bool {
	return atomic.LoadInt32(&h.isShuttingDown) == 1
}
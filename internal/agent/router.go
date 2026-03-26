package agent

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/ya5u/goemon/internal/llm"
)

type Router struct {
	backends        map[string]llm.Backend
	defaultBackend  string
	fallbackBackend string
	forceCloudFor   []string
	healthInterval  time.Duration

	mu     sync.RWMutex
	status map[string]bool

	stopCh chan struct{}
}

type RouterConfig struct {
	Default              string
	Fallback             string
	ForceCloudFor        []string
	HealthCheckIntervalS int
}

func NewRouter(cfg RouterConfig, backends map[string]llm.Backend) *Router {
	interval := time.Duration(cfg.HealthCheckIntervalS) * time.Second
	if interval == 0 {
		interval = 30 * time.Second
	}
	r := &Router{
		backends:        backends,
		defaultBackend:  cfg.Default,
		fallbackBackend: cfg.Fallback,
		forceCloudFor:   cfg.ForceCloudFor,
		healthInterval:  interval,
		status:          make(map[string]bool),
		stopCh:          make(chan struct{}),
	}
	return r
}

func (r *Router) Start(ctx context.Context) {
	// Initial health check
	r.checkAll(ctx)

	go func() {
		ticker := time.NewTicker(r.healthInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.checkAll(ctx)
			case <-r.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (r *Router) Stop() {
	close(r.stopCh)
}

func (r *Router) checkAll(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for name, b := range r.backends {
		available := b.IsAvailable(checkCtx)
		r.mu.Lock()
		r.status[name] = available
		r.mu.Unlock()
		slog.Debug("health check", "backend", name, "available", available)
	}
}

func (r *Router) Select(taskType string) (llm.Backend, error) {
	// Check if task type forces cloud
	if slices.Contains(r.forceCloudFor, taskType) {
		if b, ok := r.backends[r.fallbackBackend]; ok {
			r.mu.RLock()
			available := r.status[r.fallbackBackend]
			r.mu.RUnlock()
			if available {
				slog.Info("routing to cloud (forced)", "backend", r.fallbackBackend, "task", taskType)
				return b, nil
			}
		}
	}

	// Try default
	if b, ok := r.backends[r.defaultBackend]; ok {
		r.mu.RLock()
		available := r.status[r.defaultBackend]
		r.mu.RUnlock()
		if available {
			return b, nil
		}
	}

	// Try fallback
	if b, ok := r.backends[r.fallbackBackend]; ok {
		r.mu.RLock()
		available := r.status[r.fallbackBackend]
		r.mu.RUnlock()
		if available {
			slog.Info("routing to fallback", "backend", r.fallbackBackend)
			return b, nil
		}
	}

	// Try any available
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, available := range r.status {
		if available {
			slog.Info("routing to any available", "backend", name)
			return r.backends[name], nil
		}
	}

	return nil, fmt.Errorf("no LLM backend available")
}

func (r *Router) CurrentBackendName() string {
	b, err := r.Select("")
	if err != nil {
		return "none"
	}
	return b.Name()
}

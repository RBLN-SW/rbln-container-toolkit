/*
Copyright 2026 Rebellions Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// HealthResponse is the response structure for health endpoints.
type HealthResponse struct {
	Status    string                 `json:"status"`
	Reason    string                 `json:"reason,omitempty"`
	Checks    map[string]CheckResult `json:"checks,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// CheckResult holds the result of an individual health check.
type CheckResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// HealthServer provides HTTP health check endpoints.
type HealthServer struct {
	port   int
	server *http.Server

	mu      sync.RWMutex
	started bool
	ready   bool
	checks  map[string]CheckResult
}

// NewHealthServer creates a new health server.
func NewHealthServer(port int) *HealthServer {
	return &HealthServer{
		port:   port,
		checks: make(map[string]CheckResult),
	}
}

// Start starts the health server.
func (hs *HealthServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/live", hs.liveHandler)
	mux.HandleFunc("/ready", hs.readyHandler)
	mux.HandleFunc("/startup", hs.startupHandler)

	addr := fmt.Sprintf(":%d", hs.port)
	hs.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for context cancellation
	select {
	case <-ctx.Done():
		// Graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return hs.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// Stop stops the health server.
func (hs *HealthServer) Stop() error {
	if hs.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return hs.server.Shutdown(ctx)
}

// SetStarted sets the startup state.
func (hs *HealthServer) SetStarted(started bool) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.started = started
}

// IsStarted returns the startup state.
func (hs *HealthServer) IsStarted() bool {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return hs.started
}

// SetReady sets the readiness state.
func (hs *HealthServer) SetReady(ready bool) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.ready = ready
}

// IsReady returns the readiness state.
func (hs *HealthServer) IsReady() bool {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return hs.ready
}

// AddCheck adds a health check result.
func (hs *HealthServer) AddCheck(name string, result CheckResult) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.checks[name] = result
}

// RemoveCheck removes a health check.
func (hs *HealthServer) RemoveCheck(name string) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	delete(hs.checks, name)
}

// liveHandler handles /live requests.
func (hs *HealthServer) liveHandler(w http.ResponseWriter, _ *http.Request) {
	resp := HealthResponse{
		Status:    "alive",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	hs.writeJSON(w, http.StatusOK, resp)
}

// readyHandler handles /ready requests.
func (hs *HealthServer) readyHandler(w http.ResponseWriter, _ *http.Request) {
	hs.mu.RLock()
	ready := hs.ready
	checks := make(map[string]CheckResult)
	for k, v := range hs.checks {
		checks[k] = v
	}
	hs.mu.RUnlock()

	var resp HealthResponse
	var status int

	if ready {
		resp = HealthResponse{
			Status:    "ready",
			Checks:    checks,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		status = http.StatusOK
	} else {
		resp = HealthResponse{
			Status:    "not_ready",
			Reason:    "Setup in progress",
			Checks:    checks,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		status = http.StatusServiceUnavailable
	}

	hs.writeJSON(w, status, resp)
}

// startupHandler handles /startup requests.
func (hs *HealthServer) startupHandler(w http.ResponseWriter, _ *http.Request) {
	hs.mu.RLock()
	started := hs.started
	hs.mu.RUnlock()

	var resp HealthResponse
	var status int

	if started {
		resp = HealthResponse{
			Status:    "started",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		status = http.StatusOK
	} else {
		resp = HealthResponse{
			Status:    "starting",
			Reason:    "Initialization in progress",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		status = http.StatusServiceUnavailable
	}

	hs.writeJSON(w, status, resp)
}

// writeJSON writes a JSON response.
func (hs *HealthServer) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

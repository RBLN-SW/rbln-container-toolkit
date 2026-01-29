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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthServer_LiveEndpoint(t *testing.T) {
	// Given
	hs := NewHealthServer(8080)
	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	w := httptest.NewRecorder()

	// When
	hs.liveHandler(w, req)

	// Then
	assert.Equal(t, http.StatusOK, w.Code)
	var resp HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "alive", resp.Status)
	assert.NotEmpty(t, resp.Timestamp)
}

func TestHealthServer_ReadyEndpoint_Ready(t *testing.T) {
	// Given
	hs := NewHealthServer(8080)
	hs.SetReady(true)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	// When
	hs.readyHandler(w, req)

	// Then
	assert.Equal(t, http.StatusOK, w.Code)
	var resp HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ready", resp.Status)
}

func TestHealthServer_ReadyEndpoint_NotReady(t *testing.T) {
	// Given
	hs := NewHealthServer(8080)
	hs.SetReady(false)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	// When
	hs.readyHandler(w, req)

	// Then
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "not_ready", resp.Status)
	assert.NotEmpty(t, resp.Reason)
}

func TestHealthServer_StartupEndpoint_Started(t *testing.T) {
	// Given
	hs := NewHealthServer(8080)
	hs.SetStarted(true)
	req := httptest.NewRequest(http.MethodGet, "/startup", nil)
	w := httptest.NewRecorder()

	// When
	hs.startupHandler(w, req)

	// Then
	assert.Equal(t, http.StatusOK, w.Code)
	var resp HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "started", resp.Status)
}

func TestHealthServer_StartupEndpoint_NotStarted(t *testing.T) {
	// Given
	hs := NewHealthServer(8080)
	hs.SetStarted(false)
	req := httptest.NewRequest(http.MethodGet, "/startup", nil)
	w := httptest.NewRecorder()

	// When
	hs.startupHandler(w, req)

	// Then
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "starting", resp.Status)
}

func TestHealthServer_ReadyWithChecks(t *testing.T) {
	// Given
	hs := NewHealthServer(8080)
	hs.SetReady(true)
	hs.AddCheck("cdi_spec", CheckResult{
		Status:  "ready",
		Message: "CDI spec exists",
	})
	hs.AddCheck("runtime_config", CheckResult{
		Status:  "ready",
		Message: "Runtime configured",
	})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	// When
	hs.readyHandler(w, req)

	// Then
	assert.Equal(t, http.StatusOK, w.Code)
	var resp HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ready", resp.Status)
	assert.Contains(t, resp.Checks, "cdi_spec")
	assert.Contains(t, resp.Checks, "runtime_config")
	assert.Equal(t, "ready", resp.Checks["cdi_spec"].Status)
}

func TestHealthServer_StartStop(t *testing.T) {
	// Given
	hs := NewHealthServer(0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- hs.Start(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// When
	cancel()

	// Then
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop in time")
	}
}

func TestHealthServer_IntegrationWithDaemon(t *testing.T) {
	// Given
	hs := NewHealthServer(0)

	// When
	assert.False(t, hs.IsStarted())
	assert.False(t, hs.IsReady())
	hs.SetStarted(true)

	// Then
	assert.True(t, hs.IsStarted())
	assert.False(t, hs.IsReady())
	hs.SetReady(true)
	assert.True(t, hs.IsStarted())
	assert.True(t, hs.IsReady())
	hs.SetReady(false)
	assert.True(t, hs.IsStarted())
	assert.False(t, hs.IsReady())
}

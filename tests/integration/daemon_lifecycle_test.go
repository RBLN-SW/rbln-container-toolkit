//go:build integration

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

package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemonLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given
	binaryPath := buildDaemonBinary(t)
	tmpDir := t.TempDir()
	hostRoot := filepath.Join(tmpDir, "host")
	cdiDir := filepath.Join(hostRoot, "var", "run", "cdi")
	configDir := filepath.Join(hostRoot, "etc", "containerd")
	pidFile := filepath.Join(tmpDir, "daemon.pid")
	healthPort := "18080"

	require.NoError(t, os.MkdirAll(cdiDir, 0755))
	require.NoError(t, os.MkdirAll(configDir, 0755))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath,
		"--runtime", "containerd",
		"--restart-mode", "none",
		"--host-root-mount", hostRoot,
		"--cdi-spec-dir", "/var/run/cdi",
		"--pid-file", pidFile,
		"--health-port", healthPort,
	)
	cmd.Env = append(os.Environ(), "RBLN_CTK_DAEMON_DEBUG=true")

	require.NoError(t, cmd.Start())
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_ = cmd.Wait()
		}
	}()

	healthURL := fmt.Sprintf("http://localhost:%s", healthPort)

	// When
	waitForHealth(t, healthURL+"/startup", 10*time.Second)
	resp, err := http.Get(healthURL + "/live")
	resp2, err2 := http.Get(healthURL + "/ready")
	specPath := filepath.Join(cdiDir, "rbln.yaml")

	// Then
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	require.NoError(t, err2)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	resp2.Body.Close()

	assert.FileExists(t, specPath)

	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("Daemon did not shutdown within timeout")
	}
}

func TestDaemonHealthEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given
	binaryPath := buildDaemonBinary(t)
	tmpDir := t.TempDir()
	hostRoot := filepath.Join(tmpDir, "host")
	cdiDir := filepath.Join(hostRoot, "var", "run", "cdi")
	configDir := filepath.Join(hostRoot, "etc", "containerd")
	pidFile := filepath.Join(tmpDir, "daemon.pid")
	healthPort := "18081"

	require.NoError(t, os.MkdirAll(cdiDir, 0755))
	require.NoError(t, os.MkdirAll(configDir, 0755))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath,
		"--runtime", "containerd",
		"--restart-mode", "none",
		"--host-root-mount", hostRoot,
		"--cdi-spec-dir", "/var/run/cdi",
		"--pid-file", pidFile,
		"--health-port", healthPort,
	)

	require.NoError(t, cmd.Start())
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_ = cmd.Wait()
		}
	}()

	healthURL := fmt.Sprintf("http://localhost:%s", healthPort)
	waitForHealth(t, healthURL+"/startup", 10*time.Second)

	// When
	endpoints := []string{"/live", "/ready", "/startup"}
	responses := make([]*http.Response, 0)
	errs := make([]error, 0, len(endpoints))
	for _, endpoint := range endpoints {
		resp, err := http.Get(healthURL + endpoint)
		responses = append(responses, resp)
		errs = append(errs, err)
	}

	// Then
	for i, endpoint := range endpoints {
		require.NoError(t, errs[i], "Failed to GET %s", endpoint)
		assert.Equal(t, http.StatusOK, responses[i].StatusCode, "Endpoint %s returned %d", endpoint, responses[i].StatusCode)
		responses[i].Body.Close()
	}

	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
	_ = cmd.Wait()
}

func TestDaemonDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given
	binaryPath := buildDaemonBinary(t)
	tmpDir := t.TempDir()

	// When
	cmd := exec.Command(binaryPath,
		"--runtime", "containerd",
		"--dry-run",
		"--pid-file", filepath.Join(tmpDir, "daemon.pid"),
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.NoError(t, err, "Dry-run failed: %s", string(output))
	assert.Contains(t, string(output), "[DRY-RUN]")
}

func buildDaemonBinary(t *testing.T) string {
	t.Helper()

	projectRoot := filepath.Join("..", "..")
	binaryPath := filepath.Join(projectRoot, "bin", "rbln-ctk-daemon-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/rbln-ctk-daemon")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build daemon binary: %s", string(output))

	return binaryPath
}

func waitForHealth(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("Health endpoint %s did not become ready within %v", url, timeout)
}

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

// Package restart provides runtime restart functionality.
package restart

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// SystemdRestarter restarts runtimes using systemctl.
type SystemdRestarter struct {
	hostRootMount string
	timeout       time.Duration
}

// newSystemdRestarter creates a new SystemdRestarter.
func newSystemdRestarter(opts Options) *SystemdRestarter {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &SystemdRestarter{
		hostRootMount: opts.HostRootMount,
		timeout:       timeout,
	}
}

// Restart restarts the runtime using systemctl.
func (r *SystemdRestarter) Restart(runtime string) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	var cmd *exec.Cmd
	if r.hostRootMount != "" {
		// Use chroot for containerized deployment
		cmd = exec.CommandContext(ctx, "chroot", r.hostRootMount, "systemctl", "restart", runtime)
	} else {
		cmd = exec.CommandContext(ctx, "systemctl", "restart", runtime)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("restart %s timed out after %v", runtime, r.timeout)
		}
		return fmt.Errorf("systemctl restart %s failed: %w\nOutput: %s", runtime, err, string(output))
	}

	return nil
}

// DryRun returns a description of what would happen.
func (r *SystemdRestarter) DryRun(runtime string) string {
	if r.hostRootMount != "" {
		return fmt.Sprintf("Would run: chroot %s systemctl restart %s", r.hostRootMount, runtime)
	}
	return fmt.Sprintf("Would run: systemctl restart %s", runtime)
}

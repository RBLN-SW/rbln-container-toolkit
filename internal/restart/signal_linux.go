//go:build linux

/*
Copyright (c) 2026 Rebellions Inc.

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
	"fmt"
	"net"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// SignalRestarter restarts runtimes by sending SIGHUP via Unix socket.
type SignalRestarter struct {
	socket     string
	maxRetries int
	backoff    time.Duration
}

// newSignalRestarter creates a new SignalRestarter.
func newSignalRestarter(opts Options) (*SignalRestarter, error) {
	if opts.Socket == "" {
		return nil, fmt.Errorf("socket path is required for signal restart mode")
	}

	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	backoff := opts.RetryBackoff
	if backoff <= 0 {
		backoff = 5 * time.Second
	}

	return &SignalRestarter{
		socket:     opts.Socket,
		maxRetries: maxRetries,
		backoff:    backoff,
	}, nil
}

// Restart sends SIGHUP to the runtime process.
func (r *SignalRestarter) Restart(runtime string) error {
	var lastErr error

	for i := 0; i < r.maxRetries; i++ {
		if i > 0 {
			time.Sleep(r.backoff)
		}

		pid, err := r.getPID(runtime)
		if err != nil {
			lastErr = fmt.Errorf("failed to get PID: %w", err)
			continue
		}

		if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
			lastErr = fmt.Errorf("failed to send SIGHUP to PID %d: %w", pid, err)
			continue
		}

		return nil
	}

	return fmt.Errorf("failed to restart %s after %d retries: %w", runtime, r.maxRetries, lastErr)
}

// DryRun returns a description of what would happen.
func (r *SignalRestarter) DryRun(runtime string) string {
	return fmt.Sprintf("Would send SIGHUP to %s via socket %s", runtime, r.socket)
}

// getPID discovers the runtime process PID via Unix socket credentials.
func (r *SignalRestarter) getPID(runtime string) (int, error) {
	conn, err := net.Dial("unix", r.socket)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to socket %s: %w", r.socket, err)
	}
	defer conn.Close()

	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("connection is not a Unix socket")
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("failed to get raw connection: %w", err)
	}

	var setsockoptErr error
	err = rawConn.Control(func(fd uintptr) {
		setsockoptErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1)
	})
	if err != nil {
		return 0, fmt.Errorf("failed to control socket: %w", err)
	}
	if setsockoptErr != nil {
		return 0, fmt.Errorf("failed to set SO_PASSCRED: %w", setsockoptErr)
	}

	// Send message based on runtime type
	// Docker requires HTTP request, containerd accepts empty message
	var msg string
	if runtime == "docker" {
		msg = "GET /info HTTP/1.0\r\n\r\n"
	}

	_, _, err = unixConn.WriteMsgUnix([]byte(msg), nil, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to write to socket: %w", err)
	}

	// Wait for response data (socket is non-blocking by default in Go)
	var pollErr error
	err = rawConn.Control(func(fd uintptr) {
		pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, perr := unix.Poll(pfd, 5000)
		if perr != nil {
			pollErr = fmt.Errorf("poll failed: %w", perr)
			return
		}
		if n == 0 {
			pollErr = fmt.Errorf("timeout waiting for response from socket")
			return
		}
	})
	if err != nil {
		return 0, fmt.Errorf("failed to poll socket: %w", err)
	}
	if pollErr != nil {
		return 0, pollErr
	}

	// Read response with credentials
	oob := make([]byte, 1024)
	_, oobn, _, _, err := unixConn.ReadMsgUnix(nil, oob)
	if err != nil {
		return 0, fmt.Errorf("failed to read from socket: %w", err)
	}

	if oobn == 0 {
		return 0, fmt.Errorf("no credential data received")
	}

	oob = oob[:oobn]
	scm, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return 0, fmt.Errorf("failed to parse control message: %w", err)
	}

	if len(scm) == 0 {
		return 0, fmt.Errorf("no control messages received")
	}

	ucred, err := syscall.ParseUnixCredentials(&scm[0])
	if err != nil {
		return 0, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return int(ucred.Pid), nil
}

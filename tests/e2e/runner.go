/*
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 Rebellions Inc.
*/

package e2e

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"text/template"
)

// Runner defines the interface for executing scripts in different environments.
type Runner interface {
	Run(script string) (stdout, stderr string, err error)
}

// localRunner executes scripts on the local host.
type localRunner struct{}

// nestedContainerRunner executes scripts inside a Docker container.
type nestedContainerRunner struct {
	runner        Runner
	containerName string
}

// NewLocalRunner creates a new local runner.
func NewLocalRunner() Runner {
	return &localRunner{}
}

// Run executes the script locally using bash.
func (l *localRunner) Run(script string) (string, string, error) {
	cmd := exec.Command("bash", "-c", script)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.String(), stderr.String(),
			fmt.Errorf("script execution failed: %v\nSTDOUT: %s\nSTDERR: %s",
				err, stdout.String(), stderr.String())
	}

	return stdout.String(), stderr.String(), nil
}

// NewNestedContainerRunner creates a runner that executes inside a Docker container.
func NewNestedContainerRunner(runner Runner, baseImage, containerName string) (Runner, error) {
	// Remove existing container if present
	_, _, _ = runner.Run(fmt.Sprintf("docker rm -f %s 2>/dev/null || true", containerName))

	// Start container template
	container := outerContainer{
		Name:      containerName,
		BaseImage: baseImage,
	}

	script, err := container.Render()
	if err != nil {
		return nil, fmt.Errorf("failed to render container start script: %w", err)
	}

	_, stderr, err := runner.Run(script)
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w, stderr: %s", err, stderr)
	}

	inContainer := &nestedContainerRunner{
		runner:        runner,
		containerName: containerName,
	}

	// Install prerequisites
	prereqScript := `
set -e
export DEBIAN_FRONTEND=noninteractive
if command -v apt-get &> /dev/null; then
    apt-get update -qq
    apt-get install -y -qq curl ca-certificates > /dev/null
elif command -v dnf &> /dev/null; then
    dnf install -y -q curl ca-certificates > /dev/null
elif command -v yum &> /dev/null; then
    yum install -y -q curl ca-certificates > /dev/null
fi
`
	_, stderr, err = inContainer.Run(prereqScript)
	if err != nil {
		return nil, fmt.Errorf("failed to install prerequisites: %w, stderr: %s", err, stderr)
	}

	return inContainer, nil
}

// Run executes the script inside the container.
func (r *nestedContainerRunner) Run(script string) (string, string, error) {
	// Escape single quotes in the script
	escapedScript := strings.ReplaceAll(script, "'", "'\"'\"'")
	dockerCmd := fmt.Sprintf(`docker exec -u root "%s" bash -c '%s'`, r.containerName, escapedScript)
	return r.runner.Run(dockerCmd)
}

// Cleanup removes the container.
func (r *nestedContainerRunner) Cleanup() error {
	_, _, err := r.runner.Run(fmt.Sprintf("docker rm -f %s 2>/dev/null || true", r.containerName))
	return err
}

// outerContainer represents a container configuration.
type outerContainer struct {
	Name      string
	BaseImage string
}

// Render generates the docker run command.
func (o *outerContainer) Render() (string, error) {
	tmpl, err := template.New("startContainer").Parse(`docker run -d --name {{.Name}} --privileged {{.BaseImage}} sleep infinity`)
	if err != nil {
		return "", err
	}

	var script strings.Builder
	if err := tmpl.Execute(&script, o); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return script.String(), nil
}

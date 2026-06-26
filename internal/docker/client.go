// Package docker implements the domain.DockerEngine port using the Docker SDK.
// Each Engine instance wraps a single Docker client bound to one Docker host.
package docker

import (
	"context"
	"fmt"

	"github.com/Nolanndev/dev-natif/internal/domain"
	"github.com/docker/docker/client"
)

// Engine is the concrete implementation of domain.DockerEngine.
type Engine struct {
	cli *client.Client
}

// New creates an Engine connected to the given Docker host.
// If host is empty the client is built from environment variables and defaults.
func New(host string) (*Engine, error) {
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker client init: %w", err)
	}

	return &Engine{cli: cli}, nil
}

// Ping checks connectivity with the Docker Engine.
func (e *Engine) Ping(ctx context.Context) error {
	if _, err := e.cli.Ping(ctx); err != nil {
		return fmt.Errorf("ping: %w: %w", err, domain.ErrDockerEngine)
	}
	return nil
}

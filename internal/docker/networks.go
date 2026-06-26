package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/Nolanndev/dev-natif/internal/domain"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
)

// EnsureNetwork creates a user-defined bridge network if it does not already
// exist. Containers attached to the same network resolve each other by alias.
func (e *Engine) EnsureNetwork(ctx context.Context, name string, labels map[string]string) error {
	_, err := e.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: "bridge",
		Labels: labels,
	})
	if err != nil {
		// Idempotent: ignore "already exists".
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return nil
		}
		return fmt.Errorf("network create %q: %w: %w", name, err, domain.ErrDockerEngine)
	}
	return nil
}

// RemoveNetwork removes a network by name or ID. Missing networks are ignored.
func (e *Engine) RemoveNetwork(ctx context.Context, nameOrID string) error {
	if err := e.cli.NetworkRemove(ctx, nameOrID); err != nil {
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "not found") || strings.Contains(low, "no such") {
			return nil
		}
		return fmt.Errorf("network remove %q: %w: %w", nameOrID, err, domain.ErrDockerEngine)
	}
	return nil
}

// ListNetworksByLabel returns the IDs of networks carrying all given labels.
func (e *Engine) ListNetworksByLabel(ctx context.Context, labels map[string]string) ([]string, error) {
	f := filters.NewArgs()
	for k, v := range labels {
		f.Add("label", k+"="+v)
	}
	nets, err := e.cli.NetworkList(ctx, network.ListOptions{Filters: f})
	if err != nil {
		return nil, fmt.Errorf("network list: %w: %w", err, domain.ErrDockerEngine)
	}
	ids := make([]string, 0, len(nets))
	for _, n := range nets {
		ids = append(ids, n.ID)
	}
	return ids, nil
}

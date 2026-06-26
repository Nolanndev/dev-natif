package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/actigraph/dev-natif/internal/domain"
	"github.com/docker/docker/api/types/volume"
)

// CreateVolume creates a named Docker volume. If the volume already exists the
// call is a no-op (idempotent).
func (e *Engine) CreateVolume(ctx context.Context, name string, labels map[string]string) error {
	_, err := e.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name:   name,
		Labels: labels,
	})
	if err != nil {
		// "already exists" is not an error — treat it as success.
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return fmt.Errorf("volume create %q: %w: %w", name, err, domain.ErrDockerEngine)
	}
	return nil
}

// RemoveVolume removes a named Docker volume.
func (e *Engine) RemoveVolume(ctx context.Context, name string, force bool) error {
	if err := e.cli.VolumeRemove(ctx, name, force); err != nil {
		return fmt.Errorf("volume remove %q: %w: %w", name, err, domain.ErrDockerEngine)
	}
	return nil
}

package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/actigraph/dev-natif/internal/domain"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/archive"
)

// PullImage pulls an image from a registry. The response stream is drained so
// the pull completes before returning.
func (e *Engine) PullImage(ctx context.Context, spec domain.ImagePullSpec) error {
	rc, err := e.cli.ImagePull(ctx, spec.Ref, image.PullOptions{
		RegistryAuth: spec.AuthB64,
	})
	if err != nil {
		return fmt.Errorf("image pull %q: %w: %w", spec.Ref, err, domain.ErrDockerEngine)
	}
	defer rc.Close()

	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("image pull drain %q: %w: %w", spec.Ref, err, domain.ErrDockerEngine)
	}
	return nil
}

// BuildImage builds an image from a local context directory.
func (e *Engine) BuildImage(ctx context.Context, spec domain.ImageBuildSpec) error {
	tar, err := archive.TarWithOptions(spec.ContextDir, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("image build tar %q: %w: %w", spec.ContextDir, err, domain.ErrDockerEngine)
	}

	dockerfile := spec.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	resp, err := e.cli.ImageBuild(ctx, tar, types.ImageBuildOptions{
		Dockerfile: dockerfile,
		Tags:       []string{spec.Tag},
		Remove:     true,
	})
	if err != nil {
		return fmt.Errorf("image build %q: %w: %w", spec.Tag, err, domain.ErrDockerEngine)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("image build drain %q: %w: %w", spec.Tag, err, domain.ErrDockerEngine)
	}
	return nil
}

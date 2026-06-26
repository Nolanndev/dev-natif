package docker

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Nolanndev/dev-natif/internal/domain"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

// CreateContainer builds a container from spec and returns its Docker ID.
func (e *Engine) CreateContainer(ctx context.Context, spec domain.ContainerSpec) (string, error) {
	exposedPorts, portBindings, err := buildPortMappings(spec.Ports)
	if err != nil {
		return "", fmt.Errorf("container port mapping: %w: %w", err, domain.ErrDockerEngine)
	}

	cfg := &container.Config{
		Image:        spec.Image,
		Env:          spec.Env,
		Cmd:          strslice.StrSlice(spec.Cmd),
		Labels:       spec.Labels,
		ExposedPorts: exposedPorts,
	}

	hostCfg := &container.HostConfig{
		PortBindings: portBindings,
		Mounts:       buildMounts(spec.Mounts),
	}
	if spec.RestartPolicy != "" {
		hostCfg.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(spec.RestartPolicy),
		}
	}

	// Attach to a user-defined network with DNS aliases when requested, so
	// services and replicas resolve each other by name (docker-compose style).
	var netCfg *network.NetworkingConfig
	if spec.Network != "" {
		netCfg = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				spec.Network: {Aliases: spec.Aliases},
			},
		}
	}

	resp, err := e.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, spec.Name)
	if err != nil {
		return "", fmt.Errorf("container create %q: %w: %w", spec.Name, err, domain.ErrDockerEngine)
	}
	return resp.ID, nil
}

// StartContainer starts a stopped or newly-created container by ID.
func (e *Engine) StartContainer(ctx context.Context, id string) error {
	if err := e.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start %q: %w: %w", id, err, domain.ErrDockerEngine)
	}
	return nil
}

// StopContainer sends a SIGTERM to the container and waits for it to stop.
func (e *Engine) StopContainer(ctx context.Context, id string) error {
	if err := e.cli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return fmt.Errorf("container stop %q: %w: %w", id, err, domain.ErrDockerEngine)
	}
	return nil
}

// RemoveContainer removes a container, optionally forcing removal if running.
func (e *Engine) RemoveContainer(ctx context.Context, id string, force bool) error {
	err := e.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
	if err != nil {
		if client.IsErrNotFound(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("container remove %q: %w: %w", id, err, domain.ErrDockerEngine)
	}
	return nil
}

// InspectContainer returns the live state of one container.
func (e *Engine) InspectContainer(ctx context.Context, id string) (domain.ContainerInfo, error) {
	j, err := e.cli.ContainerInspect(ctx, id)
	if err != nil {
		if client.IsErrNotFound(err) {
			return domain.ContainerInfo{}, domain.ErrNotFound
		}
		return domain.ContainerInfo{}, fmt.Errorf("container inspect %q: %w: %w", id, err, domain.ErrDockerEngine)
	}

	health := "none"
	if j.State != nil && j.State.Health != nil {
		health = j.State.Health.Status
	}

	state := ""
	if j.State != nil {
		state = j.State.Status
	}

	var labels map[string]string
	if j.Config != nil {
		labels = j.Config.Labels
	}

	return domain.ContainerInfo{
		ID:     j.ID,
		Name:   strings.TrimPrefix(j.Name, "/"),
		Image:  j.Image,
		State:  state,
		Health: health,
		Labels: labels,
	}, nil
}

// ContainerLogs returns the last `tail` lines of a container's combined
// stdout+stderr. The engine stream is multiplexed, so it is demuxed via stdcopy.
func (e *Engine) ContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	tailStr := "all"
	if tail > 0 {
		tailStr = strconv.Itoa(tail)
	}
	rc, err := e.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       tailStr,
	})
	if err != nil {
		if client.IsErrNotFound(err) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("container logs %q: %w: %w", id, err, domain.ErrDockerEngine)
	}
	defer rc.Close()

	var out bytes.Buffer
	if _, err := stdcopy.StdCopy(&out, &out, rc); err != nil {
		return "", fmt.Errorf("container logs demux %q: %w: %w", id, err, domain.ErrDockerEngine)
	}
	return out.String(), nil
}

// ListContainersByLabel returns all containers (running or stopped) that carry
// all of the supplied labels.
func (e *Engine) ListContainersByLabel(ctx context.Context, labels map[string]string) ([]domain.ContainerInfo, error) {
	f := filters.NewArgs()
	for k, v := range labels {
		f.Add("label", k+"="+v)
	}

	list, err := e.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return nil, fmt.Errorf("container list: %w: %w", err, domain.ErrDockerEngine)
	}

	infos := make([]domain.ContainerInfo, 0, len(list))
	for _, c := range list {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		infos = append(infos, domain.ContainerInfo{
			ID:     c.ID,
			Name:   name,
			Image:  c.Image,
			State:  c.State,
			Health: "none",
			Labels: c.Labels,
		})
	}
	return infos, nil
}

// buildPortMappings converts domain PortBinding slice into Docker nat types.
func buildPortMappings(ports []domain.PortBinding) (nat.PortSet, nat.PortMap, error) {
	exposed := make(nat.PortSet, len(ports))
	bindings := make(nat.PortMap, len(ports))

	for _, pb := range ports {
		proto := pb.Protocol
		if proto == "" {
			proto = "tcp"
		}
		p, err := nat.NewPort(proto, strconv.Itoa(pb.ContainerPort))
		if err != nil {
			return nil, nil, err
		}
		exposed[p] = struct{}{}

		hostPort := ""
		if pb.HostPort != 0 {
			hostPort = strconv.Itoa(pb.HostPort)
		}
		bindings[p] = append(bindings[p], nat.PortBinding{HostPort: hostPort})
	}
	return exposed, bindings, nil
}

// buildMounts converts domain Mount slice into Docker mount.Mount slice.
func buildMounts(mounts []domain.Mount) []mount.Mount {
	ms := make([]mount.Mount, 0, len(mounts))
	for _, m := range mounts {
		ms = append(ms, mount.Mount{
			Type:     mount.TypeVolume,
			Source:   m.VolumeName,
			Target:   m.Target,
			ReadOnly: m.ReadOnly,
		})
	}
	return ms
}

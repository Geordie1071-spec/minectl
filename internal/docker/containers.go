package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/minectl/minectl/internal/config"
	"github.com/minectl/minectl/internal/domain"
)

// CreateMinecraftContainer creates a container for the given server using itzg/minecraft-server
func (c *Client) CreateMinecraftContainer(ctx context.Context, s *domain.Server, env []string) (string, error) {
	containerName := config.ContainerNamePrefix + s.Name

	portMap := nat.PortMap{
		nat.Port("25565/tcp"): []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", s.Port)}},
	}
	exposedPorts := nat.PortSet{
		nat.Port("25565/tcp"): {},
	}

	tag := s.ImageTag
	if tag == "" {
		tag = config.ImageTagForMCVersion(s.MCVersion, s.MCType)
	}
	if tag == "" {
		tag = "latest"
	}
	imageRef := config.MinecraftImage + ":" + tag
	cfg := &container.Config{
		Image:        imageRef,
		Env:          env,
		ExposedPorts: exposedPorts,
	}

	hostCfg := &container.HostConfig{
		PortBindings: portMap,
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Resources: container.Resources{
			Memory:   int64(s.MemoryMB) * 1024 * 1024,
			NanoCPUs: int64(s.CPUCores * 1e9),
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: s.DataDir,
				Target: "/data",
			},
		},
	}

	resp, err := c.cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, containerName)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// StartContainer starts a container by ID
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

// StopContainer stops a container with optional timeout (seconds). 0 = default.
func (c *Client) StopContainer(ctx context.Context, containerID string, timeoutSec *int) error {
	opts := container.StopOptions{}
	if timeoutSec != nil && *timeoutSec > 0 {
		opts.Timeout = timeoutSec
	}
	return c.cli.ContainerStop(ctx, containerID, opts)
}

// RemoveContainer removes a container (force if running)
func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
}

// InspectContainer returns container inspect info
func (c *Client) InspectContainer(ctx context.Context, containerID string) (interface{}, error) {
	return c.cli.ContainerInspect(ctx, containerID)
}

// ContainerByName finds a container ID by name (minectl-<name>)
func (c *Client) ContainerByName(ctx context.Context, name string) (string, error) {
	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return "", err
	}
	prefix := config.ContainerNamePrefix + name
	for _, cn := range containers {
		for _, n := range cn.Names {
			if strings.TrimPrefix(n, "/") == prefix {
				return cn.ID, nil
			}
		}
	}
	return "", fmt.Errorf("container not found: %s", name)
}

// ListContainers returns all minectl containers
func (c *Client) ListContainers(ctx context.Context) ([]string, error) {
	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, cn := range containers {
		for _, n := range cn.Names {
			if strings.HasPrefix(strings.TrimPrefix(n, "/"), config.ContainerNamePrefix) {
				ids = append(ids, cn.ID)
				break
			}
		}
	}
	return ids, nil
}

// ContainerRunning returns true if the container is running
func (c *Client) ContainerRunning(ctx context.Context, containerID string) (bool, error) {
	inspect, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}
	return inspect.State.Running, nil
}

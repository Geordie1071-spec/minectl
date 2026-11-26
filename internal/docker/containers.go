package docker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
)

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
		Env:          withHostUser(env),
		ExposedPorts: exposedPorts,
	}

	hostCfg := &container.HostConfig{
		PortBindings: portMap,
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Resources: container.Resources{
			Memory:   int64(ContainerMemoryMB(s.MemoryMB)) * 1024 * 1024,
			NanoCPUs: int64(s.CPUCores * 1e9),
		},
		Binds: []string{s.DataDir + ":/data:z"},
	}

	resp, err := c.cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, containerName)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *Client) CreateSetupContainer(ctx context.Context, name, imageRef, dataDir string, env []string) (string, error) {
	cfg := &container.Config{Image: imageRef, Env: withHostUser(append(env, "SETUP_ONLY=true"))}
	hostCfg := &container.HostConfig{Binds: []string{dataDir + ":/data:z"}}
	resp, err := c.cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, config.ContainerNamePrefix+name+"-setup")
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *Client) WaitContainer(ctx context.Context, containerID string) (int64, error) {
	statusCh, errCh := c.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case st := <-statusCh:
		return st.StatusCode, nil
	case err := <-errCh:
		return -1, err
	}
}

func withHostUser(env []string) []string {
	uid, gid := os.Getuid(), os.Getgid()
	if uid <= 0 || gid < 0 {
		return env
	}
	return append(env, "UID="+strconv.Itoa(uid), "GID="+strconv.Itoa(gid))
}

func ContainerMemoryMB(heapMB int) int {
	overhead := heapMB / 4
	if overhead < 512 {
		overhead = 512
	}
	return heapMB + overhead
}

func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

func (c *Client) StopContainer(ctx context.Context, containerID string, timeoutSec *int) error {
	opts := container.StopOptions{}
	if timeoutSec != nil && *timeoutSec > 0 {
		opts.Timeout = timeoutSec
	}
	return c.cli.ContainerStop(ctx, containerID, opts)
}

func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
}

func (c *Client) InspectContainer(ctx context.Context, containerID string) (interface{}, error) {
	return c.cli.ContainerInspect(ctx, containerID)
}

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

func (c *Client) ContainerState(ctx context.Context, containerID string) (running bool, exists bool, err error) {
	inspect, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, err
	}
	return inspect.State.Running, true, nil
}

func (c *Client) ContainerRunning(ctx context.Context, containerID string) (bool, error) {
	inspect, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}
	return inspect.State.Running, nil
}

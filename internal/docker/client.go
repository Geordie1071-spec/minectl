package docker

import (
	"context"

	"fmt"
	"github.com/docker/docker/client"
	"os"
	"strings"
)

type Client struct {
	cli *client.Client
}

func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

func (c *Client) API() *client.Client {
	return c.cli
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func (c *Client) CheckDockerRunning(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return Explain(err)
}

// PermissionError means Docker is running but this user may not use it,
// almost always because they were just added to the docker group and this
// login session started before that.
type PermissionError struct{ Err error }

func (e *PermissionError) Error() string {
	user := os.Getenv("USER")
	if user == "" {
		user = "this user"
	}
	return fmt.Sprintf("%s can't use Docker yet (permission denied on the Docker socket). "+
		"If you were just added to the docker group, log out and back in (or run: newgrp docker). "+
		"Otherwise add yourself: sudo usermod -aG docker %s", user, user)
}

func (e *PermissionError) Unwrap() error { return e.Err }

// Explain turns a refused Docker socket into a PermissionError that says
// what to do; other errors pass through unchanged.
func Explain(err error) error {
	if err != nil && strings.Contains(err.Error(), "permission denied") && strings.Contains(err.Error(), "docker.sock") {
		return &PermissionError{Err: err}
	}
	return err
}

type HostResources struct {
	MemoryMB int
	CPUs     int
}

func (c *Client) HostResources(ctx context.Context) (HostResources, error) {
	info, err := c.cli.Info(ctx)
	if err != nil {
		return HostResources{}, err
	}
	return HostResources{MemoryMB: int(info.MemTotal / (1024 * 1024)), CPUs: info.NCPU}, nil
}

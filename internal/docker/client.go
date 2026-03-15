package docker

import (
	"context"

	"github.com/docker/docker/client"
)

// Client wraps the Docker API client
type Client struct {
	cli *client.Client
}

// New creates a Docker client (uses env DOCKER_HOST etc.)
func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

// API returns the underlying Docker client for advanced use
func (c *Client) API() *client.Client {
	return c.cli
}

// Close closes the Docker client
func (c *Client) Close() error {
	return c.cli.Close()
}

// CheckDockerRunning pings the Docker daemon to verify it's running
func (c *Client) CheckDockerRunning(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

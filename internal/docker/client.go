package docker

import (
	"context"

	"github.com/docker/docker/client"
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
	return err
}

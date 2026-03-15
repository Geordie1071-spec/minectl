package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
)

func (c *Client) ImageExists(ctx context.Context, imageRef string) (bool, error) {
	_, _, err := c.cli.ImageInspectWithRaw(ctx, imageRef)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	return c.cli.ImagePull(ctx, imageRef, types.ImagePullOptions{})
}

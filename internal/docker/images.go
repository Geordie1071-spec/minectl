package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
)

// ImageExists returns true if the image exists locally
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

// PullImage pulls an image; progress can be read from the returned reader
func (c *Client) PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	return c.cli.ImagePull(ctx, imageRef, types.ImagePullOptions{})
}

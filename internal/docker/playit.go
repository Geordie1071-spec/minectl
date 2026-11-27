package docker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/errdefs"
)

// PlayitContainer runs the playit.gg agent. Server names can't contain "_",
// so it never collides with a server's container.
const PlayitContainer = "minectl_playit"

const playitKeyLabel = "minectl.playit-key"

func keyFingerprint(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:8])
}

// EnsurePlayitAgent starts the playit agent with secret, replacing one that
// runs with a different key.
func (c *Client) EnsurePlayitAgent(ctx context.Context, imageRef, secret string) error {
	fp := keyFingerprint(secret)
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: filters.NewArgs(filters.Arg("name", "^/"+PlayitContainer+"$"))})
	if err != nil {
		return err
	}
	if len(list) > 0 {
		cur := list[0]
		if cur.Labels[playitKeyLabel] == fp {
			if cur.State == "running" {
				return nil
			}
			return c.StartContainer(ctx, cur.ID)
		}
		if err := c.RemoveContainer(ctx, cur.ID); err != nil {
			return err
		}
	}
	if ok, err := c.ImageExists(ctx, imageRef); err != nil {
		return err
	} else if !ok {
		if err := c.PullImageWithProgress(ctx, imageRef, func(int) {}); err != nil {
			return err
		}
	}
	cfg := &container.Config{
		Image:  imageRef,
		Env:    []string{"SECRET_KEY=" + secret},
		Labels: map[string]string{playitKeyLabel: fp},
	}
	hostCfg := &container.HostConfig{
		// Host networking lets the agent reach servers on 127.0.0.1:<port>.
		NetworkMode:   "host",
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
	}
	resp, err := c.cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, PlayitContainer)
	if err != nil {
		return err
	}
	return c.StartContainer(ctx, resp.ID)
}

// RemovePlayitAgent stops and removes the agent, if there is one.
func (c *Client) RemovePlayitAgent(ctx context.Context) error {
	err := c.RemoveContainer(ctx, PlayitContainer)
	if errdefs.IsNotFound(err) {
		return nil
	}
	return err
}

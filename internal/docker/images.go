package docker

import (
	"context"
	"encoding/json"
	"fmt"
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

func (c *Client) PullImageWithProgress(ctx context.Context, imageRef string, onProgress func(percent int)) error {
	rc, err := c.PullImage(ctx, imageRef)
	if err != nil {
		return err
	}
	defer rc.Close()
	return decodePullProgress(rc, onProgress)
}

type layerProgress struct {
	current, total int64
	done           bool
}

func decodePullProgress(r io.Reader, onProgress func(percent int)) error {
	layers := map[string]*layerProgress{}
	var order []string
	dec := json.NewDecoder(r)
	for {
		var msg struct {
			ID             string `json:"id"`
			Status         string `json:"status"`
			Error          string `json:"error"`
			ProgressDetail struct {
				Current int64 `json:"current"`
				Total   int64 `json:"total"`
			} `json:"progressDetail"`
		}
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if msg.Error != "" {
			return fmt.Errorf("%s", msg.Error)
		}
		if msg.ID == "" {
			continue
		}
		l, ok := layers[msg.ID]
		if !ok {
			l = &layerProgress{}
			layers[msg.ID] = l
			order = append(order, msg.ID)
		}
		switch msg.Status {
		case "Downloading":
			l.current, l.total = msg.ProgressDetail.Current, msg.ProgressDetail.Total
		case "Download complete", "Pull complete", "Already exists":
			l.done = true
		default:
			continue
		}
		if onProgress != nil {
			onProgress(pullPercent(layers, order))
		}
	}
}

func pullPercent(layers map[string]*layerProgress, order []string) int {
	var current, total int64
	for _, id := range order {
		l := layers[id]
		if l.total == 0 {
			continue
		}
		total += l.total
		if l.done {
			current += l.total
		} else {
			current += l.current
		}
	}
	if total == 0 {
		return 0
	}
	return int(current * 100 / total)
}

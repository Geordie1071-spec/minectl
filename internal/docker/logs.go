package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

func (c *Client) GetLogs(ctx context.Context, containerID string, tail int) ([]string, error) {
	tailStr := "50"
	if tail > 0 && tail <= 10000 {
		tailStr = fmt.Sprintf("%d", tail)
	}
	rdr, err := c.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tailStr,
	})
	if err != nil {
		return nil, err
	}
	defer rdr.Close()

	var out strings.Builder
	_, err = stdcopy.StdCopy(&out, &out, rdr)
	if err != nil && err != io.EOF {
		return nil, err
	}

	var lines []string
	sc := bufio.NewScanner(strings.NewReader(out.String()))
	for sc.Scan() {
		line := sc.Text()
		lines = append(lines, line)
	}
	return lines, sc.Err()
}

func (c *Client) StreamLogs(ctx context.Context, containerID string, follow bool, outCh chan<- string) error {
	return c.streamLogs(ctx, containerID, container.LogsOptions{Follow: follow}, outCh)
}

func (c *Client) FollowLogs(ctx context.Context, containerID string, tail int, outCh chan<- string) error {
	return c.streamLogs(ctx, containerID, container.LogsOptions{Follow: true, Tail: strconv.Itoa(tail)}, outCh)
}

func (c *Client) StreamLogsSince(ctx context.Context, containerID string, follow bool, since time.Time, outCh chan<- string) error {
	opts := container.LogsOptions{Follow: follow}
	if !since.IsZero() {
		opts.Since = DockerTimestamp(since)
	}
	return c.streamLogs(ctx, containerID, opts, outCh)
}

func (c *Client) streamLogs(ctx context.Context, containerID string, opts container.LogsOptions, outCh chan<- string) error {
	opts.ShowStdout = true
	opts.ShowStderr = true
	rdr, err := c.cli.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return err
	}
	defer rdr.Close()

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		_, _ = stdcopy.StdCopy(pw, pw, rdr)
	}()
	sc := bufio.NewScanner(pr)
	for sc.Scan() {
		select {
		case outCh <- sc.Text():
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return sc.Err()
}

func DockerTimestamp(t time.Time) string {
	return fmt.Sprintf("%d.%09d", t.Unix(), t.Nanosecond())
}

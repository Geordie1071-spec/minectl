package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
)

func (c *Client) GetLogs(ctx context.Context, containerID string, tail int) ([]string, error) {
	tailStr := "50"
	if tail > 0 && tail <= 10000 {
		tailStr = fmt.Sprintf("%d", tail)
	}
	rdr, err := c.cli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
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
	rdr, err := c.cli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: false,
	})
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

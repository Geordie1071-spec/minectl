package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
)

func (c *Client) ExecCommand(ctx context.Context, containerID string, cmd []string) (stdout string, err error) {
	cfg := types.ExecConfig{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}
	resp, err := c.cli.ContainerExecCreate(ctx, containerID, cfg)
	if err != nil {
		return "", err
	}

	attach, err := c.cli.ContainerExecAttach(ctx, resp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", err
	}
	defer attach.Close()

	var out strings.Builder
	_, err = io.Copy(&out, attach.Reader)
	if err != nil && err != io.EOF {
		return out.String(), err
	}
	return strings.TrimSpace(out.String()), nil
}

func (c *Client) ExecRcon(ctx context.Context, containerID string, command string) (string, error) {
	return c.ExecCommand(ctx, containerID, []string{"rcon-cli", command})
}

func (c *Client) ExecRconStream(ctx context.Context, containerID string, command string, outCh chan<- string) error {
	cfg := types.ExecConfig{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"rcon-cli", command},
	}
	resp, err := c.cli.ContainerExecCreate(ctx, containerID, cfg)
	if err != nil {
		return err
	}

	attach, err := c.cli.ContainerExecAttach(ctx, resp.ID, types.ExecStartCheck{})
	if err != nil {
		return err
	}
	defer attach.Close()

	scanner := bufio.NewScanner(attach.Reader)
	for scanner.Scan() {
		line := scanner.Text()
		select {
		case outCh <- line:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return scanner.Err()
}

func (c *Client) ExecInspect(ctx context.Context, execID string) (int, error) {
	inspect, err := c.cli.ContainerExecInspect(ctx, execID)
	if err != nil {
		return -1, err
	}
	return inspect.ExitCode, nil
}

func DecodeDockerStream(r io.Reader, fn func(line string, stderr bool)) error {
	dec := json.NewDecoder(r)
	for {
		var frame struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := dec.Decode(&frame); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if frame.Error != "" {
			return fmt.Errorf("docker: %s", frame.Error)
		}
		if frame.Stream != "" {
			fn(frame.Stream, false)
		}
	}
}

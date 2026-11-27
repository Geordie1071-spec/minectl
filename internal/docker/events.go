package docker

import (
	"context"
	"strconv"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

// ContainerEvent is a die or oom event for a minectl container.
type ContainerEvent struct {
	Server   string
	Action   string // "die" or "oom"
	ExitCode int
}

// WatchContainers calls fn for every die and oom event of a minectl
// container until ctx ends or the event stream fails.
func (c *Client) WatchContainers(ctx context.Context, fn func(ContainerEvent)) error {
	msgs, errs := c.cli.Events(ctx, types.EventsOptions{Filters: filters.NewArgs(
		filters.Arg("type", "container"),
		filters.Arg("event", "die"),
		filters.Arg("event", "oom"),
	)})
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errs:
			return err
		case m := <-msgs:
			name := m.Actor.Attributes["name"]
			if !strings.HasPrefix(name, config.ContainerNamePrefix) {
				continue
			}
			code, _ := strconv.Atoi(m.Actor.Attributes["exitCode"])
			fn(ContainerEvent{Server: strings.TrimPrefix(name, config.ContainerNamePrefix), Action: string(m.Action), ExitCode: code})
		}
	}
}

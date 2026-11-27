package docker

import (
	"context"
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types"
)

type ContainerStats struct {
	CPUPercent    float64
	MemoryUsage   uint64
	MemoryLimit   uint64
	MemoryPercent float64
	PIDs          uint64
}

func (c *Client) GetStats(ctx context.Context, containerID string) (*ContainerStats, error) {
	rdr, err := c.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	defer rdr.Body.Close()

	var v types.StatsJSON
	if err := json.NewDecoder(rdr.Body).Decode(&v); err != nil {
		return nil, err
	}
	return computeStats(&v), nil
}

func (c *Client) StreamStats(ctx context.Context, containerID string, fn func(*ContainerStats)) error {
	rdr, err := c.cli.ContainerStats(ctx, containerID, true)
	if err != nil {
		return err
	}
	defer rdr.Body.Close()

	dec := json.NewDecoder(rdr.Body)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var v types.StatsJSON
		if err := dec.Decode(&v); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		fn(computeStats(&v))
	}
}

// computeStats matches what `docker stats` shows: CPU relative to one core,
// and memory without reclaimable page cache.
func computeStats(v *types.StatsJSON) *ContainerStats {
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)
	cpus := float64(v.CPUStats.OnlineCPUs)
	if cpus == 0 {
		cpus = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
	}
	cpuPercent := 0.0
	if sysDelta > 0 && cpuDelta > 0 {
		cpuPercent = cpuDelta / sysDelta * cpus * 100.0
	}

	memUsage := v.MemoryStats.Usage
	cache := v.MemoryStats.Stats["inactive_file"] // cgroup v2
	if cache == 0 {
		cache = v.MemoryStats.Stats["total_inactive_file"] // cgroup v1
	}
	if cache < memUsage {
		memUsage -= cache
	}
	memLimit := v.MemoryStats.Limit
	memPercent := 0.0
	if memLimit > 0 {
		memPercent = float64(memUsage) / float64(memLimit) * 100.0
	}
	return &ContainerStats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   memUsage,
		MemoryLimit:   memLimit,
		MemoryPercent: memPercent,
		PIDs:          v.PidsStats.Current,
	}
}

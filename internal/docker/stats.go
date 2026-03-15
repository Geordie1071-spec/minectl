package docker

import (
	"context"
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types"
)

// ContainerStats holds CPU/memory stats for a container
type ContainerStats struct {
	CPUPercent    float64
	MemoryUsage   uint64
	MemoryLimit   uint64
	MemoryPercent float64
	PIDs          uint64
}

// GetStats returns one-shot stats for the container
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

	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if sysDelta > 0 {
		cpuPercent = (cpuDelta / sysDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	memUsage := v.MemoryStats.Usage
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
	}, nil
}

// StreamStats decodes the container stats stream and calls fn for each update. Cancel ctx to stop.
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
		cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
		sysDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
		cpuPercent := 0.0
		if sysDelta > 0 {
			cpuPercent = (cpuDelta / sysDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
		}
		memUsage := v.MemoryStats.Usage
		memLimit := v.MemoryStats.Limit
		memPercent := 0.0
		if memLimit > 0 {
			memPercent = float64(memUsage) / float64(memLimit) * 100.0
		}
		fn(&ContainerStats{
			CPUPercent:    cpuPercent,
			MemoryUsage:   memUsage,
			MemoryLimit:   memLimit,
			MemoryPercent: memPercent,
			PIDs:          v.PidsStats.Current,
		})
	}
}

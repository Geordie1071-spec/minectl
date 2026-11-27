package docker

import (
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
)

func TestDecodePullProgress(t *testing.T) {
	stream := strings.Join([]string{
		`{"status":"Pulling from itzg/minecraft-server","id":"java21"}`,
		`{"status":"Pulling fs layer","id":"a"}`,
		`{"status":"Already exists","id":"b"}`,
		`{"status":"Downloading","progressDetail":{"current":50,"total":100},"id":"a"}`,
		`{"status":"Downloading","progressDetail":{"current":25,"total":100},"id":"c"}`,
		`{"status":"Download complete","id":"a"}`,
		`{"status":"Download complete","id":"c"}`,
	}, "\n")

	var got []int
	if err := decodePullProgress(strings.NewReader(stream), func(p int) { got = append(got, p) }); err != nil {
		t.Fatal(err)
	}
	want := []int{0, 50, 37, 62, 100}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestDecodePullProgressError(t *testing.T) {
	err := decodePullProgress(strings.NewReader(`{"error":"manifest unknown"}`), nil)
	if err == nil || !strings.Contains(err.Error(), "manifest unknown") {
		t.Fatalf("err = %v", err)
	}
}

func TestContainerMemoryMB(t *testing.T) {
	if got := ContainerMemoryMB(1024); got != 1536 {
		t.Errorf("1024 -> %d, want 1536", got)
	}
	if got := ContainerMemoryMB(8192); got != 10240 {
		t.Errorf("8192 -> %d, want 10240", got)
	}
}

func TestDockerTimestamp(t *testing.T) {
	ts := time.Date(2026, 11, 26, 18, 41, 44, 205000000, time.UTC)
	if got := DockerTimestamp(ts); got != "1795718504.205000000" {
		t.Errorf("DockerTimestamp = %q", got)
	}
}

func TestComputeStats(t *testing.T) {
	var v types.StatsJSON
	v.CPUStats.CPUUsage.TotalUsage = 3_000_000
	v.PreCPUStats.CPUUsage.TotalUsage = 1_000_000
	v.CPUStats.SystemUsage = 20_000_000
	v.PreCPUStats.SystemUsage = 10_000_000
	v.CPUStats.OnlineCPUs = 4
	v.MemoryStats.Usage = 3000
	v.MemoryStats.Limit = 4000
	v.MemoryStats.Stats = map[string]uint64{"inactive_file": 1000}
	s := computeStats(&v)
	if s.CPUPercent != 80 || s.MemoryUsage != 2000 || s.MemoryPercent != 50 {
		t.Fatalf("got %+v", s)
	}
	v.CPUStats.OnlineCPUs = 0
	v.CPUStats.CPUUsage.PercpuUsage = []uint64{1, 2}
	v.MemoryStats.Stats = map[string]uint64{"total_inactive_file": 500}
	if s := computeStats(&v); s.CPUPercent != 40 || s.MemoryUsage != 2500 {
		t.Fatalf("cgroup v1 got %+v", s)
	}
}

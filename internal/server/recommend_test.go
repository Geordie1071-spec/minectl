package server

import (
	"strings"
	"testing"

	"github.com/Geordie1071-spec/minectl/internal/docker"
)

func TestRecommend(t *testing.T) {
	big := docker.HostResources{MemoryMB: 65536, CPUs: 16}
	cases := []struct {
		mcType        string
		players, mods int
		wantMB        int
		wantCPU       float64
	}{
		{"paper", 5, 0, 2048, 2},
		{"paper", 20, 0, 3584, 2},
		{"fabric", 5, 0, 3072, 3},
		{"fabric", 5, 60, 4096, 3},
		{"neoforge", 5, 200, 7168, 4},
		{"forge", 100, 400, 16384, 4},
	}
	for _, c := range cases {
		r := Recommend(c.mcType, c.players, c.mods, big)
		if r.MemoryMB != c.wantMB || r.CPUCores != c.wantCPU {
			t.Errorf("Recommend(%s, %d players, %d mods) = %s / %g cores, want %s / %g", c.mcType, c.players, c.mods, FormatMB(r.MemoryMB), r.CPUCores, FormatMB(c.wantMB), c.wantCPU)
		}
	}
}

func TestRecommendCapsToHost(t *testing.T) {
	small := docker.HostResources{MemoryMB: 8192, CPUs: 2}
	r := Recommend("neoforge", 5, 200, small)
	if docker.ContainerMemoryMB(r.MemoryMB) > small.MemoryMB*3/4 {
		t.Errorf("recommended %s does not fit on an 8G host", FormatMB(r.MemoryMB))
	}
	if r.CPUCores != 2 {
		t.Errorf("CPU = %g, want capped to 2", r.CPUCores)
	}
	if len(r.Warnings) == 0 {
		t.Error("expected a warning about limited RAM")
	}
}

func TestCheckMemoryChoice(t *testing.T) {
	host := docker.HostResources{MemoryMB: 16384}
	rec := Recommend("fabric", 5, 60, host)
	if w := CheckMemoryChoice(2048, rec, host, 0); len(w) == 0 || !strings.Contains(w[0], "below the recommended") {
		t.Errorf("expected low-memory warning, got %v", w)
	}
	if w := CheckMemoryChoice(rec.MemoryMB, rec, host, 0); len(w) != 0 {
		t.Errorf("unexpected warnings: %v", w)
	}
	if w := CheckMemoryChoice(8192, rec, host, 8192); len(w) == 0 {
		t.Error("expected warning when running servers plus this one exceed host RAM")
	}
}

func TestFormatMB(t *testing.T) {
	cases := map[int]string{1024: "1G", 1536: "1.5G", 5120: "5G", 512: "512M", 3000: "3000M"}
	for in, want := range cases {
		if got := FormatMB(in); got != want {
			t.Errorf("FormatMB(%d) = %q, want %q", in, got, want)
		}
	}
}

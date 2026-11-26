package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

type resourcePlan struct {
	MemoryMB   int
	CPUCores   float64
	AutoMemory bool
	AutoCPU    bool
	Rec        server.Recommendation
	Warnings   []string
}

func planResources(ctx context.Context, d *docker.Client, st *store.Store, mcType string, players, mods int, memFlag, cpuFlag string) (resourcePlan, error) {
	host, _ := d.HostResources(ctx)
	rec := server.Recommend(mcType, players, mods, host)
	plan := resourcePlan{Rec: rec, Warnings: append([]string(nil), rec.Warnings...)}

	if strings.EqualFold(strings.TrimSpace(memFlag), "auto") {
		plan.MemoryMB, plan.AutoMemory = rec.MemoryMB, true
	} else {
		mb, err := parseMemory(memFlag)
		if err != nil {
			return plan, err
		}
		plan.MemoryMB = mb
		plan.Warnings = append(plan.Warnings, server.CheckMemoryChoice(mb, rec, host, runningMemoryMB(st))...)
	}

	switch c := strings.ToLower(strings.TrimSpace(cpuFlag)); c {
	case "auto":
		plan.CPUCores, plan.AutoCPU = rec.CPUCores, true
	case "none", "0", "unlimited":
		plan.CPUCores = 0
	default:
		n, err := strconv.ParseFloat(c, 64)
		if err != nil || n < 0 {
			return plan, fmt.Errorf("invalid --cpu %q (use e.g. 2, auto, or none)", cpuFlag)
		}
		plan.CPUCores = n
		if host.CPUs > 0 && n > float64(host.CPUs) {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("--cpu %g is more than this machine's %d cores", n, host.CPUs))
		}
	}
	return plan, nil
}

func runningMemoryMB(st *store.Store) int {
	servers, err := st.ListServers()
	if err != nil {
		return 0
	}
	total := 0
	for _, s := range servers {
		if s.Status == domain.StatusRunning {
			total += docker.ContainerMemoryMB(s.MemoryMB)
		}
	}
	return total
}

func (p resourcePlan) print() {
	mem := server.FormatMB(p.MemoryMB) + " RAM"
	if p.AutoMemory {
		mem += " (recommended: " + p.Rec.Reason + ")"
	}
	cpu := "no CPU limit"
	if p.CPUCores > 0 {
		cpu = strconv.FormatFloat(p.CPUCores, 'f', -1, 64) + " CPU cores"
	}
	if p.AutoCPU {
		cpu += " (recommended)"
	}
	fmt.Printf("Resources: %s, %s\n", mem, cpu)
	if p.AutoMemory || p.AutoCPU {
		fmt.Println("  Change with --memory / --cpu, or tune the estimate with --players / --mods.")
	}
	for _, w := range p.Warnings {
		fmt.Fprintln(os.Stderr, "Warning:", w)
	}
}

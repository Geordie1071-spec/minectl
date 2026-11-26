package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/modrinth"
	"github.com/Geordie1071-spec/minectl/internal/server"
)

type Plan struct {
	MemoryMB   int
	CPUCores   float64
	AutoMemory bool
	AutoCPU    bool
	Rec        server.Recommendation
	Warnings   []string
}

func ParseMemory(s string) (int, error) {
	in := s
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return config.DefaultMemoryMB, nil
	}
	mult := 0
	if strings.HasSuffix(s, "G") {
		mult = 1024
		s = strings.TrimSuffix(s, "G")
	} else if strings.HasSuffix(s, "M") {
		mult = 1
		s = strings.TrimSuffix(s, "M")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid memory: %s (use e.g. 4G or 4096M)", in)
	}
	if mult == 0 {
		mult = 1
		if n <= 64 {
			mult = 1024
		}
	}
	return n * mult, nil
}

func (m *Manager) host(ctx context.Context) docker.HostResources {
	if m.D == nil {
		return docker.HostResources{}
	}
	h, _ := m.D.HostResources(ctx)
	return h
}

func (m *Manager) PlanResources(ctx context.Context, mcType string, players, mods int, memory, cpu string) (Plan, error) {
	host := m.host(ctx)
	rec := server.Recommend(mcType, players, mods, host)
	plan := Plan{Rec: rec, Warnings: append([]string(nil), rec.Warnings...)}

	if strings.EqualFold(strings.TrimSpace(memory), "auto") {
		plan.MemoryMB, plan.AutoMemory = rec.MemoryMB, true
	} else {
		mb, err := ParseMemory(memory)
		if err != nil {
			return plan, err
		}
		plan.MemoryMB = mb
		plan.Warnings = append(plan.Warnings, server.CheckMemoryChoice(mb, rec, host, m.runningMemoryMB())...)
	}

	switch c := strings.ToLower(strings.TrimSpace(cpu)); c {
	case "auto":
		plan.CPUCores, plan.AutoCPU = rec.CPUCores, true
	case "none", "0", "unlimited":
		plan.CPUCores = 0
	default:
		n, err := strconv.ParseFloat(c, 64)
		if err != nil || n < 0 {
			return plan, fmt.Errorf("invalid --cpu %q (use e.g. 2, auto, or none)", cpu)
		}
		plan.CPUCores = n
		if host.CPUs > 0 && n > float64(host.CPUs) {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("--cpu %g is more than this machine's %d cores", n, host.CPUs))
		}
	}
	return plan, nil
}

func (m *Manager) runningMemoryMB() int {
	servers, err := m.St.ListServers()
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

func (p Plan) report(ev Events) {
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
	msg := fmt.Sprintf("Resources: %s, %s", mem, cpu)
	if p.AutoMemory || p.AutoCPU {
		msg += "\n  Change with --memory / --cpu, or tune the estimate with --players / --mods."
	}
	ev.Info(msg)
	for _, w := range p.Warnings {
		ev.Warn(w)
	}
}

type RecommendRequest struct {
	Type    string
	Players int
	Mods    int
	Modpack string
}

type RecommendResult struct {
	Recommendation server.Recommendation
	Modpack        *modrinth.ModpackRelease
	Mods           int
}

func (m *Manager) Recommend(ctx context.Context, req RecommendRequest) (*RecommendResult, error) {
	mcType, mods := req.Type, req.Mods
	res := &RecommendResult{}
	if req.Modpack != "" {
		mc := modrinth.NewClient()
		rel, err := mc.ResolveModpack(req.Modpack, "", "", "")
		if err != nil {
			return nil, err
		}
		n, err := mc.CountModpackMods(rel.FileURL)
		if err != nil {
			return nil, err
		}
		mcType, mods, res.Modpack = rel.Loader, n, rel
	}
	res.Mods = mods
	res.Recommendation = server.Recommend(mcType, req.Players, mods, m.host(ctx))
	return res, nil
}

func (m *Manager) modpackModCount(s *domain.Server) int {
	if !s.HasModrinthModpack() {
		return len(s.Mods)
	}
	mc := modrinth.NewClient()
	ver := ""
	if s.ModpackVersion != nil {
		ver = *s.ModpackVersion
	}
	rel, err := mc.ResolveModpack(*s.ModpackID, s.MCType, "", ver)
	if err != nil {
		return len(s.Mods)
	}
	n, err := mc.CountModpackMods(rel.FileURL)
	if err != nil {
		return len(s.Mods)
	}
	return n
}

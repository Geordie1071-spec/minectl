package api

import (
	"errors"
	"github.com/Geordie1071-spec/minectl/internal/expose"
	"net/http"
	"strconv"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/modrinth"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/Geordie1071-spec/minectl/internal/versions"
)

func (a *API) versions(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	mcType := strings.ToLower(r.PathValue("type"))
	list, err := versions.NewClient().Supported(mcType)
	if errors.Is(err, versions.ErrUnchecked) {
		writeErr(w, http.StatusBadRequest, "can't list versions for "+mcType)
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) searchModpacks(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 100 {
		writeErr(w, http.StatusBadRequest, "search is too long")
		return
	}
	hits, err := modrinth.NewClient().SearchModpacks(q)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if hits == nil {
		hits = []modrinth.ModResult{}
	}
	writeJSON(w, http.StatusOK, hits)
}

type recommendView struct {
	MemoryMB    int     `json:"memory_mb"`
	CPUCores    float64 `json:"cpu_cores"`
	Recommended struct {
		MemoryMB int     `json:"memory_mb"`
		CPUCores float64 `json:"cpu_cores"`
		Reason   string  `json:"reason"`
	} `json:"recommended"`
	Warnings []string                 `json:"warnings"`
	Type     string                   `json:"type"`
	Mods     int                      `json:"mods"`
	Modpack  *modrinth.ModpackRelease `json:"modpack,omitempty"`
	Host     struct {
		MemoryMB int `json:"memory_mb"`
		CPUs     int `json:"cpus"`
	} `json:"host"`
}

// recommend previews the resources create would pick. memory and cpu are
// optional; when given, the warnings cover that choice.
func (a *API) recommend(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	q := r.URL.Query()
	mcType := strings.ToLower(q.Get("type"))
	if mcType == "" {
		mcType = domain.TypePaper
	}
	players, _ := strconv.Atoi(q.Get("players"))
	mods, _ := strconv.Atoi(q.Get("mods"))
	if players <= 0 {
		players = 5
	}
	if players > 1000 || mods < 0 || mods > 5000 {
		writeErr(w, http.StatusBadRequest, "players must be 1-1000 and mods 0-5000")
		return
	}
	memory, cpu := q.Get("memory"), q.Get("cpu")
	if memory == "" {
		memory = "auto"
	}
	if cpu == "" {
		cpu = "auto"
	}
	if err := service.ValidateMemory(memory); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var out recommendView
	if pack := q.Get("modpack"); pack != "" {
		res, err := a.m.Recommend(r.Context(), service.RecommendRequest{Modpack: pack, Players: players})
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		mcType, mods, out.Modpack = res.Modpack.Loader, res.Mods, res.Modpack
	}
	plan, err := a.m.PlanResources(r.Context(), mcType, players, mods, memory, cpu)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out.MemoryMB, out.CPUCores, out.Type, out.Mods = plan.MemoryMB, plan.CPUCores, mcType, mods
	out.Recommended.MemoryMB, out.Recommended.CPUCores, out.Recommended.Reason = plan.Rec.MemoryMB, plan.Rec.CPUCores, plan.Rec.Reason
	out.Warnings = plan.Warnings
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	if a.m.D != nil {
		if h, err := a.m.D.HostResources(r.Context()); err == nil {
			out.Host.MemoryMB, out.Host.CPUs = h.MemoryMB, h.CPUs
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// address tells the UI what players should type to join: the configured
// public address if there is one, and this machine's LAN address. Hosted is
// true on a machine that's on the internet directly, such as a VPS, where
// servers need no port forwarding.
func (a *API) address(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	_, direct := expose.Hosted()
	writeJSON(w, http.StatusOK, map[string]any{
		"public": a.cfg.PublicAddress,
		"lan":    expose.LANIP(),
		"hosted": a.cfg.Hosted || direct,
	})
}

func (a *API) host(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	if !a.needDocker(w) {
		return
	}
	h, err := a.m.D.HostResources(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"memory_mb": h.MemoryMB, "cpus": h.CPUs, "running_memory_mb": a.m.RunningMemoryMB()})
}

func (a *API) listAudit(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	list, err := a.st.ListAudit(limit, before)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

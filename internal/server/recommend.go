package server

import (
	"fmt"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
)

const (
	minMemoryMB = 1024
	maxMemoryMB = 16384
)

type Recommendation struct {
	MemoryMB int
	CPUCores float64
	Reason   string
	Warnings []string
}

func isModded(mcType string) bool {
	switch mcType {
	case domain.TypeFabric, domain.TypeQuilt, domain.TypeForge, domain.TypeNeoForge:
		return true
	}
	return false
}

func Recommend(mcType string, players, mods int, host docker.HostResources) Recommendation {
	if players < 1 {
		players = 1
	}
	if mods < 0 {
		mods = 0
	}

	base, perPlayer, perMod := 1536, 100, 0
	switch mcType {
	case domain.TypeFabric, domain.TypeQuilt:
		base, perPlayer, perMod = 2048, 150, 20
	case domain.TypeForge, domain.TypeNeoForge:
		base, perPlayer, perMod = 3072, 150, 20
	}
	raw := base + players*perPlayer + modMemoryMB(mods, perMod)
	mem := roundMemory(raw)

	reason := fmt.Sprintf("%s base %s + %d player(s)", mcType, FormatMB(base), players)
	if mods > 0 {
		reason += fmt.Sprintf(" + %d mod(s)", mods)
	}

	cpu := 2.0
	if isModded(mcType) {
		cpu = 3
		if mods >= 100 || players > 20 {
			cpu = 4
		}
	}

	r := Recommendation{MemoryMB: mem, CPUCores: cpu, Reason: reason}

	if host.CPUs > 0 && r.CPUCores > float64(host.CPUs) {
		r.CPUCores = float64(host.CPUs)
	}
	if host.MemoryMB > 0 {
		budget := host.MemoryMB * 3 / 4
		if docker.ContainerMemoryMB(r.MemoryMB) > budget {
			capped := roundDown(budget * 4 / 5)
			if capped < minMemoryMB {
				capped = minMemoryMB
			}
			r.Warnings = append(r.Warnings, fmt.Sprintf("recommended %s, but this machine has %s RAM; using %s, which may lag", FormatMB(r.MemoryMB), FormatMB(host.MemoryMB), FormatMB(capped)))
			r.MemoryMB = capped
		}
	}
	return r
}

func modMemoryMB(mods, perMod int) int {
	if mods <= 100 {
		return mods * perMod
	}
	return 100*perMod + (mods-100)*perMod/2
}

func CheckMemoryChoice(chosenMB int, rec Recommendation, host docker.HostResources, othersRunningMB int) []string {
	var warnings []string
	if chosenMB < rec.MemoryMB*7/10 {
		warnings = append(warnings, fmt.Sprintf("%s is well below the recommended %s (%s); expect lag or crashes", FormatMB(chosenMB), FormatMB(rec.MemoryMB), rec.Reason))
	}
	if host.MemoryMB > 0 {
		need := docker.ContainerMemoryMB(chosenMB) + othersRunningMB
		if need > host.MemoryMB {
			msg := fmt.Sprintf("this server needs up to %s", FormatMB(docker.ContainerMemoryMB(chosenMB)))
			if othersRunningMB > 0 {
				msg += fmt.Sprintf(" and your running servers use up to %s", FormatMB(othersRunningMB))
			}
			warnings = append(warnings, msg+fmt.Sprintf(", but this machine has %s RAM", FormatMB(host.MemoryMB)))
		}
	}
	return warnings
}

func roundMemory(mb int) int {
	step := 512
	if mb > 4096 {
		step = 1024
	}
	mb = (mb + step - 1) / step * step
	if mb < minMemoryMB {
		mb = minMemoryMB
	}
	if mb > maxMemoryMB {
		mb = maxMemoryMB
	}
	return mb
}

func roundDown(mb int) int {
	return mb / 512 * 512
}

func FormatMB(mb int) string {
	if mb%1024 == 0 {
		return fmt.Sprintf("%dG", mb/1024)
	}
	if mb > 1024 && mb%512 == 0 {
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(mb)/1024), ".0") + "G"
	}
	return fmt.Sprintf("%dM", mb)
}

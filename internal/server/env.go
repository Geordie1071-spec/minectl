package server

import (
	"strconv"
	"strings"

	"github.com/minectl/minectl/internal/domain"
)

func BuildEnvVars(s *domain.Server) []string {
	env := []string{
		"EULA=TRUE",
		"TYPE=" + strings.ToUpper(s.MCType),
		"VERSION=" + s.MCVersion,
		"MEMORY=" + strconv.Itoa(s.MemoryMB) + "M",
	}
	if !s.ModsLocked && len(s.EnabledModURLs()) > 0 {
		env = append(env, "MODS="+strings.Join(s.EnabledModURLs(), ","))
	}
	if s.ModpackSource != nil && *s.ModpackSource == "modrinth" && s.ModpackID != nil && *s.ModpackID != "" {
		env = append(env, "MODRINTH_MODPACK="+s.ResolvedModpackURL())
	}
	if s.ModpackSource != nil && *s.ModpackSource == "curseforge" && s.ModpackID != nil && *s.ModpackID != "" {
		env = append(env, "CF_SLUG="+*s.ModpackID)
	}
	if s.JavaFlags != "" {
		env = append(env, "JVM_OPTS="+s.JavaFlags)
	}
	return env
}

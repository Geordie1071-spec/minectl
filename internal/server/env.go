package server

import (
	"strconv"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/domain"
)

const CurseForgeDownloadsDir = "downloads"

func BuildEnvVars(s *domain.Server) []string {
	env := []string{
		"EULA=TRUE",
		"MEMORY=" + strconv.Itoa(s.MemoryMB) + "M",
	}
	switch {
	case s.HasModrinthModpack():
		env = append(env, "TYPE=MODRINTH", "MODRINTH_MODPACK="+*s.ModpackID)
		if s.MCVersion != "" && !strings.EqualFold(s.MCVersion, "latest") {
			env = append(env, "VERSION="+s.MCVersion)
		}
		if s.ModpackVersion != nil && *s.ModpackVersion != "" {
			env = append(env, "MODRINTH_VERSION="+*s.ModpackVersion)
		}
		switch s.MCType {
		case domain.TypeFabric, domain.TypeForge, domain.TypeQuilt:
			env = append(env, "MODRINTH_LOADER="+s.MCType)
		}
	case s.HasCurseForgeModpack():
		env = append(env, "TYPE=AUTO_CURSEFORGE", "CF_DOWNLOADS_REPO=/data/"+CurseForgeDownloadsDir)
		if strings.HasPrefix(*s.ModpackID, "http") {
			env = append(env, "CF_PAGE_URL="+*s.ModpackID)
		} else {
			env = append(env, "CF_SLUG="+*s.ModpackID)
		}
		if s.ModpackVersion != nil && *s.ModpackVersion != "" {
			env = append(env, "CF_FILE_ID="+*s.ModpackVersion)
		}
	default:
		env = append(env, "TYPE="+strings.ToUpper(s.MCType), "VERSION="+s.MCVersion)
		if projects := s.EnabledModrinthProjects(); len(projects) > 0 {
			env = append(env, "MODRINTH_PROJECTS="+strings.Join(projects, ","), "MODRINTH_DOWNLOAD_DEPENDENCIES=required")
		}
	}
	if s.JavaFlags != "" {
		env = append(env, "JVM_OPTS="+s.JavaFlags)
	}
	return append(env, propertyEnv(s.Properties)...)
}

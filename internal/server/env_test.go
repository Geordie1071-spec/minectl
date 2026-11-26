package server

import (
	"strings"
	"testing"

	"github.com/Geordie1071-spec/minectl/internal/domain"
)

func envMap(env []string) map[string]string {
	m := map[string]string{}
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		m[k] = v
	}
	return m
}

func TestBuildEnvVarsModrinthModpack(t *testing.T) {
	source, slug, ver := "modrinth", "into-the-backrooms-found-footage", "xsZBYnPY"
	s := &domain.Server{MCType: "fabric", MCVersion: "1.20.1", MemoryMB: 5120, ModpackSource: &source, ModpackID: &slug, ModpackVersion: &ver,
		Mods: []domain.Mod{{Source: "modrinth", ModID: "lithium", Enabled: true}}}
	env := envMap(BuildEnvVars(s))

	want := map[string]string{"TYPE": "MODRINTH", "MODRINTH_MODPACK": slug, "MODRINTH_VERSION": ver, "MODRINTH_LOADER": "fabric", "VERSION": "1.20.1", "MEMORY": "5120M", "EULA": "TRUE"}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}
	if _, ok := env["MODRINTH_PROJECTS"]; ok {
		t.Error("individual mods should not be installed alongside a modpack")
	}
}

func TestBuildEnvVarsNeoForgeModpackOmitsLoader(t *testing.T) {
	source, slug := "modrinth", "some-pack"
	s := &domain.Server{MCType: "neoforge", MCVersion: "1.21.1", MemoryMB: 4096, ModpackSource: &source, ModpackID: &slug}
	if _, ok := envMap(BuildEnvVars(s))["MODRINTH_LOADER"]; ok {
		t.Error("MODRINTH_LOADER should be omitted for neoforge")
	}
}

func TestBuildEnvVarsIndividualMods(t *testing.T) {
	s := &domain.Server{MCType: "fabric", MCVersion: "1.21.1", MemoryMB: 3072, Mods: []domain.Mod{
		{Source: "modrinth", ModID: "lithium", VersionID: "abc", Enabled: true},
		{Source: "modrinth", ModID: "sodium", Enabled: false},
		{Source: "modrinth", ModID: "ferrite-core", Enabled: true},
	}}
	env := envMap(BuildEnvVars(s))
	if env["TYPE"] != "FABRIC" || env["VERSION"] != "1.21.1" {
		t.Errorf("TYPE/VERSION = %q/%q", env["TYPE"], env["VERSION"])
	}
	if env["MODRINTH_PROJECTS"] != "lithium:abc,ferrite-core" {
		t.Errorf("MODRINTH_PROJECTS = %q", env["MODRINTH_PROJECTS"])
	}
	if env["MODRINTH_DOWNLOAD_DEPENDENCIES"] != "required" {
		t.Error("dependencies should be downloaded")
	}
}

func TestBuildEnvVarsCurseForge(t *testing.T) {
	source, url := "curseforge", "https://www.curseforge.com/minecraft/modpacks/better-mc-fabric-bmc2"
	s := &domain.Server{MCType: "fabric", MCVersion: "1.20.1", MemoryMB: 7168, ModpackSource: &source, ModpackID: &url}
	env := envMap(BuildEnvVars(s))
	if env["TYPE"] != "AUTO_CURSEFORGE" || env["CF_PAGE_URL"] != url || env["CF_DOWNLOADS_REPO"] != "/data/downloads" {
		t.Errorf("env = %v", env)
	}
	if _, ok := env["CF_SLUG"]; ok {
		t.Error("CF_SLUG should not be set when a page URL is given")
	}

	slug, fileID := "all-the-mods-10", "6543210"
	s.ModpackID, s.ModpackVersion = &slug, &fileID
	env = envMap(BuildEnvVars(s))
	if env["CF_SLUG"] != slug || env["CF_FILE_ID"] != fileID {
		t.Errorf("env = %v", env)
	}
}

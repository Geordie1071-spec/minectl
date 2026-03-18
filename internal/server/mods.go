package server

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/minectl/minectl/internal/domain"
	"github.com/minectl/minectl/internal/modrinth"
	"github.com/minectl/minectl/internal/store"
)

func AddMod(ctx context.Context, st *store.Store, name, modIDOrSlug, version string) (*domain.Mod, error) {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return nil, fmt.Errorf("server not found: %s", name)
	}
	if s.ModsLocked {
		return nil, fmt.Errorf("server uses a modpack; cannot add individual mods")
	}
	for _, existing := range s.Mods {
		if existing.Source == "modrinth" && existing.ModID == modIDOrSlug {
			return nil, fmt.Errorf("mod already added: %s", modIDOrSlug)
		}
	}
	loader := modrinth.NormalizeLoader(s.MCType)
	if loader == "" {
		return nil, fmt.Errorf("server type %s does not support mods", s.MCType)
	}
	client := modrinth.NewClient()
	versions, err := client.GetCompatibleVersions(modIDOrSlug, s.MCVersion, loader)
	if err != nil || len(versions) == 0 {
		return nil, fmt.Errorf("no compatible version for mod %s: %v", modIDOrSlug, err)
	}
	v := &versions[0]
	if version != "" {
		for i := range versions {
			if versions[i].ID == version || versions[i].VersionNumber == version {
				v = &versions[i]
				break
			}
		}
	}
	url := v.GetVersionDownloadURL()
	if url == "" {
		return nil, fmt.Errorf("no download URL for mod version")
	}
	mod := domain.Mod{
		ID:          "mod_" + uuid.New().String()[:8],
		Source:      "modrinth",
		ModID:       modIDOrSlug,
		ModName:     modIDOrSlug,
		VersionID:   v.ID,
		DownloadURL: url,
		MCVersions:  v.GameVersions,
		Loaders:     v.Loaders,
		Enabled:     true,
		AddedAt:     time.Now().UTC(),
	}
	s.Mods = append(s.Mods, mod)
	return &mod, st.SaveServer(s)
}

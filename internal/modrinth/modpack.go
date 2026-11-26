package modrinth

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

type ModpackRelease struct {
	Slug          string
	VersionID     string
	VersionNumber string
	MCVersion     string
	Loader        string
	FileURL       string
}

func (c *Client) ResolveModpack(slug, loader, mcVersion, packVersion string) (*ModpackRelease, error) {
	all, err := c.GetModpackVersions(slug, NormalizeLoader(loader))
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		if loader != "" {
			return nil, fmt.Errorf("modpack %q has no versions for %s", slug, loader)
		}
		return nil, fmt.Errorf("modpack %q has no versions", slug)
	}
	wantMC := mcVersion != "" && !strings.EqualFold(mcVersion, "latest")

	var chosen *ModpackVersion
	for i := range all {
		v := &all[i]
		switch {
		case packVersion != "":
			if v.ID == packVersion || v.VersionNumber == packVersion {
				chosen = v
			}
		case wantMC:
			if containsString(v.GameVersions, mcVersion) {
				chosen = v
			}
		case v.VersionType == "release":
			chosen = v
		}
		if chosen != nil {
			break
		}
	}
	if chosen == nil && packVersion == "" && !wantMC {
		chosen = &all[0]
	}
	if chosen == nil {
		if packVersion != "" {
			return nil, fmt.Errorf("modpack %q has no version %q", slug, packVersion)
		}
		return nil, fmt.Errorf("modpack %q has no version for Minecraft %s (it supports: %s)", slug, mcVersion, strings.Join(packGameVersions(all), ", "))
	}

	r := &ModpackRelease{Slug: slug, VersionID: chosen.ID, VersionNumber: chosen.VersionNumber}
	if wantMC {
		r.MCVersion = mcVersion
	} else if len(chosen.GameVersions) > 0 {
		r.MCVersion = chosen.GameVersions[0]
	}
	if r.MCVersion == "" {
		return nil, fmt.Errorf("modpack version %s lists no Minecraft version", chosen.ID)
	}
	r.Loader = loader
	if r.Loader == "" {
		for _, l := range chosen.Loaders {
			if NormalizeLoader(l) != "" {
				r.Loader = strings.ToLower(l)
				break
			}
		}
	}
	if r.Loader == "" {
		return nil, fmt.Errorf("modpack version %s uses no supported loader (%s)", chosen.ID, strings.Join(chosen.Loaders, ", "))
	}
	for _, f := range chosen.Files {
		if f.Primary || r.FileURL == "" {
			r.FileURL = f.URL
		}
	}
	return r, nil
}

func (c *Client) CountModpackMods(fileURL string) (int, error) {
	if fileURL == "" {
		return 0, fmt.Errorf("modpack has no file")
	}
	ra, err := newRangeReaderAt(c.http, fileURL)
	if err != nil {
		return 0, err
	}
	zr, err := zip.NewReader(ra, ra.size)
	if err != nil {
		return 0, fmt.Errorf("read modpack: %w", err)
	}
	return countZipServerMods(zr)
}

func countMrpackServerMods(data []byte) (int, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, err
	}
	return countZipServerMods(zr)
}

func countZipServerMods(zr *zip.Reader) (int, error) {
	count := 0
	for _, f := range zr.File {
		dir, name := path.Split(f.Name)
		if (dir == "overrides/mods/" || dir == "server-overrides/mods/") && strings.HasSuffix(name, ".jar") {
			count++
		}
		if f.Name != "modrinth.index.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return 0, err
		}
		var index struct {
			Files []struct {
				Path string `json:"path"`
				Env  struct {
					Server string `json:"server"`
				} `json:"env"`
			} `json:"files"`
		}
		err = json.NewDecoder(rc).Decode(&index)
		rc.Close()
		if err != nil {
			return 0, fmt.Errorf("read modrinth.index.json: %w", err)
		}
		for _, file := range index.Files {
			if strings.HasPrefix(file.Path, "mods/") && file.Env.Server != "unsupported" {
				count++
			}
		}
	}
	return count, nil
}

func packGameVersions(all []ModpackVersion) []string {
	var out []string
	for _, v := range all {
		for _, gv := range v.GameVersions {
			if !containsString(out, gv) {
				out = append(out, gv)
			}
		}
	}
	return out
}

func containsString(vs []string, v string) bool {
	for _, x := range vs {
		if x == v {
			return true
		}
	}
	return false
}

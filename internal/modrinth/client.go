package modrinth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const baseURL = "https://api.modrinth.com/v2"

// ModResult is a search result for a mod
type ModResult struct {
	Slug        string   `json:"slug"`
	ProjectID   string   `json:"project_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Versions    []string `json:"versions"`
}

// ModVersion is a specific version of a mod
type ModVersion struct {
	ID          string   `json:"id"`
	VersionNumber string `json:"version_number"`
	GameVersions []string `json:"game_versions"`
	Loaders     []string `json:"loaders"`
	Files       []struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Primary  bool   `json:"primary"`
	} `json:"files"`
}

// ModpackVersion is a version of a modpack (project type modpack) from /project/{id}/version
type ModpackVersion struct {
	ID             string   `json:"id"`
	VersionNumber  string   `json:"version_number"`
	GameVersions   []string `json:"game_versions"`
	Loaders        []string `json:"loaders"`
	VersionType    string   `json:"version_type"` // e.g. "release"
	Featured       bool     `json:"featured"`
	DatePublished  string   `json:"date_published"`
}

// Client for Modrinth API (no auth required for read)
type Client struct {
	http *http.Client
}

// NewClient returns a Modrinth API client
func NewClient() *Client {
	return &Client{http: &http.Client{}}
}

// SearchMods searches for mods by query. gameVersion and loader filter results (Modrinth facets).
func (c *Client) SearchMods(query, gameVersion, loader string) ([]ModResult, error) {
	u := baseURL + "/search?query=" + url.QueryEscape(query) + "&limit=20"
	var facets [][]string
	facets = append(facets, []string{"project_type:mod"})
	if gameVersion != "" {
		facets = append(facets, []string{"versions:" + gameVersion})
	}
	if loader != "" {
		facets = append(facets, []string{"categories:" + loader})
	}
	if len(facets) > 0 {
		facetsJSON, _ := json.Marshal(facets)
		u += "&facets=" + url.QueryEscape(string(facetsJSON))
	}
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("modrinth api: %s: %s", resp.Status, string(body))
	}
	var result struct {
		Hits []ModResult `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Hits, nil
}

// GetCompatibleVersions returns versions of a mod for the given game version and loader
func (c *Client) GetCompatibleVersions(modID, gameVersion, loader string) ([]ModVersion, error) {
	u := baseURL + "/project/" + url.PathEscape(modID) + "/version"
	u += "?game_versions=" + url.QueryEscape(gameVersion)
	if loader != "" {
		u += "&loaders=" + url.QueryEscape(`["`+loader+`"]`)
	}
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("modrinth api: %s: %s", resp.Status, string(body))
	}
	var versions []ModVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, err
	}
	return versions, nil
}

// GetModpackVersions returns versions for a modpack (project slug or id). Optional loader filter (e.g. "forge", "fabric").
func (c *Client) GetModpackVersions(packID, loader string) ([]ModpackVersion, error) {
	if packID == "" {
		return nil, fmt.Errorf("modpack id required")
	}
	u := baseURL + "/project/" + url.PathEscape(packID) + "/version?include_changelog=false"
	if loader != "" {
		u += "&loaders=" + url.QueryEscape(`["`+loader+`"]`)
	}
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("modrinth api: %s: %s", resp.Status, string(body))
	}
	var versions []ModpackVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, err
	}
	return versions, nil
}

// ModpackVersionInfo holds recommended MC version and modpack version ID from the modpack's latest release.
type ModpackVersionInfo struct {
	MCVersion       string // e.g. "1.12.2"
	ModpackVersionID string // version id for MODRINTH_MODPACK URL
}

// GetModpackRecommendedVersion returns the recommended Minecraft version and modpack version ID for a slug and loader.
// Uses the first (newest) modpack version from the API; loader is e.g. "forge" or "fabric".
func (c *Client) GetModpackRecommendedVersion(packID, loader string) (ModpackVersionInfo, error) {
	var out ModpackVersionInfo
	versions, err := c.GetModpackVersions(packID, loader)
	if err != nil {
		return out, err
	}
	if len(versions) == 0 {
		return out, fmt.Errorf("no versions found for modpack %q with loader %q", packID, loader)
	}
	v := versions[0]
	if len(v.GameVersions) == 0 {
		return out, fmt.Errorf("modpack version %s has no game_versions", v.ID)
	}
	out.MCVersion = v.GameVersions[0]
	out.ModpackVersionID = v.ID
	return out, nil
}

// ModpackSupportsVersion returns true if the modpack has any version supporting the given MC version and loader.
func (c *Client) ModpackSupportsVersion(packID, mcVersion, loader string) (bool, error) {
	versions, err := c.GetModpackVersions(packID, loader)
	if err != nil {
		return false, err
	}
	for _, v := range versions {
		for _, gv := range v.GameVersions {
			if gv == mcVersion {
				return true, nil
			}
		}
	}
	return false, nil
}

// ResolveModpackURL returns the Modrinth modpack URL for itzg image (project ID + version)
func (c *Client) ResolveModpackURL(packID, version string) (string, error) {
	if packID == "" {
		return "", fmt.Errorf("modpack id required")
	}
	// itzg image expects MODRINTH_MODPACK as project slug or project/version URL
	if version != "" {
		return "https://api.modrinth.com/v2/project/" + packID + "/version/" + version, nil
	}
	return "https://api.modrinth.com/v2/project/" + packID, nil
}

// GetVersionDownloadURL returns the primary file download URL for a version
func (v *ModVersion) GetVersionDownloadURL() string {
	for _, f := range v.Files {
		if f.Primary {
			return f.URL
		}
	}
	if len(v.Files) > 0 {
		return v.Files[0].URL
	}
	return ""
}

// NormalizeLoader maps server type to Modrinth loader slug
func NormalizeLoader(mcType string) string {
	switch strings.ToLower(mcType) {
	case "fabric":
		return "fabric"
	case "forge", "neoforge":
		return "forge"
	case "quilt":
		return "quilt"
	default:
		return ""
	}
}

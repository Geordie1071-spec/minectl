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

type ModResult struct {
	Slug        string   `json:"slug"`
	ProjectID   string   `json:"project_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Versions    []string `json:"versions"`
}

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

type ModpackVersion struct {
	ID             string   `json:"id"`
	VersionNumber  string   `json:"version_number"`
	GameVersions   []string `json:"game_versions"`
	Loaders        []string `json:"loaders"`
	VersionType    string   `json:"version_type"`
	Featured       bool     `json:"featured"`
	DatePublished  string   `json:"date_published"`
}

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{}}
}

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

type ModpackVersionInfo struct {
	MCVersion        string
	ModpackVersionID string
}

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

func (c *Client) ResolveModpackURL(packID, version string) (string, error) {
	if packID == "" {
		return "", fmt.Errorf("modpack id required")
	}
	if version != "" {
		return "https://api.modrinth.com/v2/project/" + packID + "/version/" + version, nil
	}
	return "https://api.modrinth.com/v2/project/" + packID, nil
}

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

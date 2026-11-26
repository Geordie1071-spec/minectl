package versions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/config"
)

const (
	mojangManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
	paperURL          = "https://fill.papermc.io/v3/projects/paper"
	fabricURL         = "https://meta.fabricmc.net/v2/versions/game"
	quiltURL          = "https://meta.quiltmc.org/v3/versions/game"
	forgeURL          = "https://files.minecraftforge.net/net/minecraftforge/forge/promotions_slim.json"
	neoforgeURL       = "https://maven.neoforged.net/api/maven/versions/releases/net/neoforged/neoforge"
)

var ErrUnchecked = errors.New("version list not available for this server type")

type VersionError struct {
	Requested  string
	Type       string
	DidYouMean string
	Available  []string
}

func (e *VersionError) Error() string {
	msg := fmt.Sprintf("Minecraft %s is not available for %s", e.Requested, e.Type)
	if e.DidYouMean != "" {
		msg += fmt.Sprintf("\n  Did you mean %s? Minecraft switched to year-based versions in 2026 (26.1, 26.2, ...).", e.DidYouMean)
	}
	if len(e.Available) > 0 {
		msg += "\n  Newest available: " + strings.Join(e.Available, ", ")
	}
	return msg
}

type Resolved struct {
	Version  string
	ImageTag string
}

type Client struct {
	http     *http.Client
	manifest *mojangManifest
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}}
}

type mojangManifest struct {
	Latest struct {
		Release string `json:"release"`
	} `json:"latest"`
	Versions []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"versions"`
}

func (c *Client) getJSON(url string, v any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "minectl/"+config.Version)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *Client) mojang() (*mojangManifest, error) {
	if c.manifest != nil {
		return c.manifest, nil
	}
	var m mojangManifest
	if err := c.getJSON(mojangManifestURL, &m); err != nil {
		return nil, err
	}
	c.manifest = &m
	return &m, nil
}

func (c *Client) Releases() ([]string, error) {
	m, err := c.mojang()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, v := range m.Versions {
		if v.Type == "release" {
			out = append(out, v.ID)
		}
	}
	return out, nil
}

func (c *Client) JavaMajor(version string) (int, error) {
	m, err := c.mojang()
	if err != nil {
		return 0, err
	}
	for _, v := range m.Versions {
		if v.ID != version {
			continue
		}
		var meta struct {
			JavaVersion struct {
				MajorVersion int `json:"majorVersion"`
			} `json:"javaVersion"`
		}
		if err := c.getJSON(v.URL, &meta); err != nil {
			return 0, err
		}
		if meta.JavaVersion.MajorVersion == 0 {
			return 8, nil
		}
		return meta.JavaVersion.MajorVersion, nil
	}
	return 0, fmt.Errorf("unknown Minecraft version %s", version)
}

func (c *Client) Supported(mcType string) ([]string, error) {
	switch strings.ToLower(mcType) {
	case "vanilla":
		return c.Releases()
	case "paper":
		return c.paper()
	case "fabric":
		return c.stableGameVersions(fabricURL)
	case "quilt":
		return c.stableGameVersions(quiltURL)
	case "forge":
		return c.forge()
	case "neoforge":
		return c.neoforge()
	}
	return nil, ErrUnchecked
}

func (c *Client) paper() ([]string, error) {
	var resp struct {
		Versions map[string][]string `json:"versions"`
	}
	if err := c.getJSON(paperURL, &resp); err != nil {
		return nil, err
	}
	var all []string
	for _, group := range resp.Versions {
		for _, v := range group {
			if isRelease(v) {
				all = append(all, v)
			}
		}
	}
	sortNewestFirst(all)
	return all, nil
}

func (c *Client) stableGameVersions(url string) ([]string, error) {
	var resp []struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	}
	if err := c.getJSON(url, &resp); err != nil {
		return nil, err
	}
	var out []string
	for _, v := range resp {
		if v.Stable {
			out = append(out, v.Version)
		}
	}
	return out, nil
}

func (c *Client) forge() ([]string, error) {
	var resp struct {
		Promos map[string]string `json:"promos"`
	}
	if err := c.getJSON(forgeURL, &resp); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for key := range resp.Promos {
		mc := strings.TrimSuffix(strings.TrimSuffix(key, "-latest"), "-recommended")
		if !seen[mc] && isRelease(mc) {
			seen[mc] = true
			out = append(out, mc)
		}
	}
	sortNewestFirst(out)
	return out, nil
}

func (c *Client) neoforge() ([]string, error) {
	var resp struct {
		Versions []string `json:"versions"`
	}
	if err := c.getJSON(neoforgeURL, &resp); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, nv := range resp.Versions {
		if strings.Contains(nv, "-") {
			continue
		}
		mc := NeoForgeToMinecraft(nv)
		if mc != "" && !seen[mc] {
			seen[mc] = true
			out = append(out, mc)
		}
	}
	sortNewestFirst(out)
	return out, nil
}

func NeoForgeToMinecraft(neo string) string {
	parts := strings.Split(neo, ".")
	if len(parts) < 3 {
		return ""
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return ""
	}
	if major >= 26 {
		if len(parts) < 4 {
			return ""
		}
		if parts[2] == "0" {
			return parts[0] + "." + parts[1]
		}
		return parts[0] + "." + parts[1] + "." + parts[2]
	}
	if parts[1] == "0" {
		return "1." + parts[0]
	}
	return "1." + parts[0] + "." + parts[1]
}

func (c *Client) Resolve(mcType, requested string) (*Resolved, error) {
	requested = strings.TrimSpace(requested)
	wantLatest := requested == "" || strings.EqualFold(requested, "latest")

	supported, err := c.Supported(mcType)
	if err != nil && !errors.Is(err, ErrUnchecked) {
		return nil, fmt.Errorf("check %s versions: %w", mcType, err)
	}
	if errors.Is(err, ErrUnchecked) {
		supported = nil
	}

	version := requested
	switch {
	case wantLatest && len(supported) > 0:
		version = supported[0]
	case wantLatest:
		m, err := c.mojang()
		if err != nil {
			return nil, err
		}
		version = m.Latest.Release
	case supported != nil && !contains(supported, requested):
		verr := &VersionError{Requested: requested, Type: mcType, Available: head(supported, 6)}
		if guess := yearVersionGuess(requested); guess != "" && contains(supported, guess) {
			verr.DidYouMean = guess
		}
		return nil, verr
	}

	tag := config.ImageTagForMCVersion(version, mcType)
	if major, err := c.JavaMajor(version); err == nil {
		tag = ImageTagForJava(major)
	}
	return &Resolved{Version: version, ImageTag: tag}, nil
}

func ImageTagForJava(major int) string {
	for _, v := range []int{8, 11, 16, 17, 21, 25} {
		if major <= v {
			return "java" + strconv.Itoa(v)
		}
	}
	return "latest"
}

func yearVersionGuess(v string) string {
	rest, ok := strings.CutPrefix(v, "1.")
	if !ok {
		return ""
	}
	major, err := strconv.Atoi(strings.SplitN(rest, ".", 2)[0])
	if err != nil || major < 26 {
		return ""
	}
	return rest
}

func isRelease(v string) bool {
	for _, r := range v {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return v != ""
}

func sortNewestFirst(vs []string) {
	sort.Slice(vs, func(i, j int) bool { return Compare(vs[i], vs[j]) > 0 })
}

func Compare(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x - y
		}
	}
	return 0
}

func contains(vs []string, v string) bool {
	for _, x := range vs {
		if x == v {
			return true
		}
	}
	return false
}

func head(vs []string, n int) []string {
	if len(vs) > n {
		return vs[:n]
	}
	return vs
}

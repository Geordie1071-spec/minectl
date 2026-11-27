// Package update finds new minectl releases on GitHub and replaces the
// running binary with one, after checking it against the release checksums.
package update

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const Repo = "Geordie1071-spec/minectl"

// API is the GitHub API base. MINECTL_UPDATE_API overrides it, for forks,
// mirrors and testing.
var API = envOr("MINECTL_UPDATE_API", "https://api.github.com")

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return strings.TrimRight(v, "/")
	}
	return def
}

var client = &http.Client{Timeout: 2 * time.Minute}

type Release struct {
	Tag string `json:"tag_name"`
	URL string `json:"html_url"`
	// Version is Tag without the leading "v".
	Version string `json:"-"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (r *Release) asset(name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

// ArchiveName is the release archive GoReleaser builds for this platform:
// a zip on Windows, a tarball elsewhere.
func (r *Release) ArchiveName() string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("minectl_%s_%s_%s.%s", r.Version, runtime.GOOS, runtime.GOARCH, ext)
}

// Latest returns the newest release, or the one tagged tag if it's set.
func Latest(ctx context.Context, tag string) (*Release, error) {
	path := "/repos/" + Repo + "/releases/latest"
	if tag != "" {
		if !strings.HasPrefix(tag, "v") {
			tag = "v" + tag
		}
		path = "/repos/" + Repo + "/releases/tags/" + tag
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, API+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("couldn't reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound && tag != "":
		return nil, fmt.Errorf("there's no release %s", tag)
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, errors.New("GitHub is rate-limiting this machine; try again in an hour")
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var r Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&r); err != nil {
		return nil, err
	}
	if r.Tag == "" {
		return nil, errors.New("GitHub returned a release without a tag")
	}
	r.Version = strings.TrimPrefix(r.Tag, "v")
	return &r, nil
}

type version struct {
	nums [3]int
	pre  string
}

func parse(v string) (version, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	core, pre, _ := strings.Cut(v, "-")
	core, _, _ = strings.Cut(core, "+")
	parts := strings.Split(core, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return version{}, false
	}
	var out version
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return version{}, false
		}
		out.nums[i] = n
	}
	out.pre = pre
	return out, true
}

// Compare orders two versions like semver: -1, 0 or 1. A pre-release such as
// "2.0.0-dev" comes before "2.0.0". Unparseable versions compare as equal.
func Compare(a, b string) int {
	va, okA := parse(a)
	vb, okB := parse(b)
	if !okA || !okB {
		return 0
	}
	for i := range va.nums {
		if va.nums[i] != vb.nums[i] {
			if va.nums[i] < vb.nums[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case va.pre == vb.pre:
		return 0
	case va.pre == "":
		return 1
	case vb.pre == "":
		return -1
	case va.pre < vb.pre:
		return -1
	}
	return 1
}

// Executable is the path of the running binary, with symlinks resolved.
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// CanReplace reports whether this process may replace the binary at exe.
func CanReplace(exe string) bool {
	f, err := os.CreateTemp(filepath.Dir(exe), ".minectl-update-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

func download(ctx context.Context, url string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	_, err = io.Copy(w, io.LimitReader(resp.Body, 200<<20))
	return err
}

// checksum finds name's SHA-256 in a GoReleaser checksums.txt.
func checksum(sums io.Reader, name string) (string, error) {
	sc := bufio.NewScanner(sums)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", name)
}

// Apply downloads r for this platform, checks it against the release's
// checksums.txt, and replaces exe with it. exe is swapped by rename, so a
// failure leaves the old binary in place.
func Apply(ctx context.Context, r *Release, exe string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("Windows can't replace a running program; close minectl and run the installer again:\n  irm https://raw.githubusercontent.com/Geordie1071-spec/minectl/main/scripts/install.ps1 | iex\nor download %s from %s", r.ArchiveName(), r.URL)
	}
	name := r.ArchiveName()
	archiveURL, sumsURL := r.asset(name), r.asset("checksums.txt")
	if archiveURL == "" {
		return fmt.Errorf("release %s has no build for %s/%s", r.Tag, runtime.GOOS, runtime.GOARCH)
	}
	if sumsURL == "" {
		return fmt.Errorf("release %s has no checksums.txt, so the download can't be verified", r.Tag)
	}
	var sums strings.Builder
	if err := download(ctx, sumsURL, &sums); err != nil {
		return err
	}
	want, err := checksum(strings.NewReader(sums.String()), name)
	if err != nil {
		return err
	}

	archive, err := os.CreateTemp("", "minectl-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	h := sha256.New()
	if err := download(ctx, archiveURL, io.MultiWriter(archive, h)); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("the download doesn't match the release checksum (got %s, want %s)", got, want)
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Write the new binary next to the old one so the rename is atomic.
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".minectl-update-*")
	if err != nil {
		return fmt.Errorf("can't write to %s: %w", filepath.Dir(exe), err)
	}
	defer os.Remove(tmp.Name())
	if err := extract(archive, tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		return err
	}
	// Keep the old binary's owner, so a root-run update of a binary the
	// panel's user owns doesn't take the panel's update button away.
	if fi, err := os.Stat(exe); err == nil {
		chownLike(tmp.Name(), fi)
	}
	return os.Rename(tmp.Name(), exe)
}

func extract(r io.Reader, w io.Writer) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return errors.New("the release archive has no minectl binary")
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == "minectl" {
			_, err = io.Copy(w, io.LimitReader(tr, 200<<20))
			return err
		}
	}
}

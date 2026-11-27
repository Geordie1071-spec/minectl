package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2.0.0", "2.0.0", 0},
		{"v2.0.0", "2.0.0", 0},
		{"2.0.0-dev", "2.0.0", -1},
		{"2.0.0", "2.0.0-dev", 1},
		{"2.0.0", "2.0.1", -1},
		{"2.10.0", "2.9.9", 1},
		{"1.0", "2.0.0", -1},
		{"2.1.0-rc.1", "2.1.0-rc.2", -1},
		{"dev", "2.0.0", 0},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func tarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name string
		body []byte
	}{{"README.md", []byte("hi")}, {name, body}} {
		_ = tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0755, Size: int64(len(f.body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write(f.body)
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

// fakeGitHub serves a release whose archive holds newBinary. If badSum is
// set, checksums.txt lists the wrong hash.
func fakeGitHub(t *testing.T, newBinary []byte, badSum bool) *Release {
	t.Helper()
	var srv *httptest.Server
	r := &Release{Tag: "v9.9.9", Version: "9.9.9"}
	archive := tarGz(t, "minectl", newBinary)
	sum := sha256.Sum256(archive)
	hexSum := hex.EncodeToString(sum[:])
	if badSum {
		hexSum = strings.Repeat("0", 64)
	}
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/repos/" + Repo + "/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v9.9.9", "html_url": "https://example/rel",
				"assets": []map[string]string{
					{"name": r.ArchiveName(), "browser_download_url": srv.URL + "/archive"},
					{"name": "checksums.txt", "browser_download_url": srv.URL + "/sums"},
				},
			})
		case "/repos/" + Repo + "/releases/tags/v1.2.3":
			http.NotFound(w, req)
		case "/archive":
			_, _ = w.Write(archive)
		case "/sums":
			_, _ = w.Write([]byte("deadbeef  minectl_9.9.9_other_arch.tar.gz\n" + hexSum + "  " + r.ArchiveName() + "\n"))
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(srv.Close)
	old := API
	API = srv.URL
	t.Cleanup(func() { API = old })
	rel, err := Latest(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

func TestApply(t *testing.T) {
	rel := fakeGitHub(t, []byte("#!new minectl"), false)
	if rel.Version != "9.9.9" || rel.URL != "https://example/rel" {
		t.Fatalf("release = %+v", rel)
	}
	if _, err := Latest(context.Background(), "1.2.3"); err == nil || !strings.Contains(err.Error(), "no release v1.2.3") {
		t.Fatalf("missing tag: %v", err)
	}
	exe := filepath.Join(t.TempDir(), "minectl")
	_ = os.WriteFile(exe, []byte("#!old minectl"), 0755)
	if !CanReplace(exe) {
		t.Fatal("temp dir should be writable")
	}
	if err := Apply(context.Background(), rel, exe); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(exe)
	fi, _ := os.Stat(exe)
	if string(got) != "#!new minectl" || fi.Mode().Perm() != 0755 {
		t.Fatalf("binary = %q mode %v", got, fi.Mode())
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(exe), ".minectl-update-*")); len(left) != 0 {
		t.Fatalf("left temp files: %v", left)
	}
}

func TestApplyRejectsBadChecksum(t *testing.T) {
	rel := fakeGitHub(t, []byte("#!evil"), true)
	exe := filepath.Join(t.TempDir(), "minectl")
	_ = os.WriteFile(exe, []byte("#!old minectl"), 0755)
	if err := Apply(context.Background(), rel, exe); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "#!old minectl" {
		t.Fatalf("binary replaced despite bad checksum: %q", got)
	}
}

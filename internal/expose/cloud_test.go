package expose

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeDMI(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := dmiDir
	dmiDir = dir
	t.Cleanup(func() { dmiDir = old })
}

func TestCloudFromDMI(t *testing.T) {
	for want, files := range map[string]map[string]string{
		"aws":          {"sys_vendor": "Amazon EC2"},
		"gcp":          {"product_name": "Google Compute Engine"},
		"azure":        {"sys_vendor": "Microsoft Corporation", "chassis_asset_tag": "7783-7084-3265-9085"},
		"oracle":       {"chassis_asset_tag": "OracleCloud.com"},
		"hetzner":      {"sys_vendor": "Hetzner", "product_name": "vServer"},
		"digitalocean": {"sys_vendor": "DigitalOcean", "product_name": "Droplet"},
		"vultr":        {"sys_vendor": "Vultr"},
		"":             {"sys_vendor": "LENOVO", "product_name": "ThinkPad"},
	} {
		fakeDMI(t, files)
		if got := cloudFromDMI(); got != want {
			t.Errorf("%v: got %q, want %q", files, got, want)
		}
	}
}

// fakeMetadata serves the metadata paths each provider answers.
func fakeMetadata(t *testing.T, echo string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/latest/api/token" && r.Method == http.MethodPut:
			w.Write([]byte("tok"))
		case r.URL.Path == "/latest/meta-data/public-ipv4" && r.Header.Get("X-aws-ec2-metadata-token") == "tok":
			w.Write([]byte("203.0.113.1"))
		case strings.HasPrefix(r.URL.Path, "/computeMetadata/") && r.Header.Get("Metadata-Flavor") == "Google":
			w.Header().Set("Metadata-Flavor", "Google")
			w.Write([]byte("203.0.113.2"))
		case strings.HasPrefix(r.URL.Path, "/metadata/instance/") && r.Header.Get("Metadata") == "true":
			w.Write([]byte("")) // a standard SKU public IP: Azure doesn't say
		case r.URL.Path == "/echo":
			w.Write([]byte(echo))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	oldBase, oldEcho := metadataBase, ipEchoURL
	metadataBase, ipEchoURL = srv.URL, srv.URL+"/echo"
	t.Cleanup(func() { metadataBase, ipEchoURL = oldBase, oldEcho })
}

func TestDetectCloud(t *testing.T) {
	fakeMetadata(t, "203.0.113.9\n")
	for _, tc := range []struct {
		dmi      map[string]string
		provider string
		ip       string
	}{
		{map[string]string{"sys_vendor": "Amazon EC2"}, "aws", "203.0.113.1"},
		{map[string]string{"product_name": "Google Compute Engine"}, "gcp", "203.0.113.2"},
		{map[string]string{"chassis_asset_tag": "7783-7084-3265-9085"}, "azure", "203.0.113.9"},
		{map[string]string{"chassis_asset_tag": "OracleCloud.com"}, "oracle", "203.0.113.9"},
	} {
		fakeDMI(t, tc.dmi)
		c := detectCloud(context.Background())
		if c == nil || c.Provider != tc.provider || c.IP.String() != tc.ip {
			t.Errorf("%s: got %+v", tc.provider, c)
		}
	}

	// Providers that give the machine its public IP need no metadata request
	// (fakeMetadata's cleanup restores these).
	metadataBase, ipEchoURL = "http://192.0.2.1", "http://192.0.2.1"
	fakeDMI(t, map[string]string{"sys_vendor": "Hetzner"})
	if c := detectCloud(context.Background()); c == nil || c.Provider != "hetzner" || c.Name != "Hetzner" {
		t.Errorf("hetzner: got %+v", c)
	}

	fakeDMI(t, map[string]string{"sys_vendor": "LENOVO"})
	if c := detectCloud(context.Background()); c != nil {
		t.Errorf("a home machine: got %+v", c)
	}
}

func TestDetectCloudNoPublicIP(t *testing.T) {
	// An echo service seeing a private address means there's no public one.
	fakeMetadata(t, "10.0.0.4")
	fakeDMI(t, map[string]string{"chassis_asset_tag": "OracleCloud.com"})
	if c := detectCloud(context.Background()); c == nil || c.IP != nil {
		t.Fatalf("got %+v", c)
	}
}

func TestCloudFirewall(t *testing.T) {
	var none *Cloud
	if got := none.Firewall("25565"); got != "open TCP 25565 in your cloud provider's firewall" {
		t.Errorf("no cloud: %q", got)
	}
	if none.Tip() != "" {
		t.Error("no cloud, no tip")
	}
	aws := &Cloud{Provider: "aws"}
	if got := aws.Firewall("80 and 443"); !strings.Contains(got, "open TCP 80 and 443 in the instance's security group") {
		t.Errorf("aws: %q", got)
	}
	if !strings.Contains(aws.Tip(), "Elastic IP") {
		t.Errorf("aws tip: %q", aws.Tip())
	}
	for key, p := range providers {
		if p.name == "" || p.firewall == "" {
			t.Errorf("%s: missing name or firewall", key)
		}
	}
}

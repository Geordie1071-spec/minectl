package modrinth

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestCountMrpackServerMods(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("modrinth.index.json")
	_, _ = w.Write([]byte(`{"files":[
		{"path":"mods/a.jar","env":{"server":"required"}},
		{"path":"mods/b.jar","env":{"server":"optional"}},
		{"path":"mods/client-only.jar","env":{"server":"unsupported"}},
		{"path":"mods/c.jar"},
		{"path":"resourcepacks/pack.zip","env":{"server":"required"}}
	]}`))
	_, _ = zw.Create("overrides/mods/bundled.jar")
	_, _ = zw.Create("overrides/config/x.toml")
	_, _ = zw.Create("server-overrides/mods/server-only.jar")
	_ = zw.Close()

	n, err := countMrpackServerMods(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("count = %d, want 5", n)
	}
}

func TestNormalizeLoaderKeepsNeoForge(t *testing.T) {
	if got := NormalizeLoader("neoforge"); got != "neoforge" {
		t.Errorf("NormalizeLoader(neoforge) = %q", got)
	}
	if got := NormalizeLoader("paper"); got != "" {
		t.Errorf("NormalizeLoader(paper) = %q", got)
	}
}

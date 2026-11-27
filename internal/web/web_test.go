package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandlerWithoutBuild(t *testing.T) {
	// The repo only checks in dist/.keep, so a plain `go build` has no UI.
	if _, err := dist.ReadFile("dist/index.html"); err != nil && Handler() != nil {
		t.Fatal("Handler should be nil without a build")
	}
}

func TestSPAFallback(t *testing.T) {
	mem := fstest.MapFS{
		"dist/index.html":      {Data: []byte("<div id=root>")},
		"dist/assets/app-1.js": {Data: []byte("console.log(1)")},
		"dist/favicon.svg":     {Data: []byte("<svg/>")},
	}
	h := handlerFS(mem)
	for path, want := range map[string]string{
		"/":                 "<div id=root>",
		"/servers/survival": "<div id=root>",
		"/assets/app-1.js":  "console.log(1)",
		"/favicon.svg":      "<svg/>",
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), want) {
			t.Errorf("%s: %d %q", path, w.Code, w.Body)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/assets/missing.js", nil))
	if w.Code != 404 {
		t.Errorf("missing asset: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/assets/app-1.js", nil))
	if !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Error("hashed assets should be cached")
	}
}

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/auth"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/jobs"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

const setupTok = "test-setup-token"

type harness struct {
	t  *testing.T
	a  *API
	st *store.Store
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	t.Setenv("MINECTL_CONFIG_DIR", t.TempDir())
	st := store.New()
	t.Cleanup(func() { st.Close() })
	runner, err := jobs.New(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	a := New(Config{Manager: &service.Manager{St: st}, Store: st, Jobs: runner, SetupToken: setupTok})
	return &harness{t: t, a: a, st: st}
}

type reqOpt func(*http.Request)

func withCookie(v string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: sessionCookie, Value: v}) }
}

func withToken(v string) reqOpt {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+v) }
}

func withHeader(k, v string) reqOpt {
	return func(r *http.Request) { r.Header.Set(k, v) }
}

func (h *harness) do(method, path, body string, opts ...reqOpt) *httptest.ResponseRecorder {
	h.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "203.0.113.5:1234"
	for _, o := range opts {
		o(r)
	}
	w := httptest.NewRecorder()
	h.a.ServeHTTP(w, r)
	return w
}

func sessionFrom(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
				t.Fatalf("session cookie flags wrong: %+v", c)
			}
			return c.Value
		}
	}
	t.Fatalf("no session cookie (status %d: %s)", w.Code, w.Body)
	return ""
}

func (h *harness) setupAdmin() string {
	h.t.Helper()
	w := h.do("POST", "/api/v1/setup", `{"token":"`+setupTok+`","username":"owner","password":"hunter22hunter"}`)
	if w.Code != http.StatusOK {
		h.t.Fatalf("setup: %d %s", w.Code, w.Body)
	}
	return sessionFrom(h.t, w)
}

func (h *harness) userToken(name string, isAdmin bool) (*domain.User, string) {
	h.t.Helper()
	hash, _ := auth.HashPassword("password123")
	u, err := h.st.CreateUser(name, hash, isAdmin)
	if err != nil {
		h.t.Fatal(err)
	}
	tok := auth.NewAPIToken()
	if _, err := h.st.CreateAPIToken(u.ID, "test", auth.HashToken(tok)); err != nil {
		h.t.Fatal(err)
	}
	return u, tok
}

func TestSetupFlow(t *testing.T) {
	h := newHarness(t)
	if w := h.do("GET", "/api/v1/setup", ""); !strings.Contains(w.Body.String(), `"needed":true`) {
		t.Fatalf("setup should be needed: %s", w.Body)
	}
	if w := h.do("GET", "/api/v1/servers", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("servers before login: %d", w.Code)
	}
	if w := h.do("POST", "/api/v1/setup", `{"token":"wrong","username":"owner","password":"hunter22hunter"}`); w.Code != http.StatusForbidden {
		t.Fatalf("wrong token: %d", w.Code)
	}
	if w := h.do("POST", "/api/v1/setup", `{"token":"`+setupTok+`","username":"owner","password":"short"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("short password: %d", w.Code)
	}
	sess := h.setupAdmin()
	if w := h.do("GET", "/api/v1/auth/me", "", withCookie(sess)); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"is_admin":true`) {
		t.Fatalf("me after setup: %d %s", w.Code, w.Body)
	}
	if w := h.do("POST", "/api/v1/setup", `{"token":"`+setupTok+`","username":"again","password":"hunter22hunter"}`); w.Code != http.StatusConflict {
		t.Fatalf("second setup: %d", w.Code)
	}
	if w := h.do("GET", "/api/v1/setup", ""); !strings.Contains(w.Body.String(), `"needed":false`) {
		t.Fatalf("setup should be done: %s", w.Body)
	}
}

func TestLoginLogoutAndRateLimit(t *testing.T) {
	h := newHarness(t)
	h.setupAdmin()
	if w := h.do("POST", "/api/v1/auth/login", `{"username":"owner","password":"nope-nope"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", w.Code)
	}
	if w := h.do("POST", "/api/v1/auth/login", `{"username":"ghost","password":"nope-nope"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown user: %d", w.Code)
	}
	w := h.do("POST", "/api/v1/auth/login", `{"username":"OWNER","password":"hunter22hunter"}`)
	sess := sessionFrom(t, w)
	if w := h.do("POST", "/api/v1/auth/logout", "", withCookie(sess)); w.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", w.Code)
	}
	if w := h.do("GET", "/api/v1/auth/me", "", withCookie(sess)); w.Code != http.StatusUnauthorized {
		t.Fatalf("session should be gone after logout: %d", w.Code)
	}

	for i := 0; i < 10; i++ {
		h.do("POST", "/api/v1/auth/login", `{"username":"owner","password":"guess"}`)
	}
	w = h.do("POST", "/api/v1/auth/login", `{"username":"owner","password":"hunter22hunter"}`)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("want 429 after repeated failures, got %d", w.Code)
	}
	if entries, _ := h.st.ListAudit(50, 0); len(entries) < 10 {
		t.Fatalf("failed logins should be audited, got %d entries", len(entries))
	}
}

func TestCrossOriginRefused(t *testing.T) {
	h := newHarness(t)
	sess := h.setupAdmin()
	body := `{"username":"friend","password":"password123"}`
	if w := h.do("POST", "/api/v1/users", body, withCookie(sess), withHeader("Origin", "https://evil.example")); w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin with Origin: %d", w.Code)
	}
	if w := h.do("POST", "/api/v1/users", body, withCookie(sess), withHeader("Sec-Fetch-Site", "cross-site")); w.Code != http.StatusForbidden {
		t.Fatalf("cross-site without Origin: %d", w.Code)
	}
	if w := h.do("POST", "/api/v1/auth/login", `{"username":"owner","password":"hunter22hunter"}`, withHeader("Origin", "https://evil.example")); w.Code != http.StatusForbidden {
		t.Fatalf("login CSRF: %d", w.Code)
	}
	if w := h.do("POST", "/api/v1/users", body, withCookie(sess), withHeader("Origin", "http://example.com")); w.Code != http.StatusCreated {
		t.Fatalf("same-origin create: %d %s", w.Code, w.Body)
	}
}

func TestAPITokens(t *testing.T) {
	h := newHarness(t)
	sess := h.setupAdmin()
	w := h.do("POST", "/api/v1/tokens", `{"name":"discord bot"}`, withCookie(sess))
	if w.Code != http.StatusCreated {
		t.Fatalf("create token: %d %s", w.Code, w.Body)
	}
	var created struct {
		Token string          `json:"token"`
		Info  domain.APIToken `json:"info"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if !strings.HasPrefix(created.Token, auth.APITokenPrefix) {
		t.Fatalf("token = %q", created.Token)
	}
	if w := h.do("POST", "/api/v1/users", `{"username":"viaapi","password":"password123"}`, withToken(created.Token), withHeader("Origin", "https://elsewhere.example")); w.Code != http.StatusCreated {
		t.Fatalf("token requests aren't subject to the origin check: %d %s", w.Code, w.Body)
	}
	if w := h.do("GET", "/api/v1/tokens", "", withCookie(sess)); strings.Contains(w.Body.String(), created.Token) {
		t.Fatal("token list must not reveal tokens")
	}
	if w := h.do("DELETE", "/api/v1/tokens/"+itoa(created.Info.ID), "", withCookie(sess)); w.Code != http.StatusNoContent {
		t.Fatalf("delete token: %d", w.Code)
	}
	if w := h.do("GET", "/api/v1/auth/me", "", withToken(created.Token)); w.Code != http.StatusUnauthorized {
		t.Fatalf("deleted token still works: %d", w.Code)
	}
	if w := h.do("GET", "/api/v1/auth/me", "", withToken("not-a-token")); w.Code != http.StatusUnauthorized {
		t.Fatalf("garbage token: %d", w.Code)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// TestPermissions checks every route against every kind of caller.
func TestPermissions(t *testing.T) {
	h := newHarness(t)
	h.setupAdmin()
	for _, name := range []string{"alpha", "beta"} {
		if err := h.st.SaveServer(&domain.Server{Name: name, Status: domain.StatusStopped}); err != nil {
			t.Fatal(err)
		}
	}
	_, adminTok := h.userToken("admin2", true)
	op, opTok := h.userToken("op", false)
	viewer, viewerTok := h.userToken("viewer", false)
	_, strangerTok := h.userToken("stranger", false)
	_ = h.st.SetGrant(op.ID, "alpha", domain.RoleOperator)
	_ = h.st.SetGrant(viewer.ID, "alpha", domain.RoleViewer)

	// These reach the internet or need a specific body to pass validation.
	skipAllowed := map[string]bool{
		"GET /api/v1/versions/{type}": true,
		"GET /api/v1/modpacks/search": true,
		"POST /api/v1/auth/password":  true,
		"POST /api/v1/setup":          true,
		"POST /api/v1/auth/login":     true,
	}
	fill := strings.NewReplacer("{id}", "999", "{server}", "alpha", "{slug}", "sodium", "{player}", "Steve", "{action}", "op", "{type}", "paper")

	refusedPerm := func(w *httptest.ResponseRecorder) bool {
		return w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden ||
			(w.Code == http.StatusNotFound && strings.Contains(w.Body.String(), "server not found"))
	}

	for _, rt := range h.a.routes() {
		method, pattern, _ := strings.Cut(rt.pattern, " ")
		for _, srv := range []string{"alpha", "beta"} {
			path := fill.Replace(strings.ReplaceAll(pattern, "{name}", srv))
			if srv == "beta" && !strings.Contains(pattern, "{name}") {
				continue
			}
			callers := []struct {
				who     string
				opts    []reqOpt
				allowed bool
			}{
				{"anonymous", nil, rt.level == public},
				{"admin", []reqOpt{withToken(adminTok)}, true},
				{"operator", []reqOpt{withToken(opTok)}, allowedFor(rt.level, srv == "alpha", domain.RoleOperator)},
				{"viewer", []reqOpt{withToken(viewerTok)}, allowedFor(rt.level, srv == "alpha", domain.RoleViewer)},
				{"stranger", []reqOpt{withToken(strangerTok)}, allowedFor(rt.level, false, "")},
			}
			for _, c := range callers {
				if c.allowed && skipAllowed[rt.pattern] {
					continue
				}
				w := h.do(method, path, "", c.opts...)
				if refusedPerm(w) == c.allowed {
					t.Errorf("%s %s as %s: got %d %s, want allowed=%v", method, path, c.who, w.Code, strings.TrimSpace(w.Body.String()), c.allowed)
				}
				if !c.allowed && rt.level >= serverViewer && c.who != "anonymous" {
					hasAccess := srv == "alpha" && (c.who == "operator" || c.who == "viewer")
					if !hasAccess && w.Code != http.StatusNotFound {
						t.Errorf("%s %s as %s: callers without access should get 404, got %d", method, path, c.who, w.Code)
					}
				}
			}
		}
	}
}

func allowedFor(l level, hasGrant bool, role string) bool {
	switch l {
	case public, user:
		return true
	case admin:
		return false
	}
	if !hasGrant {
		return false
	}
	need := map[level]string{serverViewer: domain.RoleViewer, serverOperator: domain.RoleOperator, serverAdmin: domain.RoleAdmin}[l]
	return roleRank[role] >= roleRank[need]
}

func TestServerListFilteredByGrant(t *testing.T) {
	h := newHarness(t)
	sess := h.setupAdmin()
	for _, name := range []string{"alpha", "beta"} {
		_ = h.st.SaveServer(&domain.Server{Name: name, Status: domain.StatusStopped, DataDir: "/srv/" + name})
	}
	viewer, viewerTok := h.userToken("viewer", false)
	if w := h.do("PUT", "/api/v1/users/"+itoa(viewer.ID)+"/grants/beta", `{"role":"viewer"}`, withCookie(sess)); w.Code != http.StatusOK {
		t.Fatalf("grant: %d %s", w.Code, w.Body)
	}
	if w := h.do("PUT", "/api/v1/users/"+itoa(viewer.ID)+"/grants/beta", `{"role":"admin"}`, withCookie(sess)); w.Code != http.StatusBadRequest {
		t.Fatalf("admin grant should be refused: %d", w.Code)
	}
	if w := h.do("PUT", "/api/v1/users/"+itoa(viewer.ID)+"/grants/nope", `{"role":"viewer"}`, withCookie(sess)); w.Code != http.StatusNotFound {
		t.Fatalf("grant on missing server: %d", w.Code)
	}
	w := h.do("GET", "/api/v1/servers", "", withToken(viewerTok))
	var list []serverView
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Name != "beta" || list[0].Role != domain.RoleViewer || list[0].DataDir != "" {
		t.Fatalf("viewer list = %+v", list)
	}
	w = h.do("GET", "/api/v1/servers", "", withCookie(sess))
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 2 || list[0].DataDir == "" {
		t.Fatalf("admin list = %+v", list)
	}
}

func TestUserAdminGuards(t *testing.T) {
	h := newHarness(t)
	sess := h.setupAdmin()
	if w := h.do("PATCH", "/api/v1/users/1", `{"is_admin":false}`, withCookie(sess)); w.Code != http.StatusConflict {
		t.Fatalf("demoting the last admin: %d", w.Code)
	}
	if w := h.do("DELETE", "/api/v1/users/1", "", withCookie(sess)); w.Code != http.StatusConflict {
		t.Fatalf("deleting yourself: %d", w.Code)
	}
	if w := h.do("POST", "/api/v1/users", `{"username":"Owner","password":"password123"}`, withCookie(sess)); w.Code != http.StatusConflict {
		t.Fatalf("duplicate username: %d", w.Code)
	}
	friend, friendTok := h.userToken("friend", false)
	if w := h.do("PATCH", "/api/v1/users/"+itoa(friend.ID), `{"password":"brandnewpass"}`, withCookie(sess)); w.Code != http.StatusOK {
		t.Fatalf("reset password: %d %s", w.Code, w.Body)
	}
	if w := h.do("POST", "/api/v1/auth/login", `{"username":"friend","password":"brandnewpass"}`); w.Code != http.StatusOK {
		t.Fatalf("login with reset password: %d", w.Code)
	}
	if w := h.do("DELETE", "/api/v1/users/"+itoa(friend.ID), "", withCookie(sess)); w.Code != http.StatusNoContent {
		t.Fatalf("delete user: %d", w.Code)
	}
	if w := h.do("GET", "/api/v1/auth/me", "", withToken(friendTok)); w.Code != http.StatusUnauthorized {
		t.Fatalf("deleted user's token still works: %d", w.Code)
	}
}

func TestChangePassword(t *testing.T) {
	h := newHarness(t)
	sess := h.setupAdmin()
	other := sessionFrom(t, h.do("POST", "/api/v1/auth/login", `{"username":"owner","password":"hunter22hunter"}`))
	if w := h.do("POST", "/api/v1/auth/password", `{"current":"wrong","new":"anotherpass1"}`, withCookie(sess)); w.Code != http.StatusForbidden {
		t.Fatalf("wrong current password: %d", w.Code)
	}
	if w := h.do("POST", "/api/v1/auth/password", `{"current":"hunter22hunter","new":"anotherpass1"}`, withCookie(sess)); w.Code != http.StatusNoContent {
		t.Fatalf("change password: %d %s", w.Code, w.Body)
	}
	if w := h.do("GET", "/api/v1/auth/me", "", withCookie(sess)); w.Code != http.StatusOK {
		t.Fatal("current session should survive a password change")
	}
	if w := h.do("GET", "/api/v1/auth/me", "", withCookie(other)); w.Code != http.StatusUnauthorized {
		t.Fatal("other sessions should be logged out")
	}
}

func TestJobVisibilityAndReplay(t *testing.T) {
	h := newHarness(t)
	sess := h.setupAdmin()
	_ = h.st.SaveServer(&domain.Server{Name: "alpha"})
	_ = h.st.SaveServer(&domain.Server{Name: "beta"})
	viewer, viewerTok := h.userToken("viewer", false)
	_ = h.st.SetGrant(viewer.ID, "alpha", domain.RoleViewer)

	now := time.Now()
	for _, srv := range []string{"alpha", "beta"} {
		j := &domain.Job{ID: "job-" + srv, Kind: "start", Server: srv, Status: domain.JobRunning, CreatedAt: now}
		_ = h.st.InsertJob(j)
		j.Status, j.FinishedAt = domain.JobSucceeded, &now
		j.Events = []domain.JobEvent{{Seq: 1, Type: "info", Message: "Ready in 9s"}, {Seq: 2, Type: "done", Message: "running"}}
		_ = h.st.FinishJob(j)
	}
	if w := h.do("GET", "/api/v1/jobs/job-beta", "", withToken(viewerTok)); w.Code != http.StatusNotFound {
		t.Fatalf("job on a server without access: %d", w.Code)
	}
	if w := h.do("GET", "/api/v1/jobs/job-alpha", "", withToken(viewerTok)); w.Code != http.StatusOK {
		t.Fatalf("job on own server: %d", w.Code)
	}
	w := h.do("GET", "/api/v1/jobs/job-beta/events", "", withCookie(sess), withHeader("Last-Event-ID", "1"))
	body := w.Body.String()
	if w.Header().Get("Content-Type") != "text/event-stream" || strings.Contains(body, "Ready in 9s") ||
		!strings.Contains(body, "id: 2\nevent: done") || !strings.Contains(body, "event: end") {
		t.Fatalf("replay after event 1:\n%s", body)
	}
}

func TestInputValidation(t *testing.T) {
	h := newHarness(t)
	sess := h.setupAdmin()
	_ = h.st.SaveServer(&domain.Server{Name: "alpha", Status: domain.StatusRunning, ContainerID: "abc"})
	cases := []struct{ method, path, body string }{
		{"POST", "/api/v1/servers", `{"name":"Bad Name"}`},
		{"POST", "/api/v1/servers", `{"name":"ok","type":"bukkit"}`},
		{"POST", "/api/v1/servers", `{"name":"ok","port":80}`},
		{"POST", "/api/v1/servers", `{"name":"ok","memory":"lots"}`},
		{"POST", "/api/v1/servers", `{"name":"ok","modpack":"a","curseforge":"b"}`},
		{"POST", "/api/v1/servers", `{"name":"ok","data_dir":"/etc"}`},
		{"POST", "/api/v1/servers/alpha/console", `{"command":"say hi\nstop"}`},
		{"POST", "/api/v1/servers/alpha/console", `{"command":""}`},
		{"POST", "/api/v1/servers/alpha/players/bad;name/op", ``},
		{"POST", "/api/v1/servers/alpha/players/Steve/explode", ``},
	}
	for _, c := range cases {
		if w := h.do(c.method, c.path, c.body, withCookie(sess)); w.Code != http.StatusBadRequest {
			t.Errorf("%s %s %s: got %d %s", c.method, c.path, c.body, w.Code, w.Body)
		}
	}
	if w := h.do("POST", "/api/v1/servers", `{"name":"alpha"}`, withCookie(sess)); w.Code != http.StatusConflict {
		t.Errorf("duplicate server: %d", w.Code)
	}
	big := `{"name":"` + strings.Repeat("a", maxBodyBytes) + `"}`
	if w := h.do("POST", "/api/v1/servers", big, withCookie(sess)); w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("huge body: %d", w.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := newHarness(t)
	w := h.do("GET", "/api/v1/health", "")
	for _, k := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy"} {
		if w.Header().Get(k) == "" {
			t.Errorf("missing %s", k)
		}
	}
	if w := h.do("GET", "/api/v1/nope", ""); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "error") {
		t.Errorf("unknown endpoint: %d %s", w.Code, w.Body)
	}
}

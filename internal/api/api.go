// Package api serves the panel's JSON API under /api/v1.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/auth"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/jobs"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

const (
	sessionCookie = "minectl_session"
	sessionTTL    = 30 * 24 * time.Hour
	maxBodyBytes  = 1 << 20
)

type Config struct {
	Manager *service.Manager
	Store   *store.Store
	Jobs    *jobs.Runner
	// SetupToken must be presented to create the first admin. Empty once
	// an admin exists.
	SetupToken string
	// OnSetupDone runs after the first admin is created.
	OnSetupDone func()
	// TrustProxy uses X-Forwarded-For for the client address, for panels
	// behind a reverse proxy.
	TrustProxy bool
	// PublicAddress is what players type to join (a domain or IP). Empty
	// lets the UI work it out from the page address or the LAN address.
	PublicAddress string
	// UI serves everything outside /api/. Nil serves a placeholder page.
	UI http.Handler
	// Restart replaces the panel process with the binary on disk, after an
	// update. Nil turns off updating from the panel.
	Restart func()
}

type API struct {
	cfg      Config
	m        *service.Manager
	st       *store.Store
	jobs     *jobs.Runner
	mux      *http.ServeMux
	setupMu  sync.Mutex
	ipLimit  *auth.Limiter
	usrLimit *auth.Limiter
	updates  updateCache
}

// level is what a route requires of the caller.
type level int

const (
	public level = iota
	user
	admin
	// The server-scoped levels look up {name}. A caller with no access to
	// the server gets 404, so server names aren't leaked.
	serverViewer
	serverOperator
	serverAdmin
)

var roleRank = map[string]int{domain.RoleViewer: 1, domain.RoleOperator: 2, domain.RoleAdmin: 3}

type reqCtx struct {
	user        *domain.User
	sessionHash string
	viaToken    bool
	server      *domain.Server
	role        string
}

type handlerFunc func(w http.ResponseWriter, r *http.Request, c *reqCtx)

type route struct {
	pattern string
	level   level
	h       handlerFunc
}

func New(cfg Config) *API {
	a := &API{
		cfg:      cfg,
		m:        cfg.Manager,
		st:       cfg.Store,
		jobs:     cfg.Jobs,
		mux:      http.NewServeMux(),
		ipLimit:  auth.NewLimiter(10, 15*time.Minute),
		usrLimit: auth.NewLimiter(20, 15*time.Minute),
	}
	for _, rt := range a.routes() {
		a.mux.Handle(rt.pattern, a.wrap(rt))
	}
	a.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, "no such endpoint")
	})
	ui := cfg.UI
	if ui == nil {
		ui = http.HandlerFunc(placeholder)
	}
	a.mux.Handle("/", ui)
	return a
}

func (a *API) routes() []route {
	return []route{
		{"GET /api/v1/health", public, a.health},
		{"GET /api/v1/setup", public, a.setupStatus},
		{"POST /api/v1/setup", public, a.setup},
		{"POST /api/v1/auth/login", public, a.login},
		{"POST /api/v1/auth/logout", user, a.logout},
		{"GET /api/v1/auth/me", user, a.me},
		{"POST /api/v1/auth/password", user, a.changePassword},

		{"GET /api/v1/tokens", user, a.listTokens},
		{"POST /api/v1/tokens", user, a.createToken},
		{"DELETE /api/v1/tokens/{id}", user, a.deleteToken},

		{"GET /api/v1/users", admin, a.listUsers},
		{"POST /api/v1/users", admin, a.createUser},
		{"PATCH /api/v1/users/{id}", admin, a.updateUser},
		{"DELETE /api/v1/users/{id}", admin, a.deleteUser},
		{"PUT /api/v1/users/{id}/grants/{server}", admin, a.setGrant},
		{"DELETE /api/v1/users/{id}/grants/{server}", admin, a.deleteGrant},

		{"GET /api/v1/servers", user, a.listServers},
		{"POST /api/v1/servers", admin, a.createServer},
		{"GET /api/v1/servers/{name}", serverViewer, a.getServer},
		{"DELETE /api/v1/servers/{name}", serverAdmin, a.deleteServer},
		{"POST /api/v1/servers/{name}/start", serverOperator, a.startServer},
		{"POST /api/v1/servers/{name}/stop", serverOperator, a.stopServer},
		{"POST /api/v1/servers/{name}/restart", serverOperator, a.restartServer},
		{"POST /api/v1/servers/{name}/resize", serverAdmin, a.resizeServer},
		{"POST /api/v1/servers/{name}/upgrade", serverAdmin, a.upgradeServer},
		{"PUT /api/v1/servers/{name}/modpack", serverAdmin, a.setModpack},
		{"POST /api/v1/servers/{name}/mods", serverAdmin, a.addMod},
		{"DELETE /api/v1/servers/{name}/mods/{slug}", serverAdmin, a.removeMod},
		{"GET /api/v1/servers/{name}/backups", serverViewer, a.listBackups},
		{"POST /api/v1/servers/{name}/backups", serverOperator, a.createBackup},
		{"POST /api/v1/servers/{name}/backups/{id}/restore", serverAdmin, a.restoreBackup},
		{"POST /api/v1/servers/{name}/console", serverOperator, a.console},
		{"GET /api/v1/servers/{name}/players", serverViewer, a.players},
		{"POST /api/v1/servers/{name}/players/{player}/{action}", serverOperator, a.playerAction},
		{"GET /api/v1/servers/{name}/logs", serverViewer, a.streamLogs},
		{"GET /api/v1/servers/{name}/stats", serverViewer, a.streamStats},
		{"GET /api/v1/servers/{name}/jobs", serverViewer, a.serverJobs},
		{"PUT /api/v1/servers/{name}/backup-schedule", serverAdmin, a.setBackupSchedule},
		{"GET /api/v1/servers/{name}/properties", serverViewer, a.getProperties},
		{"PUT /api/v1/servers/{name}/properties", serverAdmin, a.setProperties},
		{"POST /api/v1/servers/{name}/public", serverAdmin, a.makePublic},
		{"DELETE /api/v1/servers/{name}/public", serverAdmin, a.makePrivate},

		{"GET /api/v1/jobs", admin, a.listJobs},
		{"GET /api/v1/jobs/{id}", user, a.getJob},
		{"GET /api/v1/jobs/{id}/events", user, a.jobEvents},

		{"GET /api/v1/versions/{type}", user, a.versions},
		{"GET /api/v1/modpacks/search", user, a.searchModpacks},
		{"GET /api/v1/recommend", user, a.recommend},
		{"GET /api/v1/host", admin, a.host},
		{"GET /api/v1/address", user, a.address},
		{"GET /api/v1/audit", admin, a.listAudit},
		{"GET /api/v1/settings", admin, a.getSettings},
		{"PUT /api/v1/settings", admin, a.putSettings},
		{"POST /api/v1/settings/test-notification", admin, a.testNotification},
		{"POST /api/v1/settings/playit/claim", admin, a.startPlayitClaim},
		{"POST /api/v1/settings/playit/claim/{code}", admin, a.checkPlayitClaim},
		{"DELETE /api/v1/settings/playit", admin, a.unlinkPlayit},
		{"GET /api/v1/update", admin, a.getUpdate},
		{"POST /api/v1/update", admin, a.applyUpdate},
	}
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; frame-ancestors 'none'")
	if r.TLS != nil {
		h.Set("Strict-Transport-Security", "max-age=31536000")
	}
	defer func() {
		if p := recover(); p != nil {
			if p == http.ErrAbortHandler {
				panic(p)
			}
			log.Printf("panic serving %s %s: %v", r.Method, r.URL.Path, p)
			writeErr(w, http.StatusInternalServerError, "internal error")
		}
	}()
	a.mux.ServeHTTP(w, r)
}

func (a *API) wrap(rt route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		c := &reqCtx{}
		c.user, c.sessionHash, c.viaToken = a.authenticate(r)

		// Cookies are sent automatically by browsers, so any unsafe request
		// not using a bearer token must come from the panel's own origin.
		if !safeMethod(r.Method) && !c.viaToken && !sameOrigin(r) {
			writeErr(w, http.StatusForbidden, "cross-origin request refused")
			return
		}
		if rt.level == public {
			rt.h(w, r, c)
			return
		}
		if c.user == nil {
			writeErr(w, http.StatusUnauthorized, "log in first")
			return
		}
		if rt.level == admin && !c.user.IsAdmin {
			writeErr(w, http.StatusForbidden, "only admins can do this")
			return
		}
		if rt.level >= serverViewer {
			if !a.authorizeServer(w, r, c, rt.level) {
				return
			}
		}
		rt.h(w, r, c)
	})
}

func (a *API) authorizeServer(w http.ResponseWriter, r *http.Request, c *reqCtx, lvl level) bool {
	name := r.PathValue("name")
	s, err := a.st.GetServer(name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return false
	}
	role := domain.RoleAdmin
	if !c.user.IsAdmin {
		role, err = a.st.GrantRole(c.user.ID, name)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return false
		}
	}
	if s == nil || role == "" {
		writeErr(w, http.StatusNotFound, "server not found: "+name)
		return false
	}
	need := map[level]string{serverViewer: domain.RoleViewer, serverOperator: domain.RoleOperator, serverAdmin: domain.RoleAdmin}[lvl]
	if roleRank[role] < roleRank[need] {
		writeErr(w, http.StatusForbidden, fmt.Sprintf("your role on %s (%s) can't do this", name, role))
		return false
	}
	c.server, c.role = s, role
	return true
}

// canView reports whether the caller may see a server, for endpoints that
// aren't routed by {name}.
func (a *API) canView(c *reqCtx, server string) bool {
	if c.user.IsAdmin {
		return true
	}
	role, err := a.st.GrantRole(c.user.ID, server)
	return err == nil && role != ""
}

func (a *API) authenticate(r *http.Request) (*domain.User, string, bool) {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		tok := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		if !strings.HasPrefix(tok, auth.APITokenPrefix) {
			return nil, "", true
		}
		u, err := a.st.APITokenUser(auth.HashToken(tok))
		if err != nil {
			log.Printf("token lookup: %v", err)
		}
		return u, "", true
	}
	ck, err := r.Cookie(sessionCookie)
	if err != nil || ck.Value == "" {
		return nil, "", false
	}
	hash := auth.HashToken(ck.Value)
	u, err := a.st.SessionUser(hash)
	if err != nil {
		log.Printf("session lookup: %v", err)
	}
	if u == nil {
		return nil, "", false
	}
	return u, hash, false
}

func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Non-browser clients send neither header; browsers always send
		// Sec-Fetch-Site on requests they make.
		site := r.Header.Get("Sec-Fetch-Site")
		return site == "" || site == "same-origin" || site == "none"
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && strings.EqualFold(u.Host, r.Host)
}

func (a *API) clientIP(r *http.Request) string {
	if a.cfg.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *API) audit(r *http.Request, c *reqCtx, action, server, detail string) {
	e := domain.AuditEntry{Action: action, Server: server, Detail: detail, IP: a.clientIP(r)}
	if c.user != nil {
		e.UserID, e.Username = c.user.ID, c.user.Username
	}
	if err := a.st.AddAudit(e); err != nil {
		log.Printf("audit %s: %v", action, err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decode reads a JSON body into v. An empty body leaves v unchanged.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeErr(w, http.StatusRequestEntityTooLarge, "request body too large")
		} else {
			writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		}
		return false
	}
	return true
}

func placeholder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>minectl</title>
<body style="font-family:system-ui;max-width:42em;margin:3em auto;padding:0 1em;line-height:1.5">
<h1>minectl panel</h1>
<p>The API is running at <code>/api/v1</code>. The web interface arrives in the next release; until then you can use the API directly.</p>
<p>To create the admin account, use the setup token from the panel's log:</p>
<pre style="background:#8881;padding:1em;overflow-x:auto">curl -X POST http://this-panel/api/v1/setup \
  -d '{"token":"SETUP_TOKEN","username":"admin","password":"choose-a-password"}'</pre>
<p>See the <a href="https://github.com/Geordie1071-spec/minectl#web-panel">README</a> for the API.</p>`)
}

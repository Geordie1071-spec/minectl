package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/auth"
	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

func (a *API) health(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": config.Version})
}

func (a *API) setupNeeded() bool {
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	return a.cfg.SetupToken != ""
}

func (a *API) setupStatus(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	writeJSON(w, http.StatusOK, map[string]bool{"needed": a.setupNeeded()})
}

func (a *API) setup(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Token    string `json:"token"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	ip := a.clientIP(r)
	if blocked, wait := a.ipLimit.Blocked("ip:" + ip); blocked {
		tooMany(w, wait)
		return
	}
	a.setupMu.Lock()
	token := a.cfg.SetupToken
	a.setupMu.Unlock()
	if token == "" {
		writeErr(w, http.StatusConflict, "setup is already done; log in instead")
		return
	}
	if !auth.Equal(req.Token, token) {
		a.ipLimit.Fail("ip:" + ip)
		writeErr(w, http.StatusForbidden, "wrong setup token; it's printed in the panel's log when it starts")
		return
	}
	if err := auth.ValidateUsername(req.Username); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	u, err := a.st.CreateFirstAdmin(req.Username, hash)
	if errors.Is(err, store.ErrExists) {
		writeErr(w, http.StatusConflict, "setup is already done; log in instead")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.setupMu.Lock()
	a.cfg.SetupToken = ""
	a.setupMu.Unlock()
	if a.cfg.OnSetupDone != nil {
		a.cfg.OnSetupDone()
	}
	c.user = u
	a.audit(r, c, "setup", "", "created admin "+u.Username)
	a.startSession(w, r, u)
}

func (a *API) login(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	ipKey, userKey := "ip:"+a.clientIP(r), "user:"+strings.ToLower(req.Username)
	for _, k := range []string{ipKey, userKey} {
		lim := a.ipLimit
		if k == userKey {
			lim = a.usrLimit
		}
		if blocked, wait := lim.Blocked(k); blocked {
			tooMany(w, wait)
			return
		}
	}
	u, hash, err := a.st.GetUserByName(req.Username)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if u == nil {
		auth.DummyCheck(req.Password)
	}
	if u == nil || !auth.CheckPassword(req.Password, hash) {
		a.ipLimit.Fail(ipKey)
		a.usrLimit.Fail(userKey)
		a.audit(r, &reqCtx{}, "login.failed", "", "username "+truncate(req.Username, 64))
		writeErr(w, http.StatusUnauthorized, "wrong username or password")
		return
	}
	a.ipLimit.Reset(ipKey)
	a.usrLimit.Reset(userKey)
	c.user = u
	a.audit(r, c, "login", "", "")
	a.startSession(w, r, u)
}

func (a *API) startSession(w http.ResponseWriter, r *http.Request, u *domain.User) {
	token := auth.RandomToken()
	expires := time.Now().Add(sessionTTL)
	if err := a.st.CreateSession(auth.HashToken(token), u.ID, expires); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = a.st.PruneSessions()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
	a.writeMe(w, u)
}

func secureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func tooMany(w http.ResponseWriter, wait time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
	writeErr(w, http.StatusTooManyRequests, fmt.Sprintf("too many failed attempts; try again in %s", wait.Round(time.Minute)+time.Minute))
}

func (a *API) logout(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	if c.sessionHash != "" {
		_ = a.st.DeleteSession(c.sessionHash)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) me(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	a.writeMe(w, c.user)
}

func (a *API) writeMe(w http.ResponseWriter, u *domain.User) {
	grants, err := a.st.ListGrants(u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "grants": grants})
}

func (a *API) changePassword(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !decode(w, r, &req) {
		return
	}
	key := "user:" + strings.ToLower(c.user.Username)
	if blocked, wait := a.usrLimit.Blocked(key); blocked {
		tooMany(w, wait)
		return
	}
	_, hash, err := a.st.GetUserByName(c.user.Username)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !auth.CheckPassword(req.Current, hash) {
		a.usrLimit.Fail(key)
		writeErr(w, http.StatusForbidden, "current password is wrong")
		return
	}
	if err := auth.ValidatePassword(req.New); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	newHash, err := auth.HashPassword(req.New)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.st.SetUserPassword(c.user.ID, newHash); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = a.st.DeleteUserSessions(c.user.ID, c.sessionHash)
	a.audit(r, c, "password.change", "", "")
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listTokens(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	list, err := a.st.ListAPITokens(c.user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) createToken(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 64 {
		writeErr(w, http.StatusBadRequest, "give the token a name of up to 64 characters")
		return
	}
	token := auth.NewAPIToken()
	t, err := a.st.CreateAPIToken(c.user.ID, req.Name, auth.HashToken(token))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r, c, "token.create", "", req.Name)
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "info": t})
}

func (a *API) deleteToken(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid token id")
		return
	}
	ok, err := a.st.DeleteAPIToken(c.user.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "token not found")
		return
	}
	a.audit(r, c, "token.delete", "", r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

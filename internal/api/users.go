package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Geordie1071-spec/minectl/internal/auth"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

type userView struct {
	domain.User
	Grants []domain.Grant `json:"grants"`
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	users, err := a.st.ListUsers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]userView, 0, len(users))
	for _, u := range users {
		g, err := a.st.ListGrants(u.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, userView{User: u, Grants: g})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if !decode(w, r, &req) {
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
	u, err := a.st.CreateUser(req.Username, hash, req.IsAdmin)
	if errors.Is(err, store.ErrExists) {
		writeErr(w, http.StatusConflict, "that username is taken")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail := u.Username
	if u.IsAdmin {
		detail += " (admin)"
	}
	a.audit(r, c, "user.create", "", detail)
	writeJSON(w, http.StatusCreated, userView{User: *u, Grants: []domain.Grant{}})
}

func (a *API) pathUser(w http.ResponseWriter, r *http.Request) *domain.User {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid user id")
		return nil
	}
	u, err := a.st.GetUser(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return nil
	}
	if u == nil {
		writeErr(w, http.StatusNotFound, "user not found")
	}
	return u
}

func (a *API) updateUser(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	u := a.pathUser(w, r)
	if u == nil {
		return
	}
	var req struct {
		IsAdmin  *bool   `json:"is_admin"`
		Password *string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.IsAdmin != nil && !*req.IsAdmin && u.IsAdmin {
		if n, err := a.st.CountAdmins(); err != nil || n <= 1 {
			writeErr(w, http.StatusConflict, "can't remove the last admin")
			return
		}
	}
	var newHash string
	if req.Password != nil {
		if err := auth.ValidatePassword(*req.Password); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		var err error
		if newHash, err = auth.HashPassword(*req.Password); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if req.IsAdmin != nil && *req.IsAdmin != u.IsAdmin {
		if err := a.st.SetUserAdmin(u.ID, *req.IsAdmin); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		u.IsAdmin = *req.IsAdmin
		a.audit(r, c, "user.admin", "", u.Username+" admin="+strconv.FormatBool(u.IsAdmin))
	}
	if newHash != "" {
		if err := a.st.SetUserPassword(u.ID, newHash); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		keep := ""
		if u.ID == c.user.ID {
			keep = c.sessionHash
		}
		_ = a.st.DeleteUserSessions(u.ID, keep)
		a.audit(r, c, "user.password", "", u.Username)
	}
	g, _ := a.st.ListGrants(u.ID)
	writeJSON(w, http.StatusOK, userView{User: *u, Grants: g})
}

func (a *API) deleteUser(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	u := a.pathUser(w, r)
	if u == nil {
		return
	}
	if u.ID == c.user.ID {
		writeErr(w, http.StatusConflict, "you can't delete your own account")
		return
	}
	if err := a.st.DeleteUser(u.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r, c, "user.delete", "", u.Username)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) setGrant(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	u := a.pathUser(w, r)
	if u == nil {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Role != domain.RoleViewer && req.Role != domain.RoleOperator {
		writeErr(w, http.StatusBadRequest, `role must be "viewer" or "operator"; make the user an admin for full access`)
		return
	}
	server := r.PathValue("server")
	if s, err := a.st.GetServer(server); err != nil || s == nil {
		writeErr(w, http.StatusNotFound, "server not found: "+server)
		return
	}
	if err := a.st.SetGrant(u.ID, server, req.Role); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r, c, "grant.set", server, u.Username+" "+req.Role)
	writeJSON(w, http.StatusOK, domain.Grant{UserID: u.ID, Server: server, Role: req.Role})
}

func (a *API) deleteGrant(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	u := a.pathUser(w, r)
	if u == nil {
		return
	}
	server := r.PathValue("server")
	if err := a.st.DeleteGrant(u.ID, server); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r, c, "grant.delete", server, u.Username)
	w.WriteHeader(http.StatusNoContent)
}

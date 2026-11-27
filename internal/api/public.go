package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/expose"
	"github.com/Geordie1071-spec/minectl/internal/service"
)

func (a *API) makePublic(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Via string `json:"via"`
	}
	if !decode(w, r, &req) {
		return
	}
	via := strings.ToLower(strings.TrimSpace(req.Via))
	if via == "" {
		via = service.ViaAuto
	}
	if via != service.ViaAuto && via != service.ViaRouter && via != service.ViaPlayit {
		writeErr(w, http.StatusBadRequest, "via must be auto, router, or playit")
		return
	}
	name := c.server.Name
	a.startJob(w, r, c, "public", name, via, func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		pub, err := m.MakePublic(ctx, name, via, ev)
		if err != nil {
			return "", err
		}
		return "friends can join at " + pub.Address, nil
	})
}

func (a *API) makePrivate(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	name := c.server.Name
	a.startJob(w, r, c, "private", name, "", func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		if err := m.MakePrivate(ctx, name, ev); err != nil {
			return "", err
		}
		return name + " is only reachable on your network", nil
	})
}

// startPlayitClaim returns a fresh code for the user to approve on playit.gg.
// The browser then polls checkPlayitClaim until it's linked.
func (a *API) startPlayitClaim(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	code := expose.NewClaimCode()
	writeJSON(w, http.StatusOK, map[string]string{"code": code, "url": expose.ClaimURL(code)})
}

func (a *API) checkPlayitClaim(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	code := r.PathValue("code")
	if !expose.ValidClaimCode(code) {
		writeErr(w, http.StatusBadRequest, "invalid claim code")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	status, err := a.m.PlayitClaim(ctx, code)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if status == service.ClaimLinked {
		a.audit(r, c, "settings.playit_link", "", "")
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (a *API) unlinkPlayit(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	m := a.manager(c)
	if err := m.UnlinkPlayit(ctx, service.NopEvents{}); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r, c, "settings.playit_unlink", "", "")
	w.WriteHeader(http.StatusNoContent)
}

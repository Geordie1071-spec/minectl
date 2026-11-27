package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/notify"
	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/Geordie1071-spec/minectl/internal/service"
)

func (a *API) setBackupSchedule(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Schedule string `json:"schedule"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, err := a.m.SetBackupSchedule(c.server.Name, strings.ToLower(strings.TrimSpace(req.Schedule)))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	detail := s.BackupSchedule
	if detail == "" {
		detail = "off"
	}
	a.audit(r, c, "server.backup_schedule", s.Name, detail)
	writeJSON(w, http.StatusOK, a.view(s, c.role, true))
}

func (a *API) getProperties(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	p := c.server.Properties
	if p == nil {
		p = &domain.Properties{}
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) setProperties(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var p domain.Properties
	if !decode(w, r, &p) {
		return
	}
	p.MOTD = strings.TrimSpace(p.MOTD)
	p.Difficulty = strings.ToLower(p.Difficulty)
	p.Gamemode = strings.ToLower(p.Gamemode)
	if err := server.ValidateProperties(&p); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	name := c.server.Name
	a.startJob(w, r, c, "properties", name, "", func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		if err := m.SetProperties(ctx, name, p, ev); err != nil {
			return "", err
		}
		return "game settings applied", nil
	})
}

type settingsView struct {
	DiscordWebhookSet bool `json:"discord_webhook_set"`
	BackupRetention   int  `json:"backup_retention"`
}

func (a *API) getSettings(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	cfg, err := a.st.GetConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settingsView{DiscordWebhookSet: cfg.DiscordWebhook != "", BackupRetention: cfg.BackupRetention})
}

func (a *API) putSettings(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		DiscordWebhook  *string `json:"discord_webhook"`
		BackupRetention *int    `json:"backup_retention"`
	}
	if !decode(w, r, &req) {
		return
	}
	cfg, err := a.st.GetConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var changed []string
	if req.DiscordWebhook != nil {
		hook := strings.TrimSpace(*req.DiscordWebhook)
		if hook != "" {
			if err := notify.ValidateWebhook(hook); err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		cfg.DiscordWebhook = hook
		changed = append(changed, "discord webhook")
	}
	if req.BackupRetention != nil {
		if *req.BackupRetention < 1 || *req.BackupRetention > 100 {
			writeErr(w, http.StatusBadRequest, "keep between 1 and 100 backups per server")
			return
		}
		cfg.BackupRetention = *req.BackupRetention
		changed = append(changed, "backup retention")
	}
	if err := a.st.SaveConfig(cfg); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r, c, "settings.update", "", strings.Join(changed, ", "))
	writeJSON(w, http.StatusOK, settingsView{DiscordWebhookSet: cfg.DiscordWebhook != "", BackupRetention: cfg.BackupRetention})
}

func (a *API) testNotification(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	cfg, err := a.st.GetConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cfg.DiscordWebhook == "" {
		writeErr(w, http.StatusBadRequest, "set a Discord webhook first")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := notify.NewDiscord().Send(ctx, cfg.DiscordWebhook, notify.Info, "minectl is connected", "Alerts from your panel will appear here."); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

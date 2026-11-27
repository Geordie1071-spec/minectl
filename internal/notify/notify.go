// Package notify sends panel alerts to a Discord webhook.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Level int

const (
	Info Level = iota
	Warning
	Error
)

var colors = map[Level]int{Info: 0x2c6bed, Warning: 0xf5b041, Error: 0xe5484d}

// ValidateWebhook accepts only Discord webhook URLs, so the panel can't be
// used to send requests to arbitrary addresses.
func ValidateWebhook(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return errors.New("use the webhook URL Discord gives you (https://discord.com/api/webhooks/...)")
	}
	host := strings.ToLower(u.Hostname())
	okHost := host == "discord.com" || host == "discordapp.com" || host == "ptb.discord.com" || host == "canary.discord.com"
	if !okHost || !strings.HasPrefix(u.Path, "/api/webhooks/") {
		return errors.New("use the webhook URL Discord gives you (https://discord.com/api/webhooks/...)")
	}
	return nil
}

type Discord struct {
	HTTP *http.Client
	// skipValidation lets tests point at a local server.
	skipValidation bool
}

func NewDiscord() *Discord {
	return &Discord{HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Send posts one embed. An empty webhook does nothing.
func (d *Discord) Send(ctx context.Context, webhook string, level Level, title, body string) error {
	if webhook == "" {
		return nil
	}
	if !d.skipValidation {
		if err := ValidateWebhook(webhook); err != nil {
			return err
		}
	}
	if len(body) > 3500 {
		body = body[:3500] + "…"
	}
	payload, _ := json.Marshal(map[string]any{
		"username": "minectl",
		"embeds": []map[string]any{{
			"title":       title,
			"description": body,
			"color":       colors[level],
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		}},
		"allowed_mentions": map[string]any{"parse": []string{}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("discord: %s %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

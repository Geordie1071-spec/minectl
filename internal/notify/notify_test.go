package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateWebhook(t *testing.T) {
	for _, ok := range []string{"https://discord.com/api/webhooks/1/abc", "https://discordapp.com/api/webhooks/1/abc"} {
		if ValidateWebhook(ok) != nil {
			t.Errorf("%s should be accepted", ok)
		}
	}
	for _, bad := range []string{"", "http://discord.com/api/webhooks/1/a", "https://evil.com/api/webhooks/1/a", "https://discord.com.evil.com/api/webhooks/1", "https://discord.com/other", "file:///etc/passwd"} {
		if ValidateWebhook(bad) == nil {
			t.Errorf("%s should be rejected", bad)
		}
	}
}

func TestSend(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	d := NewDiscord()
	d.skipValidation = true
	if err := d.Send(context.Background(), srv.URL, Error, "survival crashed", "out of memory @everyone"); err != nil {
		t.Fatal(err)
	}
	embed := got["embeds"].([]any)[0].(map[string]any)
	if embed["title"] != "survival crashed" || int(embed["color"].(float64)) != colors[Error] {
		t.Fatalf("payload = %v", got)
	}
	if mentions := got["allowed_mentions"].(map[string]any)["parse"].([]any); len(mentions) != 0 {
		t.Fatal("mentions must be disabled so log text can't ping people")
	}
	if err := d.Send(context.Background(), "", Info, "x", "y"); err != nil {
		t.Fatal("empty webhook should be a no-op")
	}
	d.skipValidation = false
	if err := d.Send(context.Background(), srv.URL, Info, "x", "y"); err == nil || !strings.Contains(err.Error(), "webhook URL") {
		t.Fatalf("non-Discord URL should be refused: %v", err)
	}
}

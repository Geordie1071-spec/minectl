//go:build e2e

package e2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type panelClient struct {
	t    *testing.T
	base string
	http *http.Client
}

func newPanelClient(t *testing.T, base string) *panelClient {
	jar, _ := cookiejar.New(nil)
	return &panelClient{t: t, base: base, http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
}

func (c *panelClient) do(method, path, body string) (int, []byte) {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Origin", c.base)
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func (c *panelClient) must(method, path, body string, want int, out any) {
	c.t.Helper()
	code, data := c.do(method, path, body)
	if code != want {
		c.t.Fatalf("%s %s: got %d, want %d: %s", method, path, code, want, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s %s: decode %s: %v", method, path, data, err)
		}
	}
}

type jobJSON struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

// follow streams a job's events until it ends and returns the final job and
// every message seen.
func (c *panelClient) follow(id string) (jobJSON, string) {
	c.t.Helper()
	req, _ := http.NewRequest("GET", c.base+"/api/v1/jobs/"+id+"/events", nil)
	client := *c.http
	client.Timeout = 15 * time.Minute
	resp, err := client.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var seen strings.Builder
	var final jobJSON
	event := ""
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data := strings.TrimPrefix(line, "data: ")
			if event == "end" {
				_ = json.Unmarshal([]byte(data), &final)
			} else {
				var e struct {
					Message string `json:"message"`
				}
				_ = json.Unmarshal([]byte(data), &e)
				seen.WriteString(e.Message + "\n")
			}
		}
	}
	c.t.Logf("job %s (%s):\n%s", id, final.Status, seen.String())
	return final, seen.String()
}

func (c *panelClient) runJob(method, path, body string) string {
	c.t.Helper()
	var j jobJSON
	c.must(method, path, body, http.StatusAccepted, &j)
	final, seen := c.follow(j.ID)
	if final.Status != "succeeded" {
		c.t.Fatalf("%s %s job %s: %s", method, path, final.Status, final.Error)
	}
	return seen
}

func (c *panelClient) console(server, cmd string) string {
	c.t.Helper()
	body, _ := json.Marshal(map[string]string{"command": cmd})
	var out struct {
		Output string `json:"output"`
	}
	c.must("POST", "/api/v1/servers/"+server+"/console", string(body), http.StatusOK, &out)
	return out.Output
}

func TestPanel(t *testing.T) {
	e := newEnv(t)
	port := freePort(t)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	name := fmt.Sprintf("e2e-panel-%d", rand.Intn(1_000_000))
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", "minectl-"+name).Run() })

	serve := exec.Command(binary, "serve", "--listen", fmt.Sprintf("127.0.0.1:%d", port))
	serve.Env = append(os.Environ(), "MINECTL_CONFIG_DIR="+e.configDir)
	logFile, _ := os.Create(filepath.Join(t.TempDir(), "serve.log"))
	serve.Stdout, serve.Stderr = logFile, logFile
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = serve.Process.Signal(os.Interrupt)
		_ = serve.Wait()
		logs, _ := os.ReadFile(logFile.Name())
		t.Logf("serve log:\n%s", logs)
	})

	owner := newPanelClient(t, base)
	deadline := time.Now().Add(15 * time.Second)
	for {
		if resp, err := http.Get(base + "/api/v1/health"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("panel never became healthy")
		}
		time.Sleep(200 * time.Millisecond)
	}

	token, err := os.ReadFile(filepath.Join(e.configDir, "setup-token"))
	if err != nil {
		t.Fatalf("setup token file: %v", err)
	}
	owner.must("POST", "/api/v1/setup", fmt.Sprintf(`{"token":%q,"username":"owner","password":"ownerpass1"}`, strings.TrimSpace(string(token))), http.StatusOK, nil)
	if _, err := os.Stat(filepath.Join(e.configDir, "setup-token")); !os.IsNotExist(err) {
		t.Fatal("setup token file should be removed after setup")
	}

	seen := owner.runJob("POST", "/api/v1/servers", fmt.Sprintf(`{"name":%q,"type":"paper","version":"26.3","memory":"1G","port":%d}`, name, freePort(t)))
	if !strings.Contains(seen, "Ready in") {
		t.Fatal("create job never reported ready")
	}

	t.Run("cli sees panel servers", func(t *testing.T) {
		e.name = name
		if s := e.server(); s == nil || s.Status != "running" {
			t.Fatalf("minectl list doesn't show the panel's server: %+v", s)
		}
	})

	t.Run("console and players", func(t *testing.T) {
		if out := owner.console(name, "list"); !strings.Contains(out, "players online") || strings.ContainsRune(out, '\x01') {
			t.Fatalf("console output %q", out)
		}
		var players struct {
			Online int `json:"online"`
			Max    int `json:"max"`
		}
		owner.must("GET", "/api/v1/servers/"+name+"/players", "", http.StatusOK, &players)
		if players.Max == 0 {
			t.Fatalf("players = %+v", players)
		}
	})

	t.Run("backup and restore", func(t *testing.T) {
		owner.console(name, "forceload add 0 0")
		owner.console(name, "setblock 0 100 0 minecraft:gold_block")
		owner.runJob("POST", "/api/v1/servers/"+name+"/backups", "")
		var backups []struct {
			ID string `json:"id"`
		}
		owner.must("GET", "/api/v1/servers/"+name+"/backups", "", http.StatusOK, &backups)
		if len(backups) != 1 {
			t.Fatalf("backups = %+v", backups)
		}
		owner.console(name, "setblock 0 100 0 minecraft:air")
		owner.runJob("POST", "/api/v1/servers/"+name+"/backups/"+backups[0].ID+"/restore", "")
		owner.console(name, "forceload add 0 0")
		if out := owner.console(name, "execute if block 0 100 0 minecraft:gold_block"); !strings.Contains(out, "passed") {
			t.Fatalf("gold block not restored: %q", out)
		}
	})

	t.Run("viewer is read-only", func(t *testing.T) {
		var u struct {
			ID int64 `json:"id"`
		}
		owner.must("POST", "/api/v1/users", `{"username":"friend","password":"friendpass1"}`, http.StatusCreated, &u)
		owner.must("PUT", fmt.Sprintf("/api/v1/users/%d/grants/%s", u.ID, name), `{"role":"viewer"}`, http.StatusOK, nil)

		friend := newPanelClient(t, base)
		friend.must("POST", "/api/v1/auth/login", `{"username":"friend","password":"friendpass1"}`, http.StatusOK, nil)
		var list []struct {
			Name string `json:"name"`
		}
		friend.must("GET", "/api/v1/servers", "", http.StatusOK, &list)
		if len(list) != 1 || list[0].Name != name {
			t.Fatalf("viewer list = %+v", list)
		}
		friend.must("POST", "/api/v1/servers/"+name+"/stop", "", http.StatusForbidden, nil)
		friend.must("POST", "/api/v1/servers/"+name+"/console", `{"command":"stop"}`, http.StatusForbidden, nil)
		friend.must("DELETE", "/api/v1/servers/"+name, "", http.StatusForbidden, nil)
		friend.must("POST", "/api/v1/servers", `{"name":"sneaky"}`, http.StatusForbidden, nil)
	})

	t.Run("cli respects panel locks", func(t *testing.T) {
		var j jobJSON
		owner.must("POST", "/api/v1/servers/"+name+"/restart", "", http.StatusAccepted, &j)
		time.Sleep(2 * time.Second)
		out, err := e.run("stop", name)
		if err == nil || !strings.Contains(out, "is busy: panel (owner)") {
			t.Errorf("CLI stop during a panel restart should report the lock, err=%v", err)
		}
		if final, _ := owner.follow(j.ID); final.Status != "succeeded" {
			t.Fatalf("restart: %+v", final)
		}
	})

	t.Run("delete", func(t *testing.T) {
		owner.runJob("DELETE", "/api/v1/servers/"+name+"?purge=true", "")
		var list []any
		owner.must("GET", "/api/v1/servers", "", http.StatusOK, &list)
		if len(list) != 0 {
			t.Fatalf("servers after delete: %+v", list)
		}
		if err := exec.Command("docker", "inspect", "minectl-"+name).Run(); err == nil {
			t.Fatal("container still exists")
		}
	})
}

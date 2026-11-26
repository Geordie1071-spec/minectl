package server

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
)

var (
	readyLine   = regexp.MustCompile(`Done \(\d+[.,]\d+s\)!`)
	fromModLine = regexp.MustCompile(`from mod ([A-Za-z0-9_.-]+)`)
	ErrStopped  = errors.New("server stopped before it finished starting")
)

const keepLogLines = 400

func IsReadyLine(line string) bool {
	return readyLine.MatchString(line)
}

func WaitReady(ctx context.Context, d *docker.Client, s *domain.Server, since time.Time, onLine func(line string)) ([]string, error) {
	lines := make(chan string, 100)
	streamErr := make(chan error, 1)
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		streamErr <- d.StreamLogsSince(streamCtx, s.ContainerID, true, since, lines)
		close(lines)
	}()

	var recent []string
	for line := range lines {
		recent = append(recent, line)
		if len(recent) > keepLogLines {
			recent = recent[1:]
		}
		if onLine != nil {
			onLine(line)
		}
		if IsReadyLine(line) {
			return recent, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return recent, err
	}
	if err := <-streamErr; err != nil && !errors.Is(err, context.Canceled) {
		return recent, fmt.Errorf("read logs: %w", err)
	}
	return recent, ErrStopped
}

func FailureSummary(recent []string) []string {
	var out []string
	seen := map[string]bool{}
	manual := NeedsManualDownloads(recent)
	for _, l := range recent {
		t := strings.TrimSpace(l)
		if seen[t] || strings.Contains(t, "DO NOT report this line") {
			continue
		}
		if manual && strings.Contains(t, "curseforge.com/") {
			seen[t] = true
			out = append(out, t)
			continue
		}
		if strings.Contains(t, "ERROR") || strings.HasPrefix(t, "Caused by:") || strings.Contains(t, "Exception:") {
			seen[t] = true
			out = append(out, t)
		}
	}
	if len(out) > 10 && !manual {
		out = out[len(out)-10:]
	}
	if len(out) == 0 && len(recent) > 0 {
		out = recent[max(0, len(recent)-15):]
	}
	return out
}

func NeedsManualDownloads(recent []string) bool {
	joined := strings.ToLower(strings.Join(recent, "\n"))
	return strings.Contains(joined, "/downloads") || strings.Contains(joined, "need download") || strings.Contains(joined, "manual download") || strings.Contains(joined, "excluded from automated download")
}

func DescribeStartFailure(recent []string) string {
	joined := strings.ToLower(strings.Join(recent, "\n"))
	culprit := ""
	if m := fromModLine.FindAllStringSubmatch(strings.Join(recent, "\n"), -1); len(m) > 0 {
		culprit = m[len(m)-1][1]
	}
	clientOnly := strings.Contains(joined, "net.minecraft.class_310") || strings.Contains(joined, "net/minecraft/class_310") ||
		strings.Contains(joined, "net.minecraft.client.") || strings.Contains(joined, "environment type server")
	switch {
	case NeedsManualDownloads(recent):
		return "some mods in this CurseForge pack block automated downloads; download the files listed below in a browser into the server's downloads folder"
	case strings.Contains(joined, "missingmodschecker") || strings.Contains(joined, "missingmodswindow"):
		return "this pack is missing mods that are only on CurseForge (it tried to open a 'missing mods' window); use the CurseForge edition with --curseforge instead"
	case clientOnly && culprit != "":
		return fmt.Sprintf("the mod %q uses client-only code (MinecraftClient) on the server; it can't run on a dedicated server, so remove it or ask its author for a server fix", culprit)
	case clientOnly:
		return "a mod uses client-only code (MinecraftClient) on the server; a client-only mod is installed on the server"
	case strings.Contains(joined, "mixin") && culprit != "":
		return fmt.Sprintf("the mod %q failed to load (Mixin error); it may be incompatible with this Minecraft or loader version", culprit)
	case strings.Contains(joined, "permission denied") || strings.Contains(joined, "operation not permitted"):
		return "the container can't write to its data folder (check ownership, or SELinux labels on Fedora/RHEL)"
	case strings.Contains(joined, "outofmemory") || strings.Contains(joined, "out of memory"):
		return "the server ran out of memory; try a larger --memory"
	case strings.Contains(joined, "address already in use"):
		return "the port is already in use; pick another with --port"
	case strings.Contains(joined, "is not available") || strings.Contains(joined, "failed to download"):
		return "that Minecraft version isn't available for this server type; omit --version to use the newest supported one"
	case strings.Contains(joined, "unsupportedclassversionerror"):
		return "wrong Java version for this Minecraft version"
	}
	return ""
}

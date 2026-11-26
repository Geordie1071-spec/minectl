package server

import (
	"strings"
	"testing"
)

func TestIsReadyLine(t *testing.T) {
	ready := []string{
		`[14:02:11 INFO]: Done (12.345s)! For help, type "help"`,
		`[Server thread/INFO]: Done (3,210s)! For help, type "help"`,
	}
	for _, l := range ready {
		if !IsReadyLine(l) {
			t.Errorf("expected ready: %q", l)
		}
	}
	if IsReadyLine("[init] Done downloading server jar") {
		t.Error("init line should not count as ready")
	}
}

func TestDescribeStartFailure(t *testing.T) {
	hint := DescribeStartFailure([]string{"mc-image-helper: /data/eula.txt: Permission denied"})
	if !strings.Contains(hint, "SELinux") {
		t.Errorf("hint = %q", hint)
	}
	if DescribeStartFailure([]string{"all good"}) != "" {
		t.Error("expected no hint")
	}
}

func TestClientOnlyModHint(t *testing.T) {
	logs := []string{
		"[14:39:04] [main/ERROR]: Minecraft has crashed!",
		"Caused by: org.spongepowered.asm.mixin.transformer.throwables.MixinPreProcessorException: Attach error for spb-revamped.mixins.json:rain.LightningSkyLightingUpLightningEntity from mod spb-revamped during activity",
		"Caused by: java.lang.ClassNotFoundException: net.minecraft.class_310",
		"\tat net.minecraft.server.Main.main(Main.java:109)",
		"2026-11-26T14:39:04.623Z WARN mc-server-runner Minecraft server failed. Inspect logs above for errors that indicate cause. DO NOT report this line as an error.",
	}
	hint := DescribeStartFailure(logs)
	if !strings.Contains(hint, `"spb-revamped"`) || !strings.Contains(hint, "client-only") {
		t.Errorf("hint = %q", hint)
	}
	summary := FailureSummary(logs)
	if len(summary) != 3 {
		t.Errorf("summary = %q", summary)
	}
}

func TestMissingModsCheckerHint(t *testing.T) {
	logs := []string{
		"[15:01:46] [main/ERROR]: Error thrown while opening! Exiting",
		"java.awt.HeadlessException: ",
		"\tat toni.missingmodschecker.MissingModsWindow.<init>(MissingModsWindow.java:55) ~[missingmodschecker.jar:?]",
	}
	if hint := DescribeStartFailure(logs); !strings.Contains(hint, "--curseforge") {
		t.Errorf("hint = %q", hint)
	}
}

func TestManualDownloadSummaryKeepsLinks(t *testing.T) {
	logs := []string{
		"[mc-image-helper] ERROR : The following mods are not allowed for automated download, place them in /downloads:",
		"  Some Mod  https://www.curseforge.com/minecraft/mc-mods/some-mod/download/123",
		"  Other Mod https://www.curseforge.com/minecraft/mc-mods/other-mod/download/456",
	}
	if !NeedsManualDownloads(logs) {
		t.Fatal("expected manual downloads to be detected")
	}
	if got := FailureSummary(logs); len(got) != 3 {
		t.Errorf("summary = %q", got)
	}
}

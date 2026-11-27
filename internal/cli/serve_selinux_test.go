package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLabelForSELinux(t *testing.T) {
	if b, err := os.ReadFile("/sys/fs/selinux/enforce"); err != nil || strings.TrimSpace(string(b)) != "1" {
		t.Skip("SELinux isn't enforcing here")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "minectl")
	if err := os.WriteFile(bin, []byte("#!/bin/true\n"), 0755); err != nil {
		t.Fatal(err)
	}
	labelForSELinux(dir)
	for _, p := range []string{dir, bin} {
		out, err := exec.Command("ls", "-dZ", p).Output()
		if err != nil || !strings.Contains(string(out), ":bin_t:") {
			t.Errorf("%s label: %s %v", p, out, err)
		}
	}
	// A file created later in the folder, as minectl update does, inherits it.
	later := filepath.Join(dir, "minectl.new")
	_ = os.WriteFile(later, nil, 0755)
	if out, _ := exec.Command("ls", "-Z", later).Output(); !strings.Contains(string(out), ":bin_t:") {
		t.Errorf("new file label: %s", out)
	}
}

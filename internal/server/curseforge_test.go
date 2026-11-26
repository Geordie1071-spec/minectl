package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInstallEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".install-curseforge.env")
	content := "SERVER=\"./fabric-server-mc.1.20.1-loader.0.19.5-launcher.1.1.2.jar\"\nFAMILY=\"FABRIC\"\nTYPE=\"FABRIC\"\nVERSION=\"1.20.1\"\nMODPACK_NAME=\"Into the backrooms: Found Footage (horror)\"\nMODPACK_VERSION=\"2.0.9\"\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	env, err := readInstallEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if env["TYPE"] != "FABRIC" || env["VERSION"] != "1.20.1" || env["MODPACK_NAME"] != "Into the backrooms: Found Footage (horror)" || env["MODPACK_VERSION"] != "2.0.9" {
		t.Errorf("env = %v", env)
	}
}

func TestCountJars(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.jar", "b.jar", "notes.txt"} {
		writeFile(t, filepath.Join(dir, n), "x")
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.jar"), 0755); err != nil {
		t.Fatal(err)
	}
	if got := countJars(dir); got != 2 {
		t.Errorf("countJars = %d, want 2", got)
	}
}

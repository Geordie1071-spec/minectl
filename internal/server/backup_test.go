package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestArchiveRoundTrip(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "server.properties"), "motd=hi")
	writeFile(t, filepath.Join(src, "world", "region", "r.0.0.mca"), "chunkdata")
	if err := os.MkdirAll(filepath.Join(src, "empty"), 0755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := writeTarGz(src, &buf); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := extractTarGz(&buf, dst); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, filepath.Join(dst, "server.properties")); got != "motd=hi" {
		t.Errorf("server.properties = %q", got)
	}
	if got := readFile(t, filepath.Join(dst, "world", "region", "r.0.0.mca")); got != "chunkdata" {
		t.Errorf("region file = %q", got)
	}
	if info, err := os.Stat(filepath.Join(dst, "empty")); err != nil || !info.IsDir() {
		t.Errorf("empty dir not restored: %v", err)
	}
}

func TestExtractRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "../escape.txt", Mode: 0644, Size: 1, Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	_ = gz.Close()

	parent := t.TempDir()
	dst := filepath.Join(parent, "data")
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(&buf, dst); err == nil {
		t.Fatal("expected error for path traversal")
	}
	if _, err := os.Stat(filepath.Join(parent, "escape.txt")); !os.IsNotExist(err) {
		t.Fatal("file escaped destination directory")
	}
}

func TestRestoreDataDirReplacesContents(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "level.dat"), "old-world")
	archive := filepath.Join(t.TempDir(), "bak.tar.gz")
	if err := writeArchiveFile(src, archive); err != nil {
		t.Fatal(err)
	}

	dataDir := filepath.Join(t.TempDir(), "data")
	writeFile(t, filepath.Join(dataDir, "level.dat"), "new-world")
	writeFile(t, filepath.Join(dataDir, "added-later.txt"), "x")

	if err := restoreDataDir(archive, dataDir); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dataDir, "level.dat")); got != "old-world" {
		t.Errorf("level.dat = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "added-later.txt")); !os.IsNotExist(err) {
		t.Error("files not in backup should be gone after restore")
	}
	if _, err := os.Stat(dataDir + ".pre-restore"); !os.IsNotExist(err) {
		t.Error("pre-restore dir should be cleaned up")
	}
}

func TestRestoreDataDirRollsBackOnCorruptArchive(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "bad.tar.gz")
	writeFile(t, archive, "not a gzip file")

	dataDir := filepath.Join(t.TempDir(), "data")
	writeFile(t, filepath.Join(dataDir, "level.dat"), "current")

	if err := restoreDataDir(archive, dataDir); err == nil {
		t.Fatal("expected error for corrupt archive")
	}
	if got := readFile(t, filepath.Join(dataDir, "level.dat")); got != "current" {
		t.Errorf("data not rolled back, level.dat = %q", got)
	}
}

func TestPruneBackupsKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	base := time.Now()
	var backups []domain.Backup
	for i := 0; i < 5; i++ {
		p := filepath.Join(dir, string(rune('a'+i))+".tar.gz")
		writeFile(t, p, "x")
		backups = append(backups, domain.Backup{ID: string(rune('a' + i)), Path: p, CreatedAt: base.Add(time.Duration(i) * time.Minute)})
	}

	kept := pruneBackups(backups, 2)
	if len(kept) != 2 || kept[0].ID != "d" || kept[1].ID != "e" {
		t.Fatalf("kept = %+v", kept)
	}
	for _, id := range []string{"a", "b", "c"} {
		if _, err := os.Stat(filepath.Join(dir, id+".tar.gz")); !os.IsNotExist(err) {
			t.Errorf("backup %s file should be deleted", id)
		}
	}
}

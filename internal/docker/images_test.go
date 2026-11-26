package docker

import (
	"strings"
	"testing"
)

func TestDecodePullProgress(t *testing.T) {
	stream := strings.Join([]string{
		`{"status":"Pulling from itzg/minecraft-server","id":"java21"}`,
		`{"status":"Pulling fs layer","id":"a"}`,
		`{"status":"Already exists","id":"b"}`,
		`{"status":"Downloading","progressDetail":{"current":50,"total":100},"id":"a"}`,
		`{"status":"Downloading","progressDetail":{"current":25,"total":100},"id":"c"}`,
		`{"status":"Download complete","id":"a"}`,
		`{"status":"Download complete","id":"c"}`,
	}, "\n")

	var got []int
	if err := decodePullProgress(strings.NewReader(stream), func(p int) { got = append(got, p) }); err != nil {
		t.Fatal(err)
	}
	want := []int{0, 50, 37, 62, 100}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestDecodePullProgressError(t *testing.T) {
	err := decodePullProgress(strings.NewReader(`{"error":"manifest unknown"}`), nil)
	if err == nil || !strings.Contains(err.Error(), "manifest unknown") {
		t.Fatalf("err = %v", err)
	}
}

func TestContainerMemoryMB(t *testing.T) {
	if got := ContainerMemoryMB(1024); got != 1536 {
		t.Errorf("1024 -> %d, want 1536", got)
	}
	if got := ContainerMemoryMB(8192); got != 10240 {
		t.Errorf("8192 -> %d, want 10240", got)
	}
}

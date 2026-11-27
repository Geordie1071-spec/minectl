package docker

import (
	"errors"
	"strings"
	"testing"
)

func TestExplainPermissionDenied(t *testing.T) {
	raw := errors.New(`permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock: Get "http://%2Fvar%2Frun%2Fdocker.sock/v1.44/_ping": dial unix /var/run/docker.sock: connect: permission denied`)
	t.Setenv("USER", "opc")
	err := Explain(raw)
	var perm *PermissionError
	if !errors.As(err, &perm) || !errors.Is(err, raw) {
		t.Fatalf("not explained: %v", err)
	}
	for _, want := range []string{"opc can't use Docker yet", "log out and back in", "newgrp docker", "usermod -aG docker opc"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message missing %q: %s", want, err)
		}
	}
	other := errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?")
	if Explain(other) != other || Explain(nil) != nil {
		t.Error("other errors should pass through")
	}
}

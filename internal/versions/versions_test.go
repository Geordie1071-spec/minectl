package versions

import (
	"reflect"
	"strings"
	"testing"
)

func TestNeoForgeToMinecraft(t *testing.T) {
	cases := map[string]string{
		"21.1.77":    "1.21.1",
		"21.0.167":   "1.21",
		"20.4.237":   "1.20.4",
		"26.3.0.118": "26.3",
		"26.1.2.40":  "26.1.2",
		"26.3":       "",
		"junk":       "",
	}
	for in, want := range cases {
		if got := NeoForgeToMinecraft(in); got != want {
			t.Errorf("NeoForgeToMinecraft(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestImageTagForJava(t *testing.T) {
	cases := map[int]string{8: "java8", 16: "java16", 17: "java17", 21: "java21", 25: "java25", 22: "java25", 30: "latest"}
	for in, want := range cases {
		if got := ImageTagForJava(in); got != want {
			t.Errorf("ImageTagForJava(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestYearVersionGuess(t *testing.T) {
	cases := map[string]string{"1.26.3": "26.3", "1.26": "26", "1.21.1": "", "26.3": ""}
	for in, want := range cases {
		if got := yearVersionGuess(in); got != want {
			t.Errorf("yearVersionGuess(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortNewestFirst(t *testing.T) {
	vs := []string{"1.20.4", "26.1", "1.21.11", "1.21.9", "26.3", "26.1.2"}
	sortNewestFirst(vs)
	want := []string{"26.3", "26.1.2", "26.1", "1.21.11", "1.21.9", "1.20.4"}
	if !reflect.DeepEqual(vs, want) {
		t.Errorf("got %v, want %v", vs, want)
	}
}

func TestVersionErrorMessage(t *testing.T) {
	err := &VersionError{Requested: "1.26.3", Type: "paper", DidYouMean: "26.3", Available: []string{"26.3", "26.2"}}
	msg := err.Error()
	for _, want := range []string{"1.26.3 is not available for paper", "Did you mean 26.3?", "26.3, 26.2"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}

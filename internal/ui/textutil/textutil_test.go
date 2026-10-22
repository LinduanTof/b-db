package textutil

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestFitCJK(t *testing.T) {
	line := FitLine("张三abc", 10)
	if got := ansi.StringWidth(line); got != 10 {
		t.Fatalf("width = %d, want 10", got)
	}
}

func TestOverlay(t *testing.T) {
	base := strings.Repeat(strings.Repeat("a", 20)+"\n", 9) + strings.Repeat("a", 20)
	modal := "┌──┐\n│中│\n└──┘"
	got := Overlay(base, modal, 20, 10)
	lines := strings.Split(got, "\n")
	if len(lines) != 10 {
		t.Fatalf("lines = %d, want 10", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != 20 {
			t.Fatalf("line %d width = %d, want 20: %q", i, w, line)
		}
	}
	if !strings.Contains(got, "│中│") {
		t.Fatalf("modal content missing:\n%s", got)
	}
}

func TestOverlayIntoShortBase(t *testing.T) {
	got := Overlay("short", "MMM", 20, 5)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("lines = %d, want 5", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != 20 {
			t.Fatalf("line %d width = %d, want 20", i, w)
		}
	}
}

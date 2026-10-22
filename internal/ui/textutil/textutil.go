package textutil

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func Width(s string) int { return ansi.StringWidth(s) }

func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "")
}

func FitLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w > width {
		s = ansi.Truncate(s, width, "")
		w = ansi.StringWidth(s)
	}
	if w < width {
		s += strings.Repeat(" ", width-w)
	}
	return s
}

func FitLines(s string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = FitLine(lines[i], width)
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

// Overlay draws modal centered on top of base. Both strings are assumed to
// contain ANSI-styled content and are measured with terminal cell widths.
func Overlay(base, modal string, width, height int) string {
	if width <= 0 || height <= 0 {
		return base
	}
	baseLines := strings.Split(base, "\n")
	modalLines := strings.Split(modal, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, strings.Repeat(" ", width))
	}
	for i := range baseLines {
		baseLines[i] = FitLine(baseLines[i], width)
	}

	modalW := 0
	for _, line := range modalLines {
		modalW = max(modalW, ansi.StringWidth(line))
	}
	x := max(0, (width-modalW)/2)
	y := max(0, (height-len(modalLines))/2)

	for i, modalLine := range modalLines {
		row := y + i
		if row >= len(baseLines) {
			break
		}
		line := baseLines[row]
		left := ansi.Truncate(line, x, "")
		if w := ansi.StringWidth(left); w < x {
			left += strings.Repeat(" ", x-w)
		}
		rightStart := x + modalW
		var right string
		if ansi.StringWidth(line) > rightStart {
			right = ansi.TruncateLeft(line, rightStart, "")
		}
		rightW := width - rightStart
		if w := ansi.StringWidth(right); w < rightW {
			right += strings.Repeat(" ", rightW-w)
		} else if w > rightW {
			right = ansi.Truncate(right, rightW, "")
		}
		baseLines[row] = left + FitLine(modalLine, modalW) + right
	}
	return strings.Join(baseLines, "\n")
}

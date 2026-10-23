package statusbar

import (
	"strings"

	"github.com/LinduanTof/b-db/internal/ui/textutil"
	"github.com/LinduanTof/b-db/internal/ui/theme"
)

type Model struct {
	Width   int
	Left    string
	Center  string
	Right   string
	IsError bool
}

func (m Model) View() string {
	if m.Width <= 0 {
		return ""
	}
	left, center, right := m.Left, m.Center, m.Right
	if center != "" {
		available := m.Width - textutil.Width(left) - textutil.Width(right) - 2
		if available < 6 {
			center = ""
		} else {
			center = textutil.Truncate(center, available)
		}
	}
	if center != "" {
		line := left + " " + center
		gap := m.Width - textutil.Width(line) - textutil.Width(right) - 1
		if gap < 1 {
			gap = 1
		}
		line += strings.Repeat(" ", gap) + right
		style := theme.Status
		if m.IsError {
			style = theme.StatusError
		}
		return style.Render(textutil.FitLine(line, m.Width))
	}

	gap := m.Width - textutil.Width(left) - textutil.Width(right)
	if gap < 1 {
		left = textutil.Truncate(left, max(0, m.Width-textutil.Width(right)-1))
		gap = m.Width - textutil.Width(left) - textutil.Width(right)
	}
	if gap < 0 {
		right = textutil.Truncate(right, m.Width)
		gap = max(0, m.Width-textutil.Width(left)-textutil.Width(right))
	}
	line := left + strings.Repeat(" ", gap) + right
	style := theme.Status
	if m.IsError {
		style = theme.StatusError
	}
	return style.Render(textutil.FitLine(line, m.Width))
}

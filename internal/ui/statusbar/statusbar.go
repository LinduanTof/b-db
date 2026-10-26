package statusbar

import (
	"strings"

	"github.com/LinduanTof/dbrix/internal/ui/textutil"
	"github.com/LinduanTof/dbrix/internal/ui/theme"
)

type Model struct {
	Width   int
	Left    string
	Right   string
	IsError bool
}

func (m Model) View() string {
	if m.Width <= 0 {
		return ""
	}
	left, right := m.Left, m.Right
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

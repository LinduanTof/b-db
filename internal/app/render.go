package app

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/LinduanTof/dbrix/internal/config"
	"github.com/LinduanTof/dbrix/internal/i18n"
	"github.com/LinduanTof/dbrix/internal/ui/textutil"
	"github.com/LinduanTof/dbrix/internal/ui/theme"
)

func (m *Model) layout() {
	if m.width < 20 || m.height < 6 {
		return
	}
	m.filterInput.SetWidth(max(8, min(30, m.width-30)))
	mainH := m.height - 2
	if m.effectiveZoom() {
		switch m.focus {
		case focusEditor:
			m.sideW, m.rightW = 0, m.width
			m.editorH, m.resultsH = mainH, 0
		case focusResults:
			m.sideW, m.rightW = 0, m.width
			m.editorH, m.resultsH = 0, mainH
		default:
			m.sideW, m.rightW = m.width, 0
			m.editorH, m.resultsH = 0, 0
		}
		m.sidebar.SetSize(max(1, m.sideW), max(1, mainH-1))
		m.editor.SetSize(max(1, m.rightW), max(1, m.editorH-2))
		m.results.SetSize(max(1, m.rightW), max(1, m.resultsH-1))
		return
	}

	sideW := m.width / 4
	if sideW < 25 {
		sideW = 25
	}
	if m.width < 70 {
		sideW = m.width / 3
	}
	if preferred := m.sidebar.PreferredWidth(); sideW < preferred {
		sideW = preferred
	}
	if sideW > 40 {
		sideW = 40
	}
	if sideW > m.width-20 {
		sideW = m.width - 20
	}
	rightW := m.width - sideW - 1

	m.columnsActive = false
	if m.layoutMode == config.LayoutColumns && rightW-1 >= 60 {
		m.columnsActive = true
		editorW := (rightW - 1) / 2
		resultsW := rightW - 1 - editorW
		m.sideW, m.rightW = sideW, rightW
		m.editorW, m.resultsW = editorW, resultsW
		m.editorH, m.resultsH = mainH, mainH
		m.sidebar.SetSize(sideW, max(1, mainH-1))
		m.editor.SetSize(editorW, max(1, mainH-2))
		m.results.SetSize(resultsW, max(1, mainH-1))
		return
	}

	editorH := mainH / 3
	if editorH < 6 {
		editorH = 6
	}
	if editorH > mainH-5 {
		editorH = mainH - 5
	}
	if editorH < 3 {
		editorH = 3
	}
	resultsH := mainH - editorH - 1
	if resultsH < 1 {
		resultsH = 1
	}
	if m.layoutMode == config.LayoutResultsTop {
		editorH, resultsH = resultsH, editorH
	}

	m.sideW, m.rightW, m.editorH, m.resultsH = sideW, rightW, editorH, resultsH
	m.sidebar.SetSize(sideW, max(1, mainH-1))
	m.editor.SetSize(rightW, max(1, editorH-2))
	m.results.SetSize(rightW, resultsH-1)
}

func (m *Model) render() string {
	if m.width <= 0 || m.height <= 0 {
		return "Loading…"
	}
	if m.showHelp {
		return m.renderHelp()
	}
	if m.detail.IsOpen() {
		return m.detail.View()
	}
	base := m.renderMain()
	if m.options.IsOpen() {
		return textutil.Overlay(base, m.options.View(), m.width, m.height)
	}
	if m.tableMenu.IsOpen() {
		return textutil.Overlay(base, m.tableMenu.View(), m.width, m.height)
	}
	if m.dirInput.IsOpen() {
		return textutil.Overlay(base, m.dirInput.View(), m.width, m.height)
	}
	if m.form.IsOpen() {
		return textutil.Overlay(base, m.form.View(), m.width, m.height)
	}
	if m.textView.IsOpen() {
		return m.textView.View()
	}
	if m.cellEdit.IsOpen() {
		return textutil.Overlay(base, m.cellEdit.View(), m.width, m.height)
	}
	if m.confirm.IsOpen() {
		return textutil.Overlay(base, m.confirm.View(), m.width, m.height)
	}
	if m.connForm.IsOpen() {
		return textutil.Overlay(base, m.connForm.View(), m.width, m.height)
	}
	if m.completion.IsOpen() {
		return textutil.Overlay(base, m.completion.View(), m.width, m.height)
	}
	return base
}

func (m *Model) renderMain() string {
	if m.width < 40 || m.height < 10 {
		return textutil.FitLines(theme.Error.Render(m.t.T(i18n.KeyTerminalTooSmall)), m.width, m.height)
	}

	mainH := m.height - 2
	var body string
	if m.effectiveZoom() {
		switch m.focus {
		case focusEditor:
			body = renderPane(m.t.T(i18n.KeyPaneSQL), m.editorHint(), m.tabBar(m.width)+"\n"+m.editor.View(), m.width, mainH)
		case focusResults:
			body = renderPane(m.resultsPaneTitle(), m.resultsPaneHint(), m.results.View(), m.width, mainH)
		default:
			body = renderPane(m.t.T(i18n.KeyPaneConnections), m.sidebarHint(m.width), m.sidebar.View(), m.width, mainH)
		}
	} else {
		side := renderPane(m.t.T(i18n.KeyPaneConnections), m.sidebarHint(m.sideW), m.sidebar.View(), m.sideW, mainH)
		separator := strings.Repeat("│\n", mainH-1) + "│"
		switch {
		case m.columnsActive:
			editorView := renderPane(m.t.T(i18n.KeyPaneSQL), m.editorHint(), m.tabBar(m.editorW)+"\n"+m.editor.View(), m.editorW, mainH)
			resultsView := renderPane(m.resultsPaneTitle(), m.resultsPaneHint(), m.results.View(), m.resultsW, mainH)
			body = lipgloss.JoinHorizontal(lipgloss.Top, side, separator, editorView, separator, resultsView)
		case m.layoutMode == config.LayoutResultsTop:
			resultsView := renderPane(m.resultsPaneTitle(), m.resultsPaneHint(), m.results.View(), m.rightW, m.resultsH)
			editorView := renderPane(m.t.T(i18n.KeyPaneSQL), m.editorHint(), m.tabBar(m.rightW)+"\n"+m.editor.View(), m.rightW, m.editorH)
			right := lipgloss.JoinVertical(lipgloss.Top, resultsView, strings.Repeat("─", m.rightW), editorView)
			body = lipgloss.JoinHorizontal(lipgloss.Top, side, separator, right)
		default:
			editorView := renderPane(m.t.T(i18n.KeyPaneSQL), m.editorHint(), m.tabBar(m.rightW)+"\n"+m.editor.View(), m.rightW, m.editorH)
			resultsView := renderPane(m.resultsPaneTitle(), m.resultsPaneHint(), m.results.View(), m.rightW, m.resultsH)
			right := lipgloss.JoinVertical(lipgloss.Top, editorView, strings.Repeat("─", m.rightW), resultsView)
			body = lipgloss.JoinHorizontal(lipgloss.Top, side, separator, right)
		}
	}

	m.statusbar.Width = m.width
	m.statusbar.Left = m.statusLeft()
	m.statusbar.Right = m.statusRight()
	m.statusbar.IsError = m.statusErr
	menu := theme.MenuBar.Render(textutil.FitLine(" "+m.t.T(i18n.KeyMenuBarHint), m.width))
	return body + "\n" + m.statusbar.View() + "\n" + menu
}

func (m *Model) renderHelp() string {
	sections := []struct {
		title string
		keys  string
	}{
		{m.t.T(i18n.KeyHelpGlobal), m.t.T(i18n.KeyHelpGlobalKeys)},
		{m.t.T(i18n.KeyHelpSidebar), m.t.T(i18n.KeyHelpSidebarKeys)},
		{m.t.T(i18n.KeyHelpEditor), m.t.T(i18n.KeyHelpEditorKeys)},
		{m.t.T(i18n.KeyHelpResults), m.t.T(i18n.KeyHelpResultsKeys)},
	}
	lines := []string{theme.Title.Render(m.t.T(i18n.KeyHelpTitle)), ""}
	for _, section := range sections {
		lines = append(lines, theme.Header.Render(section.title), "  "+section.keys)
	}
	lines = append(lines, "",
		theme.Muted.Render(m.t.T(i18n.KeyMenuHint)),
		theme.Muted.Render(m.t.T(i18n.KeyHelpClose)),
	)

	maxWidth := max(20, m.width-8)
	for i, line := range lines {
		lines[i] = textutil.Truncate(line, maxWidth)
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
	return center(box, m.width, m.height)
}

func (m *Model) editorHint() string {
	if !m.editor.Finding() {
		return m.t.T(i18n.KeyHintEditor)
	}
	query, current, total := m.editor.FindStatus()
	switch {
	case query == "":
		return m.t.T(i18n.KeyFindPrompt)
	case total == 0:
		return m.t.T(i18n.KeyFindPrompt) + query + "  " + m.t.T(i18n.KeyFindNoMatch)
	default:
		return fmt.Sprintf("%s%s  (%d/%d)", m.t.T(i18n.KeyFindPrompt), query, current, total)
	}
}

func (m *Model) sidebarHint(width int) string {
	title := m.t.T(i18n.KeyPaneConnections)
	available := width - textutil.Width(title) - 7
	full := m.t.T(i18n.KeyHintSidebar)
	if textutil.Width(full) <= available {
		return full
	}
	return m.t.T(i18n.KeyHintSidebarCompact)
}

func (m *Model) resultsPaneTitle() string {
	switch m.context {
	case contextInfo:
		return m.t.T(i18n.KeyPaneInfo)
	case contextTables:
		return m.t.T(i18n.KeyPaneOverview)
	default:
		return m.t.T(i18n.KeyPaneResults)
	}
}

func (m *Model) resultsPaneHint() string {
	switch m.context {
	case contextInfo:
		return m.t.T(i18n.KeyHintOverview)
	case contextTables:
		if m.filtering {
			return m.t.T(i18n.KeyHintFilter) + m.filterInput.View()
		}
		return m.t.T(i18n.KeyHintOverview)
	case contextData, contextQuery:
		if m.filtering {
			return m.t.T(i18n.KeyHintFilter) + m.filterInput.View()
		}
		return m.t.T(i18n.KeyHintResults)
	default:
		return m.t.T(i18n.KeyHintResults)
	}
}

func (m *Model) statusLeft() string {
	parts := make([]string, 0, 3)
	switch m.focus {
	case focusSidebar:
		parts = append(parts, m.t.T(i18n.KeyPaneConnections))
	case focusEditor:
		parts = append(parts, m.t.T(i18n.KeyPaneSQL))
	case focusResults:
		parts = append(parts, m.t.T(i18n.KeyPaneResults))
	}
	if crumb := m.breadcrumb(); crumb != "" {
		parts = append(parts, crumb)
	}
	if title := m.activeTabTitle(); title != "" {
		parts = append(parts, title)
	}
	return " " + strings.Join(parts, "  │  ") + " "
}

func (m *Model) breadcrumb() string {
	if m.connName == "" {
		return ""
	}
	parts := []string{sessionBase(m.connName)}
	database := sessionDatabase(m.connName)
	if database != "" {
		parts = append(parts, database)
	}
	schema, table := "", ""
	if m.pager != nil {
		schema, table = m.pager.schema, m.pager.table
	} else if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		schema, table = m.tabs[m.activeTab].source.schema, m.tabs[m.activeTab].source.table
	}
	if table != "" {
		if schema != "" && schema != "main" && schema != database {
			parts = append(parts, schema)
		}
		parts = append(parts, table)
	}
	return strings.Join(parts, " / ")
}

func (m *Model) statusRight() string {
	if m.status == "" {
		return ""
	}
	icon := "•"
	switch {
	case m.statusErr:
		icon = "✗"
	case m.running || m.exporting || m.importing:
		icon = "…"
	case m.status == m.t.T(i18n.KeyStatusReady):
		icon = "✓"
	}
	return icon + " " + m.status + " "
}

func renderPane(title, hint, body string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	left := theme.Title.Render(" " + title)
	right := ""
	if hint != "" && textutil.Width(hint)+textutil.Width(title)+6 < width {
		right = theme.Muted.Render(hint + " ")
	}
	gap := width - textutil.Width(left) - textutil.Width(right)
	if gap < 0 {
		gap = 0
	}
	titleLine := textutil.FitLine(left+strings.Repeat(" ", gap)+right, width)
	if height == 1 {
		return titleLine
	}
	return titleLine + "\n" + textutil.FitLines(body, width, height-1)
}

func center(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	contentW := 0
	for _, line := range lines {
		contentW = max(contentW, textutil.Width(line))
	}
	left := max(0, (width-contentW)/2)
	top := max(0, (height-len(lines))/2)
	out := make([]string, 0, height)
	for i := 0; i < top && len(out) < height; i++ {
		out = append(out, "")
	}
	for _, line := range lines {
		out = append(out, strings.Repeat(" ", left)+line)
	}
	return textutil.FitLines(strings.Join(out, "\n"), width, height)
}

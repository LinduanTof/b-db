package app

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/ui/editor"
	"github.com/LinduanTof/b-db/internal/ui/textutil"
	"github.com/LinduanTof/b-db/internal/ui/theme"
)

type tableSource struct {
	schema string
	table  string
}

type editorTab struct {
	title  string
	editor editor.Model
	source tableSource
}

type tabSaveMsg struct {
	gen uint64
}

func (m *Model) resetTabs() {
	used := map[string]bool{}
	title := nextTabTitle(m.t, used)
	ed := editor.New(m.t)
	if m.history != nil {
		ed.SetHistory(m.history.Entries())
	}
	m.tabs = []editorTab{{title: title, editor: ed}}
	m.activeTab = 0
	m.editor = ed
}

func (m *Model) syncActiveTab() {
	if len(m.tabs) == 0 {
		return
	}
	if m.activeTab < 0 {
		m.activeTab = 0
	}
	if m.activeTab >= len(m.tabs) {
		m.activeTab = len(m.tabs) - 1
	}
	m.tabs[m.activeTab].editor = m.editor
}

func (m *Model) tabsBlank() bool {
	return len(m.tabs) == 1 && strings.TrimSpace(m.editor.Value()) == ""
}

func (m *Model) restoreTabsFor(name string) {
	saved, ok := m.tabStates[name]
	if !ok || len(saved.Tabs) == 0 || !m.tabsBlank() {
		return
	}
	used := make(map[string]bool, len(saved.Tabs))
	tabs := make([]editorTab, 0, len(saved.Tabs))
	for _, savedTab := range saved.Tabs {
		ed := editor.New(m.t)
		if savedTab.Content != "" {
			ed.SetValue(savedTab.Content)
		}
		if m.history != nil {
			ed.SetHistory(m.history.Entries())
		}
		title := savedTab.Title
		if title == "" || used[title] {
			title = nextTabTitle(m.t, used)
		}
		used[title] = true
		tabs = append(tabs, editorTab{title: title, editor: ed})
	}
	if len(tabs) == 0 {
		return
	}
	m.tabs = tabs
	m.activeTab = saved.Active
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		m.activeTab = 0
	}
	m.editor = m.tabs[m.activeTab].editor
	m.layout()
}

func (m *Model) newTab() tea.Cmd {
	m.syncActiveTab()
	m.completion.Close()
	ed := editor.New(m.t)
	if m.history != nil {
		ed.SetHistory(m.history.Entries())
	}
	m.tabs = append(m.tabs, editorTab{title: m.nextTabTitle(), editor: ed})
	m.activeTab = len(m.tabs) - 1
	m.editor = ed
	m.layout()
	return tea.Batch(m.editor.Focus(), m.scheduleTabSave())
}

func (m *Model) closeTab() tea.Cmd {
	m.syncActiveTab()
	m.completion.Close()
	if len(m.tabs) <= 1 {
		m.setStatus(m.t.T(i18n.KeyStatusLastTab), false)
		return nil
	}
	index := m.activeTab
	m.tabs = append(m.tabs[:index], m.tabs[index+1:]...)
	if m.activeTab >= len(m.tabs) {
		m.activeTab = len(m.tabs) - 1
	}
	m.editor = m.tabs[m.activeTab].editor
	if m.history != nil {
		m.editor.SetHistory(m.history.Entries())
	}
	m.layout()
	return tea.Batch(m.editor.Focus(), m.scheduleTabSave())
}

func (m *Model) switchTab(delta int) tea.Cmd {
	if len(m.tabs) <= 1 {
		return nil
	}
	m.syncActiveTab()
	m.completion.Close()
	m.activeTab = (m.activeTab + delta + len(m.tabs)) % len(m.tabs)
	m.editor = m.tabs[m.activeTab].editor
	if m.history != nil {
		m.editor.SetHistory(m.history.Entries())
	}
	m.layout()
	return tea.Batch(m.editor.Focus(), m.scheduleTabSave())
}

func (m *Model) nextTabTitle() string {
	used := make(map[string]bool, len(m.tabs))
	for _, tab := range m.tabs {
		used[tab.title] = true
	}
	return nextTabTitle(m.t, used)
}

func (m *Model) activeTabTitle() string {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return ""
	}
	return m.tabs[m.activeTab].title
}

func nextTabTitle(t *i18n.Translator, used map[string]bool) string {
	for n := 1; ; n++ {
		title := fmt.Sprintf(t.T(i18n.KeyTabUntitled), n)
		if !used[title] {
			return title
		}
	}
}

func (m *Model) tabBar(width int) string {
	if width <= 0 || len(m.tabs) == 0 {
		return ""
	}
	var b strings.Builder
	for i, tab := range m.tabs {
		label := fmt.Sprintf(" %d:%s ", i+1, tab.title)
		if i == m.activeTab {
			b.WriteString(theme.Title.Render(label))
		} else {
			b.WriteString(theme.Muted.Render(label))
		}
		if i < len(m.tabs)-1 {
			b.WriteString(theme.Muted.Render("│"))
		}
	}
	return textutil.FitLine(b.String(), width)
}

func (m *Model) scheduleTabSave() tea.Cmd {
	if m.tabSavePending {
		return nil
	}
	m.tabSavePending = true
	m.tabSaveGen++
	gen := m.tabSaveGen
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return tabSaveMsg{gen: gen}
	})
}

func savedTabsFrom(tabs []editorTab, active int) savedTabs {
	out := savedTabs{Active: active, Tabs: make([]savedTab, 0, len(tabs))}
	for _, tab := range tabs {
		out.Tabs = append(out.Tabs, savedTab{Title: tab.title, Content: tab.editor.Value()})
	}
	return out
}

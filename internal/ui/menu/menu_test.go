package menu

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/dbrix/internal/i18n"
)

func testItems() []Item {
	return []Item{
		{Kind: KindHeader, LabelKey: i18n.KeyMenuActions},
		{ID: "add", LabelKey: i18n.KeyCmdAdd, Kind: KindAction},
		{ID: "disconnect", LabelKey: i18n.KeyCmdDisconnect, Kind: KindAction},
		{Kind: KindHeader, LabelKey: i18n.KeyMenuSettings},
		{ID: "theme", LabelKey: i18n.KeyMenuTheme, Kind: KindChoice, Choices: []string{"default", "dracula", "nord"}, Value: 0},
		{ID: "lang", LabelKey: i18n.KeyMenuLanguage, Kind: KindChoice, Choices: []string{"en", "zh"}, Value: 0},
	}
}

func newOpen() Model {
	m := New(i18n.New("en"))
	m.SetSize(100, 30)
	m.Open("", testItems())
	return m
}

func TestOpenSelectsFirstAction(t *testing.T) {
	m := newOpen()
	if m.selected != 1 {
		t.Fatalf("selected = %d, want 1", m.selected)
	}
	view := m.View()
	for _, want := range []string{"Options", "ACTIONS", "SETTINGS", "Theme", "default"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestMoveSkipsHeaders(t *testing.T) {
	m := newOpen()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.selected != 2 {
		t.Fatalf("selected = %d, want 2", m.selected)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.selected != 1 {
		t.Fatalf("selected = %d, want 1", m.selected)
	}
}

func TestChoiceChange(t *testing.T) {
	m := newOpen()
	m.selected = 4
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = model
	if cmd == nil {
		t.Fatal("expected change command")
	}
	msg, ok := cmd().(ChangedMsg)
	if !ok || msg.ID != "theme" || msg.Value != 1 {
		t.Fatalf("message = %+v", msg)
	}
	if !strings.Contains(m.View(), "dracula") {
		t.Fatalf("view should show new value:\n%s", m.View())
	}
}

func TestActionAndEsc(t *testing.T) {
	m := newOpen()
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model
	if cmd == nil {
		t.Fatal("expected action command")
	}
	if msg := cmd().(ActionMsg); msg.ID != "add" {
		t.Fatalf("action = %+v", msg)
	}
	if m.IsOpen() {
		t.Fatal("menu should close after an action")
	}

	m = newOpen()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() {
		t.Fatal("esc should close the menu")
	}
}

func menuPoint(m Model, itemIndex int) (int, int) {
	_, itemLines := m.bodyLines(m.innerWidth())
	lines := strings.Split(m.View(), "\n")
	modalW := 0
	for _, line := range lines {
		modalW = max(modalW, ansi.StringWidth(line))
	}
	x := max(0, (100-modalW)/2) + 3
	y := max(0, (30-len(lines))/2) + itemLines[itemIndex] + 1
	return x, y
}

func TestMouseClickSelectsThenActivates(t *testing.T) {
	m := newOpen()
	x, y := menuPoint(m, 2)

	model, cmd := m.HandleClick(x, y, 100, 30)
	m = model
	if cmd != nil {
		t.Fatal("first click should only select")
	}
	if !m.IsOpen() || m.selected != 2 {
		t.Fatalf("menu open=%v selected=%d", m.IsOpen(), m.selected)
	}

	model, cmd = m.HandleClick(x, y, 100, 30)
	m = model
	if cmd == nil {
		t.Fatal("second click should activate the action")
	}
	if msg := cmd().(ActionMsg); msg.ID != "disconnect" {
		t.Fatalf("action = %+v", msg)
	}
	if m.IsOpen() {
		t.Fatal("action click should close the menu")
	}
}

func TestMouseClickChoiceSelectsThenChanges(t *testing.T) {
	m := newOpen()
	x, y := menuPoint(m, 4)

	model, cmd := m.HandleClick(x, y, 100, 30)
	m = model
	if cmd != nil || m.selected != 4 {
		t.Fatalf("first click: cmd=%v selected=%d", cmd, m.selected)
	}

	model, cmd = m.HandleClick(x, y, 100, 30)
	m = model
	if cmd == nil {
		t.Fatal("second click should change the choice")
	}
	if msg := cmd().(ChangedMsg); msg.ID != "theme" || msg.Value != 1 {
		t.Fatalf("message = %+v", msg)
	}
	if !m.IsOpen() {
		t.Fatal("choice click should keep the menu open")
	}
}

func TestMouseClickOutsideCloses(t *testing.T) {
	m := newOpen()
	model, _ := m.HandleClick(0, 0, 100, 30)
	if model.IsOpen() {
		t.Fatal("click outside should close the menu")
	}
}

func TestMouseWheelMovesSelection(t *testing.T) {
	m := newOpen()
	m.HandleWheel(1)
	if m.selected != 2 {
		t.Fatalf("selected = %d, want 2", m.selected)
	}
	m.HandleWheel(-1)
	if m.selected != 1 {
		t.Fatalf("selected = %d, want 1", m.selected)
	}
}

func TestViewLinesShareWidth(t *testing.T) {
	m := newOpen()
	lines := strings.Split(m.View(), "\n")
	w := ansi.StringWidth(lines[0])
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != w {
			t.Fatalf("line %d width = %d, want %d", i, got, w)
		}
	}
}

func tabbedMenu() Model {
	m := New(i18n.New("en"))
	m.SetSize(100, 30)
	m.OpenTabs("", []string{"Actions", "Settings"}, []Item{
		{ID: "add", LabelKey: i18n.KeyCmdAdd, Kind: KindAction},
		{ID: "theme", LabelKey: i18n.KeyMenuTheme, Kind: KindChoice, Choices: []string{"default", "dracula"}, Tab: 1},
	})
	return m
}

func TestTabsSwitch(t *testing.T) {
	m := tabbedMenu()
	if m.tab != 0 || m.selected != 0 {
		t.Fatalf("tab=%d selected=%d", m.tab, m.selected)
	}
	if !strings.Contains(m.View(), "Actions") || !strings.Contains(m.View(), "Settings") {
		t.Fatalf("missing tabs:\n%s", m.View())
	}
	if !strings.Contains(m.View(), "add a new data source") || strings.Contains(m.View(), "Theme") {
		t.Fatalf("actions tab content wrong:\n%s", m.View())
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != 1 || m.selected != 1 {
		t.Fatalf("after tab: tab=%d selected=%d", m.tab, m.selected)
	}
	if !strings.Contains(m.View(), "Theme") || strings.Contains(m.View(), "add a new data source") {
		t.Fatalf("settings tab content wrong:\n%s", m.View())
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.tab != 0 {
		t.Fatalf("tab = %d, want 0", m.tab)
	}
}

func TestClickTab(t *testing.T) {
	m := tabbedMenu()
	lines := strings.Split(m.View(), "\n")
	modalW := 0
	for _, line := range lines {
		modalW = max(modalW, ansi.StringWidth(line))
	}
	x0 := max(0, (100-modalW)/2)
	y0 := max(0, (30-len(lines))/2)
	span := m.tabRanges[1]
	x := x0 + 2 + span[0] + 1
	y := y0 + 1 + m.tabLine

	model, _ := m.HandleClick(x, y, 100, 30)
	if model.tab != 1 {
		t.Fatalf("tab = %d, want 1", model.tab)
	}
}

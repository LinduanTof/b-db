package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/i18n"
)

func ctrlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}

func TestEditorTabs(t *testing.T) {
	m := testModel(t, "en")
	m.focus = focusEditor
	m.editor.SetValue("SELECT 1;")

	m = sendKey(t, m, ctrlKey('t'))
	if len(m.tabs) != 2 || m.activeTab != 1 || m.editor.Value() != "" {
		t.Fatalf("new tab state: tabs=%d active=%d value=%q", len(m.tabs), m.activeTab, m.editor.Value())
	}
	if m.tabs[0].editor.Value() != "SELECT 1;" {
		t.Fatalf("first tab content = %q", m.tabs[0].editor.Value())
	}
	if content := m.View().Content; !strings.Contains(content, "Query 1") || !strings.Contains(content, "Query 2") {
		t.Fatalf("tab bar missing titles:\n%s", content)
	}

	m.editor.SetValue("SELECT 2;")
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModCtrl})
	if m.activeTab != 0 || m.editor.Value() != "SELECT 1;" {
		t.Fatalf("ctrl+tab state: active=%d value=%q", m.activeTab, m.editor.Value())
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModCtrl | tea.ModShift})
	if m.activeTab != 1 || m.editor.Value() != "SELECT 2;" {
		t.Fatalf("ctrl+shift+tab state: active=%d value=%q", m.activeTab, m.editor.Value())
	}

	m = sendKey(t, m, ctrlKey('w'))
	if len(m.tabs) != 1 || m.activeTab != 0 || m.editor.Value() != "SELECT 1;" {
		t.Fatalf("close tab state: tabs=%d active=%d value=%q", len(m.tabs), m.activeTab, m.editor.Value())
	}
	m = sendKey(t, m, ctrlKey('w'))
	if !strings.Contains(m.status, "Cannot close the last tab") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestEditorTabsPersist(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.shutdown()

	m.focus = focusEditor
	m.editor.SetValue("SELECT 1;")
	m = sendKey(t, m, ctrlKey('t'))
	m.editor.SetValue("SELECT 2;")
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModCtrl | tea.ModShift})
	if m.activeTab != 0 {
		t.Fatalf("active tab = %d", m.activeTab)
	}
	if err := m.saveTabStatesNow(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(tabStatePath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("tab state permissions = %o", info.Mode().Perm())
	}

	m2 := New(m.cfg, i18n.New("en"))
	model, _ := m2.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m2 = model.(*Model)
	m2 = runCmd(t, m2, m2.connectCmd("local-sqlite"))
	defer m2.shutdown()

	if len(m2.tabs) != 2 || m2.activeTab != 0 || m2.editor.Value() != "SELECT 1;" {
		t.Fatalf("restored tabs: tabs=%d active=%d value=%q", len(m2.tabs), m2.activeTab, m2.editor.Value())
	}
	if m2.tabs[1].editor.Value() != "SELECT 2;" {
		t.Fatalf("restored second tab = %q", m2.tabs[1].editor.Value())
	}
	m2.focus = focusEditor
	m2 = sendKey(t, m2, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModCtrl})
	if m2.activeTab != 1 || m2.editor.Value() != "SELECT 2;" {
		t.Fatalf("restored active tab: active=%d value=%q", m2.activeTab, m2.editor.Value())
	}
}

func TestEditorTabsAutoSave(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.shutdown()

	m.focus = focusEditor
	m.editor.SetValue("SELECT 9;")
	m.tabSavePending = false
	if cmd := m.scheduleTabSave(); cmd == nil {
		t.Fatal("expected a debounce command")
	}
	model, cmd := m.Update(tabSaveMsg{gen: m.tabSaveGen})
	m = model.(*Model)
	if cmd != nil {
		t.Fatalf("auto save should not return a command: %#v", cmd)
	}
	states, err := loadTabStates()
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := states["local-sqlite"]
	if !ok || len(saved.Tabs) != 1 || saved.Tabs[0].Content != "SELECT 9;" {
		t.Fatalf("saved tabs = %#v", saved)
	}
}

func TestSessionTabsAreIsolated(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.shutdown()

	m.focus = focusEditor
	m.editor.SetValue("SELECT A;")
	m = sendKey(t, m, ctrlKey('t'))
	m.editor.SetValue("SELECT B;")

	data, err := os.ReadFile(m.cfg.Connections[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(t.TempDir(), "second.db")
	if err := os.WriteFile(secondPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	m = runCmd(t, m, m.addConnection(config.Connection{
		Name: "second",
		Type: config.TypeSQLite,
		Path: secondPath,
	}))
	if m.connName != "second" || m.conn == nil {
		t.Fatalf("connection = %q, status=%s", m.connName, m.status)
	}
	if len(m.tabs) != 1 || m.editor.Value() != "" {
		t.Fatalf("new session tabs: tabs=%d value=%q", len(m.tabs), m.editor.Value())
	}

	m = runCmd(t, m, m.switchToConnection("local-sqlite"))
	if m.connName != "local-sqlite" || m.conn == nil {
		t.Fatalf("connection = %q", m.connName)
	}
	if len(m.tabs) != 2 || m.activeTab != 1 || m.editor.Value() != "SELECT B;" {
		t.Fatalf("restored tabs: tabs=%d active=%d value=%q", len(m.tabs), m.activeTab, m.editor.Value())
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModCtrl | tea.ModShift})
	if m.editor.Value() != "SELECT A;" {
		t.Fatalf("first tab = %q", m.editor.Value())
	}
}

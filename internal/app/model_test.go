package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/linduan/bdb/internal/config"
	"github.com/linduan/bdb/internal/history"
	"github.com/linduan/bdb/internal/i18n"
	uicomplete "github.com/linduan/bdb/internal/ui/complete"
	"github.com/linduan/bdb/internal/ui/connection"
	"github.com/linduan/bdb/internal/ui/menu"
	"github.com/linduan/bdb/internal/ui/sidebar"
	"github.com/linduan/bdb/internal/ui/theme"
)

func testModel(t *testing.T, lang string) *Model {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dbPath, err := filepath.Abs(filepath.Join("..", "..", "testdata", "demo.db"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		DefaultConnection: "local-sqlite",
		UI:                config.UIConfig{Language: lang},
		Query:             config.QueryConfig{Timeout: config.Duration(5 * time.Second), MaxRows: 100},
		Connections: []config.Connection{
			{Name: "local-sqlite", Type: config.TypeSQLite, Path: dbPath},
		},
	}
	m := New(cfg, i18n.New(lang))
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return model.(*Model)
}

func runCmd(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()
	const maxSteps = 10
	for i := 0; i < maxSteps && cmd != nil; i++ {
		msg := cmd()
		if msg == nil {
			return m
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				m = runCmd(t, m, sub)
			}
			return m
		}
		switch msg.(type) {
		case connOpenedMsg, queryDoneMsg, exportedMsg, detailLoadedMsg, scriptDoneMsg,
			completionTablesMsg, completionColumnsMsg, uicomplete.AcceptedMsg, connection.SubmittedMsg,
			sidebar.SchemasLoadedMsg, sidebar.TablesLoadedMsg, sidebar.ColumnsLoadedMsg,
			sidebar.ConnectMsg, sidebar.SelectTableMsg, sidebar.TableMenuMsg,
			ddlLoadedMsg, indexesLoadedMsg, menu.ActionMsg, menu.ChangedMsg:
		default:
			return m
		}
		model, next := m.Update(msg)
		m = model.(*Model)
		cmd = next
	}
	return m
}

func sendKey(t *testing.T, m *Model, msg tea.KeyPressMsg) *Model {
	t.Helper()
	model, cmd := m.Update(msg)
	return runCmd(t, model.(*Model), cmd)
}

func connect(t *testing.T, m *Model) *Model {
	t.Helper()
	cmd := m.connectCmd("local-sqlite")
	model, next := m.Update(cmd())
	m = runCmd(t, model.(*Model), next)
	if m.conn == nil {
		t.Fatalf("connection failed: %s", m.status)
	}
	return m
}

func TestEscOpensOptionsMenu(t *testing.T) {
	m := testModel(t, "en")
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if !m.options.IsOpen() {
		t.Fatal("esc should open the options menu")
	}
	content := m.View().Content
	for _, want := range []string{"Options", "Actions", "Settings", "Theme", "Language"} {
		if !strings.Contains(content, want) {
			t.Fatalf("menu missing %q:\n%s", want, content)
		}
	}
	if !strings.Contains(content, "Connections") {
		t.Fatalf("overlay should keep the background visible:\n%s", content)
	}
	lines := strings.Split(content, "\n")
	if len(lines) != 30 {
		t.Fatalf("lines = %d, want 30", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != 100 {
			t.Fatalf("line %d width = %d, want 100", i, w)
		}
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.options.IsOpen() {
		t.Fatal("esc should close the menu")
	}
}

func TestQuitKeys(t *testing.T) {
	m := testModel(t, "en")
	_, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	if cmd == nil {
		t.Fatal("q should request quit outside the editor")
	}

	m.focus = focusEditor
	_ = m.editor.Focus()
	m.editor.SetValue("")
	model, _ := m.Update(tea.KeyPressMsg{Text: "q"})
	m = model.(*Model)
	if got := m.editor.Value(); got != "q" {
		t.Fatalf("editor value = %q, want q", got)
	}
}

func TestStatusbarMenuHint(t *testing.T) {
	m := testModel(t, "en")
	for _, want := range []string{"Esc menu", "q quit", "? help"} {
		if !strings.Contains(m.View().Content, want) {
			t.Fatalf("status bar missing %q", want)
		}
	}
}

func TestHelpShowsShortcuts(t *testing.T) {
	m := testModel(t, "en")
	m = sendKey(t, m, tea.KeyPressMsg{Text: "?"})
	content := m.View().Content
	for _, want := range []string{"Keyboard shortcuts", "esc: options menu", "ctrl+enter"} {
		if !strings.Contains(content, want) {
			t.Fatalf("help missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, ":connect") {
		t.Fatalf("help should not advertise command mode:\n%s", content)
	}
}

func TestOptionsMenu(t *testing.T) {
	m := testModel(t, "en")
	defer theme.Set("default")

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	for i := 0; i < 6; i++ {
		m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if theme.Current() != "catppuccin" {
		t.Fatalf("theme = %q, want catppuccin", theme.Current())
	}
	if !strings.Contains(m.View().Content, "catppuccin") {
		t.Fatal("menu should show the updated theme")
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.t.Lang() != i18n.Zh {
		t.Fatalf("lang = %q, want zh", m.t.Lang())
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.options.IsOpen() {
		t.Fatal("esc should close the menu")
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.connForm.IsOpen() {
		t.Fatal("add action should open the connection form")
	}
}

func TestMenuDisconnect(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.conn != nil || m.connName != "" {
		t.Fatalf("connection not closed: conn=%v name=%q", m.conn, m.connName)
	}
	if strings.Contains(m.sidebar.View(), "main") {
		t.Fatalf("sidebar still shows schema after disconnect:\n%s", m.sidebar.View())
	}
}

func TestTableDetail(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	for i := 0; i < 3; i++ {
		m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m = sendKey(t, m, tea.KeyPressMsg{Text: "d"})
	if !m.detail.IsOpen() {
		t.Fatalf("detail should open, status=%s", m.status)
	}
	content := m.View().Content
	for _, want := range []string{"Table structure: users", "email", "Indexes", "Foreign keys"} {
		if !strings.Contains(content, want) {
			t.Fatalf("detail missing %q:\n%s", want, content)
		}
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.detail.IsOpen() {
		t.Fatal("esc should close detail")
	}
}

func TestScriptExecution(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	path := filepath.Join(t.TempDir(), "script.sql")
	if err := os.WriteFile(path, []byte("SELECT 1 AS a;\n-- comment;\nSELECT 2 AS b;"), 0o600); err != nil {
		t.Fatal(err)
	}
	m = runCmd(t, m, m.runScriptCmd(path))
	if !strings.Contains(m.status, "2/2") {
		t.Fatalf("status = %q", m.status)
	}
	rs := m.results.Result()
	if rs == nil || len(rs.Rows) != 1 {
		t.Fatalf("result = %#v", rs)
	}
	if got := rs.Rows[0][0]; got != int64(2) {
		t.Fatalf("last result = %#v, want 2", got)
	}
}

func TestCopyCellAndMarkdownExport(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.outDir = t.TempDir()

	m.editor.SetValue("SELECT name FROM users ORDER BY id LIMIT 1;")
	m.focus = focusEditor
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = runCmd(t, model.(*Model), cmd)
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})

	m = sendKey(t, m, tea.KeyPressMsg{Text: "c"})
	if !strings.Contains(m.status, "Copied") {
		t.Fatalf("copy status = %q", m.status)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: 'm', Mod: tea.ModCtrl})
	entries, err := os.ReadDir(m.outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".md") {
		t.Fatalf("exports = %v", entries)
	}
	data, err := os.ReadFile(filepath.Join(m.outDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "| name |") || !strings.Contains(string(data), "张三") {
		t.Fatalf("markdown export:\n%s", data)
	}
}

func TestHistoryStore(t *testing.T) {
	m := testModel(t, "en")
	store := history.Load(filepath.Join(t.TempDir(), "history"), 10)
	m.SetHistory(store)

	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.editor.SetValue("SELECT 1;")
	m.focus = focusEditor
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = runCmd(t, model.(*Model), cmd)

	if entries := store.Entries(); len(entries) != 1 || entries[0] != "SELECT 1;" {
		t.Fatalf("history = %#v", entries)
	}
}

func TestSQLCompletion(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.focus = focusEditor
	_ = m.editor.Focus()
	m.editor.SetValue("sel")

	m = sendKey(t, m, tea.KeyPressMsg{Code: ' ', Mod: tea.ModCtrl})
	if !m.completion.IsOpen() {
		t.Fatalf("completion should open, status=%s", m.status)
	}
	content := m.View().Content
	if !strings.Contains(content, "Completions: sel") || !strings.Contains(content, "SELECT") {
		t.Fatalf("completion view:\n%s", content)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := m.editor.Value(); got != "SELECT" {
		t.Fatalf("value = %q, want SELECT", got)
	}
}

func TestAutoCompletionTables(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.focus = focusEditor
	_ = m.editor.Focus()
	m.editor.SetValue("SELECT * FROM us")

	m = sendKey(t, m, tea.KeyPressMsg{Text: "e"})
	if !m.completion.IsOpen() {
		t.Fatalf("completion should open automatically, status=%s", m.status)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := m.editor.Value(); got != "SELECT * FROM users" {
		t.Fatalf("value = %q", got)
	}
}

func TestColumnCompletion(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.focus = focusEditor
	_ = m.editor.Focus()
	m.editor.SetValue("SELECT users.")

	m = sendKey(t, m, tea.KeyPressMsg{Code: ' ', Mod: tea.ModCtrl})
	m = sendKey(t, m, tea.KeyPressMsg{Code: ' ', Mod: tea.ModCtrl})
	if !m.completion.IsOpen() {
		t.Fatalf("column completion should open, status=%s", m.status)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Text: "na"})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := m.editor.Value(); got != "SELECT users.name" {
		t.Fatalf("value = %q", got)
	}
}

func TestAddConnectionFromUI(t *testing.T) {
	m := testModel(t, "en")
	m = sendKey(t, m, tea.KeyPressMsg{Text: "a"})
	if !m.connForm.IsOpen() {
		t.Fatal("'a' should open the connection form")
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = sendKey(t, m, tea.KeyPressMsg{Text: "smoke"})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	path := filepath.Join(t.TempDir(), "smoke.db")
	m = sendKey(t, m, tea.KeyPressMsg{Text: path})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.connName != "smoke" || m.conn == nil {
		t.Fatalf("connection = %q, status=%s", m.connName, m.status)
	}
	defer m.conn.Close()
	if !strings.Contains(m.sidebar.View(), "smoke") {
		t.Fatalf("sidebar missing new connection:\n%s", m.sidebar.View())
	}
	conns, err := config.LoadRuntimeConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 || conns[0].Name != "smoke" || conns[0].Path != path {
		t.Fatalf("saved connections = %+v", conns)
	}
}

func TestAddConnectionDuplicate(t *testing.T) {
	m := testModel(t, "en")
	m.addConnection(config.Connection{Name: "local-sqlite", Type: config.TypeSQLite, Path: "./x.db"})
	if !m.connForm.IsOpen() {
		t.Fatal("form should stay open on duplicate name")
	}
	if !strings.Contains(m.connForm.View(), "already exists") {
		t.Fatalf("form should show duplicate error:\n%s", m.connForm.View())
	}
}

func TestAddConnectionCancel(t *testing.T) {
	m := testModel(t, "en")
	m = sendKey(t, m, tea.KeyPressMsg{Text: "a"})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.connForm.IsOpen() {
		t.Fatal("esc should close the form")
	}
}

func TestTableActionsMenu(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	for i := 0; i < 3; i++ {
		m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.tableMenu.IsOpen() {
		t.Fatalf("table menu should open, status=%s", m.status)
	}
	content := m.View().Content
	for _, want := range []string{"users", "View data", "View DDL", "View structure", "View indexes", "Generate SELECT"} {
		if !strings.Contains(content, want) {
			t.Fatalf("table menu missing %q:\n%s", want, content)
		}
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.textView.IsOpen() {
		t.Fatalf("DDL viewer should open, status=%s", m.status)
	}
	ddlView := m.View().Content
	if !strings.Contains(ddlView, "DDL: users") || !strings.Contains(ddlView, "CREATE TABLE users") {
		t.Fatalf("DDL view:\n%s", ddlView)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.textView.IsOpen() {
		t.Fatal("esc should close the DDL viewer")
	}
}

func TestTableDataAction(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	for i := 0; i < 3; i++ {
		m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	rs := m.results.Result()
	if rs == nil || len(rs.Rows) != 5 {
		t.Fatalf("result = %#v, status=%s", rs, m.status)
	}
	if !strings.Contains(m.editor.Value(), "users") {
		t.Fatalf("editor = %q", m.editor.Value())
	}
}

func TestTableIndexesAction(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	for i := 0; i < 3; i++ {
		m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	for i := 0; i < 3; i++ {
		m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.textView.IsOpen() {
		t.Fatalf("index viewer should open, status=%s", m.status)
	}
	if !strings.Contains(m.View().Content, "Indexes: users") {
		t.Fatalf("index view:\n%s", m.View().Content)
	}
}

func TestMouseClickSidebar(t *testing.T) {
	m := testModel(t, "en")
	model, cmd := m.Update(tea.MouseClickMsg{X: 3, Y: 1, Button: tea.MouseLeft})
	m = runCmd(t, model.(*Model), cmd)
	if m.conn == nil {
		t.Fatalf("click should connect, status=%s", m.status)
	}
	defer m.conn.Close()
	if m.focus != focusSidebar {
		t.Fatalf("focus = %v, want sidebar", m.focus)
	}
}

func TestMouseClickResults(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m.editor.SetValue("SELECT id, name FROM users ORDER BY id;")
	m.focus = focusEditor
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = runCmd(t, model.(*Model), cmd)

	x := m.sideW + 1 + 7
	y := m.editorH + 2 + 3
	model, _ = m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = model.(*Model)
	if m.focus != focusResults {
		t.Fatalf("focus = %v, want results", m.focus)
	}
	row, col := m.results.CursorPosition()
	if row != 0 || col != 0 {
		t.Fatalf("cursor = %d,%d, want 0,0", row, col)
	}
	if got := m.results.CursorCell(); got != "1" {
		t.Fatalf("cell = %q, want 1", got)
	}

	model, _ = m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	m = model.(*Model)
	if row, _ := m.results.CursorPosition(); row != 3 {
		t.Fatalf("wheel row = %d, want 3", row)
	}
}

func TestMenuSettings(t *testing.T) {
	m := testModel(t, "en")
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	for i := 0; i < 8; i++ {
		m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if got := m.cfg.Query.Timeout.Std(); got != 10*time.Second {
		t.Fatalf("timeout = %s, want 10s", got)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.cfg.Query.MaxRows != 500 {
		t.Fatalf("max rows = %d, want 500", m.cfg.Query.MaxRows)
	}
	if m.executor.MaxRows != 500 {
		t.Fatalf("executor max rows = %d, want 500", m.executor.MaxRows)
	}
}

func TestKeyStrings(t *testing.T) {
	if got := (tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl}).String(); got != "ctrl+enter" {
		t.Fatalf("ctrl+enter = %q", got)
	}
	if got := (tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}).String(); got != "shift+tab" {
		t.Fatalf("shift+tab = %q", got)
	}
}

func TestLayoutAndFocus(t *testing.T) {
	m := testModel(t, "en")
	content := m.View().Content
	if !strings.Contains(content, "Connections") || !strings.Contains(content, "SQL") || !strings.Contains(content, "Results") {
		t.Fatalf("panes missing:\n%s", content)
	}
	if got := strings.Count(content, "\n") + 1; got != 30 {
		t.Fatalf("rendered %d lines, want 30", got)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focus != focusEditor {
		t.Fatalf("focus = %v, want editor", m.focus)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focus != focusResults {
		t.Fatalf("focus = %v, want results", m.focus)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.focus != focusEditor {
		t.Fatalf("focus = %v, want editor after shift+tab", m.focus)
	}
}

func TestQueryFlow(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	if !strings.Contains(m.sidebar.View(), "main") {
		t.Fatalf("sidebar should show schema main:\n%s", m.sidebar.View())
	}

	m.editor.SetValue("SELECT id, name, email FROM users ORDER BY id;")
	m.focus = focusEditor
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = runCmd(t, model.(*Model), cmd)

	rs := m.results.Result()
	if rs == nil {
		t.Fatalf("no result, status=%s", m.status)
	}
	if len(rs.Rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(rs.Rows))
	}
	content := m.View().Content
	if !strings.Contains(content, "张三") {
		t.Fatalf("rendered view should contain CJK data:\n%s", content)
	}
	if !strings.Contains(m.status, "5 row(s)") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestSidebarBrowseAndSelect(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if !strings.Contains(m.sidebar.View(), "users") {
		t.Fatalf("sidebar should list tables:\n%s", m.sidebar.View())
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: 's'})
	if m.focus != focusEditor {
		t.Fatalf("focus = %v, want editor after select", m.focus)
	}
	if got := m.editor.Value(); !strings.Contains(got, "users") {
		t.Fatalf("editor value = %q", got)
	}
}

func TestExportCSV(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.outDir = t.TempDir()

	m.editor.SetValue("SELECT id, name FROM users ORDER BY id;")
	m.focus = focusEditor
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = runCmd(t, model.(*Model), cmd)
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})

	model, cmd = m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	m = runCmd(t, model.(*Model), cmd)
	if m.results.Result() == nil {
		t.Fatal("result lost after export")
	}
	if !strings.Contains(m.status, "bdb-export-") {
		t.Fatalf("status = %q", m.status)
	}
}

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/history"
	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/ui/celledit"
	uicomplete "github.com/LinduanTof/b-db/internal/ui/complete"
	"github.com/LinduanTof/b-db/internal/ui/confirm"
	"github.com/LinduanTof/b-db/internal/ui/connection"
	"github.com/LinduanTof/b-db/internal/ui/menu"
	"github.com/LinduanTof/b-db/internal/ui/sidebar"
	"github.com/LinduanTof/b-db/internal/ui/theme"
)

func testModel(t *testing.T, lang string) *Model {
	return testModelWithFocus(t, lang, config.FocusModeManual)
}

func testModelWithFocus(t *testing.T, lang, focusMode string) *Model {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	srcPath, err := filepath.Abs(filepath.Join("..", "..", "testdata", "demo.db"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "demo.db")
	if err := os.WriteFile(dbPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		DefaultConnection: "local-sqlite",
		UI:                config.UIConfig{Language: lang, FocusMode: focusMode},
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
		case connOpenedMsg, queryDoneMsg, tablePageDoneMsg, databasesLoadedMsg, exportedMsg, detailLoadedMsg, scriptDoneMsg,
			completionTablesMsg, completionColumnsMsg, uicomplete.AcceptedMsg, connection.SubmittedMsg,
			editInfoMsg, mutationDoneMsg, celledit.SubmittedMsg, confirm.ConfirmedMsg,
			sidebar.SchemasLoadedMsg, sidebar.TablesLoadedMsg, sidebar.ColumnsLoadedMsg,
			sidebar.ConnectMsg, sidebar.TableOpenMsg,
			sidebar.TableActivateMsg, sidebar.EditorOpenMsg, sidebar.ConnectionInfoMsg, sidebar.NodeMenuMsg, overviewLoadedMsg, versionLoadedMsg,
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
	m.editor.SetValue("SELECT 1;")
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
	if m.editor.Value() != "SELECT 1;" {
		t.Fatalf("unsaved editor content should survive when no session is active, got %q", m.editor.Value())
	}
	conns, err := config.LoadRuntimeConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 || conns[0].Name != "smoke" || conns[0].Path != path {
		t.Fatalf("saved connections = %+v", conns)
	}
}

func TestAddConnectionSavesCurrentSession(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.shutdown()

	m.editor.SetValue("SELECT 42;")
	m.focus = focusEditor
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = runCmd(t, model.(*Model), cmd)
	if m.results.Result() == nil {
		t.Fatal("editor query should produce results")
	}

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
	if m.editor.Value() != "" || m.context != contextInfo {
		t.Fatalf("new session should start clean: editor=%q context=%v", m.editor.Value(), m.context)
	}
	if view := ansi.Strip(m.sidebar.View()); !strings.Contains(view, "2 active") {
		t.Fatalf("sidebar should show two active sessions:\n%s", view)
	}

	saved, ok := m.sessions["local-sqlite"]
	if !ok {
		t.Fatal("previous session was not saved")
	}
	if saved.tabs[saved.activeTab].editor.Value() != "SELECT 42;" || saved.results.Result() == nil {
		t.Fatalf("saved session state = editor %q, results %#v", saved.tabs[saved.activeTab].editor.Value(), saved.results.Result())
	}

	m = runCmd(t, m, m.switchToConnection("local-sqlite"))
	if m.connName != "local-sqlite" || m.conn == nil {
		t.Fatalf("connection = %q", m.connName)
	}
	if m.editor.Value() != "SELECT 42;" || m.results.Result() == nil {
		t.Fatalf("restored session state = editor %q, results %#v", m.editor.Value(), m.results.Result())
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
	m = sendKey(t, m, tea.KeyPressMsg{Code: 'm'})
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
	m.editor.SetValue("SELECT 42;")

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	for i := 0; i < 3; i++ {
		m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: 'm'})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	rs := m.results.Result()
	if rs == nil || len(rs.Rows) != 5 {
		t.Fatalf("result = %#v, status=%s", rs, m.status)
	}
	if m.focus != focusResults {
		t.Fatalf("focus = %v, want results", m.focus)
	}
	if got := m.editor.Value(); got != "SELECT 42;" {
		t.Fatalf("table data must not modify the editor, got %q", got)
	}
}

func TestSidebarDoubleClickOpensTableData(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	model, cmd := m.Update(sidebar.TableOpenMsg{Schema: "main", Table: "users", Kind: "table"})
	m = runCmd(t, model.(*Model), cmd)
	rs := m.results.Result()
	if rs == nil || len(rs.Rows) != 5 {
		t.Fatalf("result = %#v, status=%s", rs, m.status)
	}
	if m.focus != focusResults || m.pager == nil {
		t.Fatalf("focus=%v pager=%#v", m.focus, m.pager)
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
	m = sendKey(t, m, tea.KeyPressMsg{Code: 'm'})
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

func TestSidebarHeightExcludesPaneTitle(t *testing.T) {
	m := testModel(t, "en")
	m.layout()
	lines := strings.Split(m.sidebar.View(), "\n")
	want := m.height - 3 // status bar + menu bar + pane title
	if len(lines) != want {
		t.Fatalf("sidebar lines = %d, want %d", len(lines), want)
	}
}

func TestPostgresDatabaseDiscovery(t *testing.T) {
	m := testModel(t, "en")
	pg := config.Connection{
		Name:     "pg",
		Type:     config.TypePostgres,
		Host:     "h",
		User:     "u",
		Password: "p",
		Database: "bdb",
		ReadOnly: true,
	}
	m.cfg.Connections = append(m.cfg.Connections, pg)
	m.sidebar.AddConnection(pg)
	if cmd := m.handleDatabasesLoaded(databasesLoadedMsg{
		id:        m.connectSeq,
		base:      "pg",
		databases: []string{"bdb", "other", "postgres"},
	}); cmd != nil {
		t.Fatal("database discovery should not return a command")
	}

	if m.cfg.Find("pg/bdb") != nil {
		t.Fatal("current database should keep the base connection name")
	}
	for _, name := range []string{"other", "postgres"} {
		logical := "pg/" + name
		connection := m.cfg.Find(logical)
		if connection == nil || connection.Database != name || !connection.ReadOnly {
			t.Fatalf("connection %q = %#v", logical, connection)
		}
	}
	view := ansi.Strip(m.sidebar.View())
	for _, want := range []string{"pg", "bdb", "other", "postgres"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sidebar missing %q:\n%s", want, view)
		}
	}
}

func TestMySQLDatabaseDiscovery(t *testing.T) {
	m := testModel(t, "en")
	mysql := config.Connection{
		Name:     "my",
		Type:     config.TypeMySQL,
		Host:     "h",
		User:     "u",
		Password: "p",
		ReadOnly: true,
	}
	m.cfg.Connections = append(m.cfg.Connections, mysql)
	m.sidebar.AddConnection(mysql)
	if cmd := m.handleDatabasesLoaded(databasesLoadedMsg{
		id:        m.connectSeq,
		base:      "my",
		databases: []string{"app", "report"},
	}); cmd != nil {
		t.Fatal("database discovery should not return a command")
	}
	for _, name := range []string{"app", "report"} {
		logical := "my/" + name
		connection := m.cfg.Find(logical)
		if connection == nil || connection.Database != name || !connection.ReadOnly {
			t.Fatalf("connection %q = %#v", logical, connection)
		}
	}
	view := ansi.Strip(m.sidebar.View())
	for _, want := range []string{"my", "app", "report"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sidebar missing %q:\n%s", want, view)
		}
	}
}

func TestCloseSessionShortcut(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	m = sendKey(t, m, tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl | tea.ModShift})
	if m.conn != nil || m.connName != "" {
		t.Fatalf("session should be closed: conn=%v name=%q", m.conn, m.connName)
	}
	if !strings.Contains(m.status, "Disconnected") {
		t.Fatalf("status = %q", m.status)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl | tea.ModShift})
	if !strings.Contains(m.status, "No connection") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestDisconnectKeepsOtherSessions(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.shutdown()

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

	m.disconnect()
	if m.conn != nil || m.connName != "" {
		t.Fatalf("current session should be closed: conn=%v name=%q", m.conn, m.connName)
	}
	if _, ok := m.sessions["local-sqlite"]; !ok {
		t.Fatal("other sessions should remain open")
	}
	if _, ok := m.sessions["second"]; ok {
		t.Fatal("current session should be removed")
	}

	m = runCmd(t, m, m.switchToConnection("local-sqlite"))
	if m.connName != "local-sqlite" || m.conn == nil {
		t.Fatalf("connection = %q", m.connName)
	}
}

func TestSidebarShortcutHint(t *testing.T) {
	m := testModel(t, "en")
	m.layout()
	if hint := m.sidebarHint(m.sideW); !strings.Contains(hint, "⏎") {
		t.Fatalf("compact hint = %q", hint)
	}
	if content := m.View().Content; !strings.Contains(content, "⏎") {
		t.Fatalf("sidebar hint missing from view:\n%s", content)
	}
	m.width = 200
	m.layout()
	if hint := m.sidebarHint(m.sideW); hint != m.t.T(i18n.KeyHintSidebar) {
		t.Fatalf("wide hint = %q", hint)
	}
}

func TestStatusbarSections(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m.focus = focusResults
	left := m.statusLeft()
	for _, want := range []string{"Results", "Query 1", "│", "local-sqlite"} {
		if !strings.Contains(left, want) {
			t.Fatalf("status left = %q, missing %q", left, want)
		}
	}
	sidebarView := ansi.Strip(m.sidebar.View())
	for _, want := range []string{"● local-sqlite", "1 active"} {
		if !strings.Contains(sidebarView, want) {
			t.Fatalf("sidebar header = %q, missing %q", sidebarView, want)
		}
	}
	m.setStatus(m.t.T(i18n.KeyStatusReady), false)
	if right := m.statusRight(); !strings.Contains(right, "✓ Ready") {
		t.Fatalf("ready right = %q", right)
	}
	m.setStatus("boom", true)
	if right := m.statusRight(); !strings.Contains(right, "✗ boom") {
		t.Fatalf("error right = %q", right)
	}
	m.setStatus("", false)
	if right := m.statusRight(); right != "" {
		t.Fatalf("empty right = %q", right)
	}
}

func TestMouseClickSidebar(t *testing.T) {
	m := testModel(t, "en")
	model, cmd := m.Update(tea.MouseClickMsg{X: 3, Y: 2, Button: tea.MouseLeft})
	m = runCmd(t, model.(*Model), cmd)
	if m.conn == nil {
		t.Fatalf("click should connect, status=%s", m.status)
	}
	defer m.conn.Close()
	if m.focus != focusSidebar {
		t.Fatalf("focus = %v, want sidebar", m.focus)
	}
}

func TestMouseDoubleClickOpensTable(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	model, cmd := m.Update(tea.MouseClickMsg{X: 3, Y: 3, Button: tea.MouseLeft})
	m = runCmd(t, model.(*Model), cmd)
	if !strings.Contains(m.sidebar.View(), "users") {
		t.Fatalf("schema should expand and list tables:\n%s", m.sidebar.View())
	}

	model, cmd = m.Update(tea.MouseClickMsg{X: 3, Y: 4, Button: tea.MouseLeft})
	m = runCmd(t, model.(*Model), cmd)
	if m.focus != focusSidebar || m.context != contextInfo {
		t.Fatal("single click should only select the table")
	}

	model, cmd = m.Update(tea.MouseClickMsg{X: 3, Y: 4, Button: tea.MouseLeft})
	m = runCmd(t, model.(*Model), cmd)
	if m.results.Result() == nil {
		t.Fatalf("double click should open table data, status=%s", m.status)
	}
	if m.focus != focusResults {
		t.Fatalf("focus = %v, want results", m.focus)
	}
	if m.context != contextData {
		t.Fatalf("context = %v, want data", m.context)
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

	x := m.sideW + 2
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

func TestManualZoomLayout(t *testing.T) {
	m := testModel(t, "en")

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyF2})
	content := m.View().Content
	if strings.Contains(content, "SQL") || strings.Contains(content, "Results") {
		t.Fatalf("sidebar zoom should hide other panes:\n%s", content)
	}
	if got := strings.Count(content, "\n") + 1; got != 30 {
		t.Fatalf("lines = %d, want 30", got)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	content = m.View().Content
	if strings.Contains(content, "Connections") || strings.Contains(content, "Results") {
		t.Fatalf("editor zoom should hide other panes:\n%s", content)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	content = m.View().Content
	if strings.Contains(content, "Connections") || strings.Contains(content, "SQL") {
		t.Fatalf("results zoom should hide other panes:\n%s", content)
	}
	if !strings.Contains(content, "Results") {
		t.Fatalf("results title missing:\n%s", content)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyF2})
	content = m.View().Content
	for _, want := range []string{"Connections", "SQL", "Results"} {
		if !strings.Contains(content, want) {
			t.Fatalf("split layout missing %q:\n%s", want, content)
		}
	}
}

func TestAutoFocusLayout(t *testing.T) {
	m := testModelWithFocus(t, "en", config.FocusModeAuto)

	content := m.View().Content
	for _, want := range []string{"Connections", "SQL", "Results"} {
		if !strings.Contains(content, want) {
			t.Fatalf("initial split missing %q:\n%s", want, content)
		}
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	content = m.View().Content
	if strings.Contains(content, "Connections") || strings.Contains(content, "Results") {
		t.Fatalf("editor should auto zoom:\n%s", content)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	content = m.View().Content
	if strings.Contains(content, "Connections") || strings.Contains(content, "SQL") {
		t.Fatalf("results should auto zoom:\n%s", content)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	content = m.View().Content
	if !strings.Contains(content, "Connections") || !strings.Contains(content, "SQL") {
		t.Fatalf("sidebar focus should restore split layout:\n%s", content)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyF2})
	content = m.View().Content
	if !strings.Contains(content, "Connections") || !strings.Contains(content, "Results") {
		t.Fatalf("F2 should pause auto zoom:\n%s", content)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyF2})
	content = m.View().Content
	if strings.Contains(content, "Connections") {
		t.Fatalf("F2 should resume auto zoom:\n%s", content)
	}
}

func TestMenuFocusLayout(t *testing.T) {
	m := testModel(t, "en")
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	for i := 0; i < 11; i++ {
		m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.zoom {
		t.Fatal("menu action should enable focused layout")
	}
	if m.options.IsOpen() {
		t.Fatal("menu should close after the action")
	}
}

func TestMouseClickZoomResults(t *testing.T) {
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

	m.focus = focusResults
	m.zoom = true
	m.layout()

	model, _ = m.Update(tea.MouseClickMsg{X: 1, Y: 4, Button: tea.MouseLeft})
	m = model.(*Model)
	row, col := m.results.CursorPosition()
	if row != 0 || col != 0 {
		t.Fatalf("cursor = %d,%d, want 0,0", row, col)
	}
}

func TestViewFullCell(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m.editor.SetValue("SELECT 'hello world' AS note;")
	m.focus = focusEditor
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = runCmd(t, model.(*Model), cmd)

	m.focus = focusEditor
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.textView.IsOpen() {
		t.Fatalf("cell viewer should open, focus=%v status=%s", m.focus, m.status)
	}
	content := m.View().Content
	if !strings.Contains(content, "Cell: note") || !strings.Contains(content, "hello world") {
		t.Fatalf("cell view:\n%s", content)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.textView.IsOpen() {
		t.Fatal("esc should close the cell viewer")
	}
}

func TestCellEditFlow(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m = runCmd(t, m, m.tableData("main", "users"))
	if _, _, ok := m.results.Source(); !ok {
		t.Fatalf("table data should be editable, status=%s", m.status)
	}
	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	m = sendKey(t, m, tea.KeyPressMsg{Text: "e"})
	if !m.cellEdit.IsOpen() {
		t.Fatalf("cell editor should open, status=%s", m.status)
	}
	m.cellEdit.SetValue("AliceEdited")
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.confirm.IsOpen() {
		t.Fatal("confirm dialog should open after editing")
	}
	m = sendKey(t, m, tea.KeyPressMsg{Text: "y"})

	rs, err := m.conn.Querier().Query(context.Background(), "SELECT name FROM users WHERE id = 1;", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rows) != 1 || rs.Rows[0][0] != "AliceEdited" {
		t.Fatalf("query result = %#v", rs.Rows)
	}
	if m.results.Result() == nil {
		t.Fatal("results should refresh after update")
	}
}

func TestDeleteRowFlow(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m = runCmd(t, m, m.tableData("main", "users"))
	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Text: "x"})
	if !m.confirm.IsOpen() {
		t.Fatalf("delete confirm should open, status=%s", m.status)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Text: "y"})

	rs, err := m.conn.Querier().Query(context.Background(), "SELECT COUNT(*) FROM users;", 10)
	if err != nil {
		t.Fatal(err)
	}
	if rs.Rows[0][0] != int64(4) {
		t.Fatalf("remaining rows = %#v, want 4", rs.Rows[0][0])
	}
}

func TestCellEditRejectsReadOnlyOrCustomQuery(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Text: "e"})
	if m.cellEdit.IsOpen() {
		t.Fatal("editing should not open for a custom query")
	}
	if !strings.Contains(m.status, "table data") {
		t.Fatalf("status = %q", m.status)
	}

	m = runCmd(t, m, m.tableData("main", "users"))
	m.cfg.Connections[0].ReadOnly = true
	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Text: "e"})
	if m.cellEdit.IsOpen() {
		t.Fatal("editing should not open for a read-only connection")
	}
	if !strings.Contains(m.status, "read-only") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInsertTemplate(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	m = runCmd(t, m, m.editInfoCmd("main", "users", "insert"))
	if !strings.Contains(m.editor.Value(), "INSERT INTO users") {
		t.Fatalf("editor = %q", m.editor.Value())
	}
	if !strings.Contains(m.editor.Value(), "VALUES") {
		t.Fatalf("editor = %q", m.editor.Value())
	}
}

func TestSessionSwitching(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.shutdown()

	m = runCmd(t, m, m.tableData("main", "users"))
	m.editor.SetValue("SELECT 1;")

	data, err := os.ReadFile(m.cfg.Connections[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(t.TempDir(), "second.db")
	if err := os.WriteFile(secondPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	second := config.Connection{Name: "second", Type: config.TypeSQLite, Path: secondPath}
	m.cfg.Connections = append(m.cfg.Connections, second)
	m.sidebar.AddConnection(second)

	m = runCmd(t, m, m.switchToConnection("second"))
	if m.connName != "second" || m.conn == nil {
		t.Fatalf("connection = %q", m.connName)
	}
	if m.editor.Value() != "" {
		t.Fatalf("new session editor = %q", m.editor.Value())
	}

	m.editor.SetValue("SELECT 2;")
	m = runCmd(t, m, m.switchToConnection("local-sqlite"))
	if m.connName != "local-sqlite" || m.conn == nil {
		t.Fatalf("connection = %q", m.connName)
	}
	if m.editor.Value() != "SELECT 1;" {
		t.Fatalf("restored editor = %q", m.editor.Value())
	}
	if m.results.Result() == nil {
		t.Fatal("results should be restored with the session")
	}
	if m.pager == nil || m.pager.schema != "main" || m.pager.table != "users" {
		t.Fatalf("pager should be restored with the session: %#v", m.pager)
	}
}

func TestSessionSwitchCancelsMutation(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		m.sessions = map[string]*session{}
		m.shutdown()
	}()

	ran := false
	m.pendingCmd = m.mutationCmd("update", func(ctx context.Context, _ db.Mutator) (int64, error) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		ran = true
		return 1, nil
	})
	m.saveCurrentSession()
	msg, ok := m.pendingCmd().(mutationDoneMsg)
	if !ok {
		t.Fatalf("mutation message = %T", msg)
	}
	if !errors.Is(msg.err, context.Canceled) {
		t.Fatalf("mutation error = %v, want context.Canceled", msg.err)
	}
	if ran {
		t.Fatal("mutation ran after the session was saved")
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

func TestEditorQueryIgnoresMaxRows(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.cfg.Query.MaxRows = 2

	m.editor.SetValue("SELECT * FROM users ORDER BY id;")
	m.focus = focusEditor
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = runCmd(t, model.(*Model), cmd)

	rs := m.results.Result()
	if rs == nil || len(rs.Rows) != 5 || rs.Truncated {
		t.Fatalf("editor query result = %#v, status=%s", rs, m.status)
	}
	if !strings.Contains(m.status, "5 row(s)") || strings.Contains(m.status, "truncated") {
		t.Fatalf("status = %q", m.status)
	}
	if m.pager != nil {
		t.Fatalf("editor query must not create a table pager: %#v", m.pager)
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
	if m.focus != focusResults {
		t.Fatalf("focus = %v, want results after preview", m.focus)
	}
	if m.results.Result() == nil {
		t.Fatalf("preview should load table data, status=%s", m.status)
	}
	if got := m.editor.Value(); strings.Contains(got, "users") {
		t.Fatalf("preview should not touch the editor, got %q", got)
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

func TestCellEditValuePreservesNullAndMultiline(t *testing.T) {
	value, err := cellEditValue("", editTarget{dataType: "integer", wasNull: true})
	if err != nil || value != nil {
		t.Fatalf("empty NULL value = %#v, err = %v", value, err)
	}
	value, err = cellEditValue("NULL", editTarget{dataType: "text"})
	if err != nil || value != nil {
		t.Fatalf("NULL sentinel = %#v, err = %v", value, err)
	}
	value, err = cellEditValue("line1\nline2", editTarget{dataType: "text"})
	if err != nil || value != "line1\nline2" {
		t.Fatalf("multiline value = %#v, err = %v", value, err)
	}
	value, err = cellEditValue("", editTarget{dataType: "text"})
	if err != nil || value != "" {
		t.Fatalf("empty text = %#v, err = %v", value, err)
	}
	if _, err := cellEditValue("abc", editTarget{dataType: "integer"}); err == nil {
		t.Fatal("expected invalid integer error")
	}
	value, err = cellEditValue("1 day", editTarget{dataType: "interval"})
	if err != nil || value != "1 day" {
		t.Fatalf("interval value = %#v, err = %v", value, err)
	}
}

func TestReadOnlyBlocksSQLWrites(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.cfg.Connections[0].ReadOnly = true
	for _, stmt := range []string{
		"UPDATE users SET name = 'blocked' WHERE id = 1;",
		"PRAGMA journal_mode = WAL;",
		"EXECUTE 'DELETE FROM users';",
	} {
		m.editor.SetValue(stmt)
		if cmd := m.runQuery(); cmd != nil {
			t.Fatalf("read-only connection should reject %q", stmt)
		}
		if !strings.Contains(m.status, "read-only") {
			t.Fatalf("status = %q", m.status)
		}
	}
	m.editor.SetValue("PRAGMA table_info(users);")
	if cmd := m.runQuery(); cmd == nil {
		t.Fatal("read-only PRAGMA introspection should be allowed")
	}
	m.cancelQuery()
	rs, err := m.conn.Querier().Query(context.Background(), "SELECT name FROM users WHERE id = 1;", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rows) != 1 || rs.Rows[0][0] == "blocked" {
		t.Fatalf("row changed on read-only connection: %#v", rs.Rows)
	}
}

func TestPendingSourceIsolation(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	tableCmd := m.tableData("main", "users")
	m.editor.SetValue("SELECT 1;")
	customCmd := m.runQuery()
	m = runCmd(t, m, customCmd)
	if _, _, ok := m.results.Source(); ok {
		t.Fatal("custom query must not inherit the table data source")
	}

	m = runCmd(t, m, tableCmd)
	if _, _, ok := m.results.Source(); ok {
		t.Fatal("stale table query must not attach its source to a newer result")
	}
}

func TestStaleAsyncMessagesIgnored(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	stale := m.connectSeq - 1

	model, _ := m.Update(ddlLoadedMsg{id: stale, title: "ddl", text: "SELECT 1;"})
	m = model.(*Model)
	if m.textView.IsOpen() {
		t.Fatal("stale DDL message must not open the viewer")
	}
	model, _ = m.Update(indexesLoadedMsg{id: stale, title: "indexes", text: "idx"})
	m = model.(*Model)
	if m.textView.IsOpen() {
		t.Fatal("stale index message must not open the viewer")
	}

	m = runCmd(t, m, m.tableData("main", "users"))
	m.focus = focusResults
	info := m.editInfoCmd("main", "users", "edit")()
	edit, ok := info.(editInfoMsg)
	if !ok {
		t.Fatalf("edit info message = %T", info)
	}
	edit.id = stale
	model, _ = m.Update(edit)
	m = model.(*Model)
	if m.cellEdit.IsOpen() {
		t.Fatal("stale edit info must not open the cell editor")
	}

	edit.id = m.connectSeq
	model, _ = m.Update(edit)
	m = model.(*Model)
	if !m.cellEdit.IsOpen() {
		t.Fatalf("current edit info should open the cell editor, status=%s", m.status)
	}
	m.cellEdit.Close()

	m.setStatus("keep", false)
	model, _ = m.Update(mutationDoneMsg{id: stale, kind: "update", affected: 1})
	m = model.(*Model)
	if m.status != "keep" {
		t.Fatalf("stale mutation changed status to %q", m.status)
	}
	model, _ = m.Update(scriptDoneMsg{id: stale, total: 3, executed: 2})
	m = model.(*Model)
	if m.status != "keep" {
		t.Fatalf("stale script changed status to %q", m.status)
	}
}

func TestAddConnectionExpandsEnv(t *testing.T) {
	m := testModel(t, "en")
	path := filepath.Join(t.TempDir(), "env.db")
	t.Setenv("BDB_TEST_DB_PATH", path)
	cmd := m.addConnection(config.Connection{
		Name: "env-sqlite",
		Type: config.TypeSQLite,
		Path: "${BDB_TEST_DB_PATH}",
	})
	if cmd == nil {
		t.Fatalf("addConnection returned no command: %s", m.status)
	}
	m = runCmd(t, m, cmd)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	if m.connName != "env-sqlite" {
		t.Fatalf("connected to %q, want env-sqlite", m.connName)
	}
	conn := m.cfg.Find("env-sqlite")
	if conn == nil || conn.Path != path {
		t.Fatalf("runtime config path = %#v, want %q", conn, path)
	}
	data, err := os.ReadFile(config.RuntimeConnectionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "${BDB_TEST_DB_PATH}") {
		t.Fatalf("runtime file should keep the env reference:\n%s", data)
	}
}

func TestModalOverlaysBlockMouse(t *testing.T) {
	m := testModel(t, "en")
	if m.anyOverlayOpen() {
		t.Fatal("no overlay should be open initially")
	}
	m.cellEdit.Open("Edit", "value")
	if !m.anyOverlayOpen() {
		t.Fatal("cell editor should block mouse interaction")
	}
	m.cellEdit.Close()
	m.confirm.Open("Confirm", "statement")
	if !m.anyOverlayOpen() {
		t.Fatal("confirm dialog should block mouse interaction")
	}
}

func TestStaleConnectionResponseIgnored(t *testing.T) {
	m := testModel(t, "en")
	m.connectSeq = 2
	m.setStatus("current", false)
	m = runCmd(t, m, func() tea.Msg {
		return connOpenedMsg{id: 1, name: "stale", err: errors.New("stale")}
	})
	if m.status != "current" {
		t.Fatalf("stale response changed status to %q", m.status)
	}
}

func TestInitDoesNotAutoConnect(t *testing.T) {
	m := testModel(t, "en")
	if cmd := m.Init(); cmd != nil {
		t.Fatal("Init should not connect automatically")
	}
}

func TestShutdownClosesAllSessions(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	current := m.conn

	background, err := db.Open(context.Background(), db.Config{
		Name:   "background",
		Type:   "sqlite",
		Params: map[string]string{"path": ":memory:"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := background.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	m.sessions["background"] = &session{conn: background}

	m.Shutdown()
	m.Shutdown() // must be safe to call twice

	for name, conn := range map[string]db.Conn{"current": current, "background": background} {
		if err := conn.Ping(context.Background()); err == nil {
			t.Fatalf("%s connection is still open after Shutdown", name)
		}
	}
	if len(m.sessions) != 0 {
		t.Fatalf("sessions = %d after Shutdown, want 0", len(m.sessions))
	}
}

func TestConnectionInfoPanel(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	if m.context != contextInfo {
		t.Fatalf("context = %v, want info", m.context)
	}
	rs := m.results.Result()
	if rs == nil {
		t.Fatal("connection info missing")
	}
	values := map[string]string{}
	for _, row := range rs.Rows {
		if len(row) < 2 {
			continue
		}
		key, _ := row[0].(string)
		value, _ := row[1].(string)
		values[key] = value
	}
	if values["Name"] != "local-sqlite" {
		t.Fatalf("name = %q", values["Name"])
	}
	if values["Status"] != m.t.T(i18n.KeyStatusConnected) {
		t.Fatalf("status = %q", values["Status"])
	}
}

func TestDatabaseOverview(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.connName = "local-sqlite/main"
	m = runCmd(t, m, m.loadOverviewCmd())
	if m.context != contextTables {
		t.Fatalf("context = %v, want tables", m.context)
	}
	rs := m.results.Result()
	if rs == nil {
		t.Fatal("overview missing")
	}
	found := false
	for _, row := range rs.Rows {
		if len(row) >= 2 {
			if name, _ := row[1].(string); name == "users" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("overview missing users: %#v", rs.Rows)
	}
}

func TestEnterTableOpensTabAndData(t *testing.T) {
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
	tabs := len(m.tabs)
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.tabs) != tabs+1 {
		t.Fatalf("tabs = %d, want %d", len(m.tabs), tabs+1)
	}
	if got := m.editor.Value(); !strings.Contains(got, "users") {
		t.Fatalf("editor = %q", got)
	}
	if m.context != contextData || m.focus != focusResults {
		t.Fatalf("context = %v, focus = %v", m.context, m.focus)
	}
	if m.results.Result() == nil {
		t.Fatalf("results missing, status=%s", m.status)
	}

	m.focus = focusSidebar
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.tabs) != tabs+1 {
		t.Fatalf("opening the same table should reuse its tab, tabs = %d", len(m.tabs))
	}
	if got := m.editor.Value(); !strings.Contains(got, "users") {
		t.Fatalf("editor after reuse = %q", got)
	}
}

func TestConnectionInfoShowsVersion(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	if m.infoVersion == "" || m.infoVersion == "-" {
		t.Fatalf("version = %q", m.infoVersion)
	}
	if m.results.Result() == nil {
		t.Fatal("connection info missing")
	}
}

func TestOverviewColumnsAndStats(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.connName = "local-sqlite/main"
	m = runCmd(t, m, m.loadOverviewCmd())
	rs := m.results.Result()
	if rs == nil {
		t.Fatal("overview missing")
	}
	if len(rs.Columns) != 7 {
		t.Fatalf("columns = %d, want 7", len(rs.Columns))
	}
	for _, row := range rs.Rows {
		if len(row) != 7 {
			t.Fatalf("row = %#v, want 7 values", row)
		}
		if row[1] == "users" {
			for _, index := range []int{3, 4, 5, 6} {
				if row[index] != "-" {
					t.Fatalf("sqlite stat column %d = %#v, want -", index, row[index])
				}
			}
		}
	}
}

func TestOverviewFilter(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.connName = "local-sqlite/main"
	m = runCmd(t, m, m.loadOverviewCmd())
	if m.context != contextTables {
		t.Fatalf("context = %v", m.context)
	}
	all := len(m.overviewAll)
	if all == 0 {
		t.Fatal("no tables in overview")
	}

	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Code: '/'})
	if !m.filtering {
		t.Fatal("filter input should open")
	}
	for _, char := range "users" {
		m = sendKey(t, m, tea.KeyPressMsg{Text: string(char)})
	}
	rs := m.results.Result()
	if rs == nil || len(rs.Rows) == 0 || len(rs.Rows) >= all {
		t.Fatalf("filtered rows = %d, all = %d", len(rs.Rows), all)
	}
	for _, row := range rs.Rows {
		if name, _ := row[1].(string); !strings.Contains(name, "users") {
			t.Fatalf("filtered row = %#v", row)
		}
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.filtering || m.overviewFilter != "" {
		t.Fatalf("filter still active: %v %q", m.filtering, m.overviewFilter)
	}
	if got := len(m.results.Result().Rows); got != all {
		t.Fatalf("rows after clearing filter = %d, want %d", got, all)
	}
}

func TestSessionRestoresOverviewState(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.connName = "local-sqlite/main"
	m = runCmd(t, m, m.loadOverviewCmd())
	m.overviewFilter = "users"
	m.rebuildOverview()
	filtered := len(m.results.Result().Rows)

	m.saveCurrentSession()
	saved, ok := m.sessions["local-sqlite/main"]
	if !ok {
		t.Fatal("session was not saved")
	}
	m.resetSession()
	m.activateSession("local-sqlite/main", saved)

	if m.context != contextTables {
		t.Fatalf("context = %v", m.context)
	}
	if m.overviewFilter != "users" {
		t.Fatalf("filter = %q", m.overviewFilter)
	}
	if len(m.overviewAll) == 0 {
		t.Fatal("overview rows were not restored")
	}
	if got := len(m.results.Result().Rows); got != filtered {
		t.Fatalf("restored rows = %d, want %d", got, filtered)
	}
}

func TestBreadcrumbAfterTableOpen(t *testing.T) {
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

	left := m.statusLeft()
	if !strings.Contains(left, "local-sqlite / users") {
		t.Fatalf("status left = %q", left)
	}
}

func TestOverviewSchemaColumn(t *testing.T) {
	cases := map[string]bool{
		config.TypePostgres: true,
		config.TypeSQLite:   true,
		config.TypeMySQL:    false,
		config.TypeMongo:    false,
		config.TypeRedis:    false,
	}
	for typ, want := range cases {
		if got := typeHasSchema(typ); got != want {
			t.Fatalf("typeHasSchema(%q) = %v, want %v", typ, got, want)
		}
	}
}

func TestOverviewEnterOpensTableData(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.connName = "local-sqlite/main"
	m = runCmd(t, m, m.loadOverviewCmd())
	m.focus = focusResults

	row, ok := m.results.CursorRow()
	if !ok || len(row) < 2 {
		t.Fatalf("overview cursor row = %#v", row)
	}
	want, _ := row[1].(string)
	if want == "" {
		t.Fatalf("overview cursor row = %#v", row)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.context != contextData {
		t.Fatalf("context = %v, want data", m.context)
	}
	if m.pager == nil || m.pager.table != want {
		t.Fatalf("pager table = %#v, want %q", m.pager, want)
	}
	if m.focus != focusResults {
		t.Fatalf("focus = %v", m.focus)
	}
}

func TestLayoutMenuChoice(t *testing.T) {
	m := testModel(t, "en")
	m.layout()
	stackedEditorH := m.editorH
	stackedResultsH := m.resultsH

	m.applyMenuChange(menu.ChangedMsg{ID: "layout", Value: 1})
	if m.layoutMode != config.LayoutColumns || !m.columnsActive {
		t.Fatalf("columns layout not active: mode=%q active=%v", m.layoutMode, m.columnsActive)
	}
	if m.editorW <= 0 || m.resultsW <= 0 {
		t.Fatalf("editorW=%d resultsW=%d", m.editorW, m.resultsW)
	}

	m.applyMenuChange(menu.ChangedMsg{ID: "layout", Value: 2})
	if m.layoutMode != config.LayoutResultsTop || m.columnsActive {
		t.Fatalf("results-top layout not active: mode=%q active=%v", m.layoutMode, m.columnsActive)
	}
	if m.editorH != stackedResultsH || m.resultsH != stackedEditorH {
		t.Fatalf("heights not swapped: editor=%d results=%d", m.editorH, m.resultsH)
	}

	m.applyMenuChange(menu.ChangedMsg{ID: "layout", Value: 0})
	if m.layoutMode != config.LayoutStacked {
		t.Fatalf("mode = %q", m.layoutMode)
	}
	if m.editorH != stackedEditorH || m.resultsH != stackedResultsH {
		t.Fatalf("stacked heights = %d/%d, want %d/%d", m.editorH, m.resultsH, stackedEditorH, stackedResultsH)
	}
}

func TestColumnsLayoutViewWidth(t *testing.T) {
	m := testModel(t, "en")
	m.layoutMode = config.LayoutColumns
	m.layout()
	if !m.columnsActive {
		t.Fatal("columns layout not active")
	}
	for _, line := range strings.Split(m.View().Content, "\n") {
		if got := ansi.StringWidth(line); got != m.width {
			t.Fatalf("line width = %d, want %d: %q", got, m.width, line)
		}
	}
}

func TestConnectionNodeMenu(t *testing.T) {
	m := testModel(t, "en")
	m = sendKey(t, m, tea.KeyPressMsg{Code: 'm'})
	if !m.tableMenu.IsOpen() {
		t.Fatalf("menu should open for a connection node, status=%s", m.status)
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "Connect") {
		t.Fatalf("connection menu missing Connect:\n%s", ansi.Strip(m.View().Content))
	}
}

func TestNodeMenuItemsPerKind(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	connection := m.nodeMenuItems(sidebar.NodeMenuMsg{Kind: sidebar.KindConnection, ConnName: "local-sqlite"})
	if len(connection) != 3 {
		t.Fatalf("connection items = %d, want 3", len(connection))
	}
	database := m.nodeMenuItems(sidebar.NodeMenuMsg{Kind: sidebar.KindDatabase, Label: "main", SessionName: "local-sqlite/main"})
	if len(database) != 9 {
		t.Fatalf("database items = %d, want 9", len(database))
	}
	if database[0].ID != "db.open" || database[2].ID != "db.close" || database[4].ID != "db.export.sql" {
		t.Fatalf("database items = %#v", database)
	}
	table := m.nodeMenuItems(sidebar.NodeMenuMsg{Kind: sidebar.KindTable, Schema: "main", Table: "users", Sub: "table"})
	if len(table) != 12 || table[0].ID != "table.data" || table[7].ID != "table.export.sql" {
		t.Fatalf("table items = %#v", table)
	}
}

func TestCloseSession(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	background, err := db.Open(context.Background(), db.Config{
		Name:   "other",
		Type:   "sqlite",
		Params: map[string]string{"path": ":memory:"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := background.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	m.sessions["other"] = &session{conn: background}

	m.closeSession("other")
	if _, ok := m.sessions["other"]; ok {
		t.Fatal("session was not removed")
	}
	if err := background.Ping(context.Background()); err == nil {
		t.Fatal("connection is still open after closeSession")
	}
}

func TestExportTableSQL(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.outDir = t.TempDir()
	m = runCmd(t, m, m.exportTableCmd("main", "users", "sql"))

	entries, err := os.ReadDir(m.outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("files = %d, want 1", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(m.outDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "CREATE TABLE") || !strings.Contains(string(data), "INSERT INTO") {
		t.Fatalf("SQL dump:\n%s", data)
	}
}

func TestExportDatabaseSQL(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	base := m.cfg.Find("local-sqlite")
	if base == nil {
		t.Fatal("missing base connection")
	}
	derived := *base
	derived.Name = "local-sqlite/main"
	derived.Database = "main"
	m.cfg.Connections = append(m.cfg.Connections, derived)
	m.connName = "local-sqlite/main"
	m.outDir = t.TempDir()

	m = runCmd(t, m, m.exportDatabaseCmd("local-sqlite/main", "sql"))
	entries, err := os.ReadDir(m.outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("files = %d, want 1", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(m.outDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "users") || !strings.Contains(string(data), "INSERT INTO") {
		t.Fatalf("database SQL dump:\n%s", data)
	}
}

func TestExportProgress(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.outDir = t.TempDir()

	var messages []tea.Msg
	m.SetSender(func(msg tea.Msg) { messages = append(messages, msg) })
	m = runCmd(t, m, m.exportTableCmd("main", "users", "sql"))
	if len(messages) != 1 {
		t.Fatalf("progress messages = %d, want 1", len(messages))
	}
	progress, ok := messages[0].(exportProgressMsg)
	if !ok || progress.label != "users" || progress.total != 0 {
		t.Fatalf("progress = %#v", messages[0])
	}

	model, _ := m.Update(progress)
	m = model.(*Model)
	if !m.exporting {
		t.Fatal("exporting flag not set")
	}
	if !strings.Contains(m.status, "users") {
		t.Fatalf("status = %q", m.status)
	}
	model, _ = m.Update(exportedMsg{path: "/tmp/out.sql"})
	m = model.(*Model)
	if m.exporting {
		t.Fatal("exporting flag not cleared")
	}
}

func TestExportDatabaseProgress(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	base := m.cfg.Find("local-sqlite")
	if base == nil {
		t.Fatal("missing base connection")
	}
	derived := *base
	derived.Name = "local-sqlite/main"
	derived.Database = "main"
	m.cfg.Connections = append(m.cfg.Connections, derived)
	m.connName = "local-sqlite/main"
	m.outDir = t.TempDir()

	var messages []tea.Msg
	m.SetSender(func(msg tea.Msg) { messages = append(messages, msg) })
	m = runCmd(t, m, m.exportDatabaseCmd("local-sqlite/main", "sql"))
	if len(messages) < 2 {
		t.Fatalf("progress messages = %d, want at least 2", len(messages))
	}
	last, ok := messages[len(messages)-1].(exportProgressMsg)
	if !ok || last.total == 0 || last.done != last.total {
		t.Fatalf("last progress = %#v", messages[len(messages)-1])
	}
}

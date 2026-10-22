package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ac "github.com/linduan/bdb/internal/complete"
	"github.com/linduan/bdb/internal/config"
	"github.com/linduan/bdb/internal/db"
	_ "github.com/linduan/bdb/internal/db/sql"
	"github.com/linduan/bdb/internal/export"
	"github.com/linduan/bdb/internal/history"
	"github.com/linduan/bdb/internal/i18n"
	"github.com/linduan/bdb/internal/query"
	"github.com/linduan/bdb/internal/sqlscript"
	uicomplete "github.com/linduan/bdb/internal/ui/complete"
	"github.com/linduan/bdb/internal/ui/connection"
	"github.com/linduan/bdb/internal/ui/detail"
	"github.com/linduan/bdb/internal/ui/editor"
	"github.com/linduan/bdb/internal/ui/menu"
	"github.com/linduan/bdb/internal/ui/results"
	"github.com/linduan/bdb/internal/ui/sidebar"
	"github.com/linduan/bdb/internal/ui/statusbar"
	"github.com/linduan/bdb/internal/ui/textutil"
	"github.com/linduan/bdb/internal/ui/theme"
)

type focus int

const (
	focusSidebar focus = iota
	focusEditor
	focusResults
)

type connOpenedMsg struct {
	name string
	conn db.Conn
	err  error
}

type queryDoneMsg struct {
	id      uint64
	outcome query.Outcome
}

type exportedMsg struct {
	path string
	err  error
}

type detailLoadedMsg struct {
	title   string
	columns []db.Column
	indexes []db.Index
	fks     []db.ForeignKey
	err     error
}

type scriptDoneMsg struct {
	total    int
	executed int
	failedAt int
	result   *db.ResultSet
	exec     *db.ExecResult
	elapsed  time.Duration
	err      error
}

type tableRef struct {
	Name   string
	Schema string
}

type completionTablesMsg struct {
	tables []tableRef
	err    error
}

type completionColumnsMsg struct {
	table   string
	columns []string
	err     error
}

type Model struct {
	cfg *config.Config
	t   *i18n.Translator

	sidebar    sidebar.Model
	editor     editor.Model
	results    results.Model
	statusbar  statusbar.Model
	options    menu.Model
	detail     detail.Model
	completion uicomplete.Model
	connForm   connection.Model

	completer    *ac.Engine
	tableSchemas map[string]string

	history       *history.Store
	pendingScript string
	themeName     string

	width, height                    int
	sideW, rightW, editorH, resultsH int

	focus focus

	conn     db.Conn
	connName string
	executor query.Executor

	querySeq uint64
	cancel   context.CancelFunc
	running  bool

	status    string
	statusErr bool
	showHelp  bool
	outDir    string
}

func New(cfg *config.Config, t *i18n.Translator) *Model {
	m := &Model{
		cfg:    cfg,
		t:      t,
		status: t.T(i18n.KeyStatusReady),
		focus:  focusSidebar,
	}
	themeName := cfg.UI.Theme
	if saved := config.LoadThemeState(); saved != "" {
		themeName = saved
	}
	m.themeName = theme.Set(themeName)
	if err := config.MergeRuntime(cfg); err != nil {
		m.setStatus(t.T(i18n.KeyErrorLoadConnections, err), true)
	}
	m.sidebar = sidebar.New(cfg.Connections, t)
	m.editor = editor.New(t)
	m.results = results.New(t)
	m.options = menu.New(t)
	m.detail = detail.New(t)
	m.completion = uicomplete.New(t)
	m.connForm = connection.New(t)
	m.completer = ac.New()
	m.tableSchemas = map[string]string{}
	if wd, err := os.Getwd(); err == nil {
		m.outDir = wd
	}
	if cfg.DefaultConnection != "" {
		m.setStatus(t.T(i18n.KeyStatusConnecting, cfg.DefaultConnection), false)
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	if name := m.cfg.DefaultConnection; name != "" {
		return m.connectCmd(name)
	}
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.options.SetSize(msg.Width, msg.Height)
		m.detail.SetSize(msg.Width, msg.Height)
		m.completion.SetSize(msg.Width, msg.Height)
		m.connForm.SetSize(msg.Width, msg.Height)
		m.layout()
		return m, nil
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	case menu.ActionMsg:
		return m, m.runMenuAction(msg)
	case menu.ChangedMsg:
		m.applyMenuChange(msg)
		return m, nil
	case detailLoadedMsg:
		if m.detail.IsOpen() && m.detail.Title() == msg.title {
			if msg.err != nil {
				m.detail.SetData(nil, nil, nil, msg.err)
				m.setStatus(m.t.T(i18n.KeyErrorDetail, msg.err), true)
			} else {
				m.detail.SetData(msg.columns, msg.indexes, msg.fks, nil)
			}
		}
		return m, nil
	case scriptDoneMsg:
		m.handleScriptDone(msg)
		return m, nil
	case completionTablesMsg:
		if msg.err == nil {
			names := make([]string, 0, len(msg.tables))
			m.tableSchemas = make(map[string]string, len(msg.tables))
			for _, ref := range msg.tables {
				names = append(names, ref.Name)
				key := strings.ToLower(ref.Name)
				if _, ok := m.tableSchemas[key]; !ok {
					m.tableSchemas[key] = ref.Schema
				}
			}
			m.completer.SetTables(names)
		}
		return m, nil
	case completionColumnsMsg:
		if msg.err == nil {
			m.completer.SetColumns(msg.table, msg.columns)
			m.setStatus(m.t.T(i18n.KeyStatusReady), false)
		}
		return m, nil
	case uicomplete.AcceptedMsg:
		if m.focus == focusEditor {
			m.editor.ReplaceBeforeCursor(msg.PrefixLen, msg.Text)
		}
		m.completion.Close()
		return m, nil
	case connection.SubmittedMsg:
		return m, m.addConnection(msg.Conn)
	case connOpenedMsg:
		return m, m.handleConnOpened(msg)
	case queryDoneMsg:
		m.handleQueryDone(msg)
		return m, nil
	case exportedMsg:
		if msg.err != nil {
			m.setStatus(m.t.T(i18n.KeyErrorExport, msg.err), true)
		} else {
			m.setStatus(m.t.T(i18n.KeyStatusExported, msg.path), false)
		}
		return m, nil
	case sidebar.ConnectMsg:
		return m, m.connectCmd(msg.Name)
	case sidebar.SelectTableMsg:
		return m, m.openTable(msg)
	case sidebar.SchemasLoadedMsg, sidebar.TablesLoadedMsg, sidebar.ColumnsLoadedMsg:
		if err := sidebar.MsgError(msg); err != nil {
			m.setStatus(m.t.T(i18n.KeyErrorCatalog, err), true)
		}
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "bdb"
	// Lets terminals that support the Kitty keyboard protocol distinguish
	// ctrl+enter from plain enter.
	v.KeyboardEnhancements = tea.KeyboardEnhancements{
		ReportAlternateKeys:        true,
		ReportAllKeysAsEscapeCodes: true,
		ReportAssociatedText:       true,
	}
	return v
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	if key == "ctrl+c" {
		m.shutdown()
		return tea.Quit
	}
	if m.options.IsOpen() {
		if key == "q" {
			m.shutdown()
			return tea.Quit
		}
		var cmd tea.Cmd
		m.options, cmd = m.options.Update(msg)
		return cmd
	}
	if m.detail.IsOpen() {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return cmd
	}
	if m.connForm.IsOpen() {
		var cmd tea.Cmd
		m.connForm, cmd = m.connForm.Update(msg)
		return cmd
	}
	if m.showHelp {
		switch key {
		case "?", "esc":
			m.showHelp = false
		case "q":
			m.shutdown()
			return tea.Quit
		}
		return nil
	}
	if key == "?" && m.focus != focusEditor {
		m.showHelp = true
		return nil
	}
	if key == "esc" {
		if m.running {
			m.cancelQuery()
			return nil
		}
		m.completion.Close()
		m.options.Open(m.menuItems())
		return nil
	}
	if key == "q" && m.focus != focusEditor {
		m.shutdown()
		return tea.Quit
	}
	if key == "a" && m.focus != focusEditor {
		m.completion.Close()
		m.connForm.Open()
		return nil
	}
	switch key {
	case "tab":
		return m.cycleFocus(1)
	case "shift+tab":
		return m.cycleFocus(-1)
	}

	switch m.focus {
	case focusSidebar:
		if key == "d" {
			if schema, table, ok := m.sidebar.SelectedTable(); ok {
				return m.openDetail(schema, table)
			}
			return nil
		}
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		return cmd
	case focusEditor:
		if key == "ctrl+space" {
			return m.refreshCompletion(true)
		}
		if m.completion.IsOpen() {
			switch key {
			case "up", "down", "ctrl+p", "ctrl+n", "tab", "enter", "esc":
				var cmd tea.Cmd
				m.completion, cmd = m.completion.Update(msg)
				return cmd
			}
		}
		switch key {
		case "ctrl+enter", "f5":
			return m.runQuery()
		case "ctrl+p":
			m.editor.BrowseHistory(true)
			return nil
		case "ctrl+n":
			m.editor.BrowseHistory(false)
			return nil
		}
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		if key == "backspace" || msg.Text != "" {
			if refresh := m.refreshCompletion(false); refresh != nil {
				return tea.Batch(cmd, refresh)
			}
		} else if m.completion.IsOpen() {
			m.completion.Close()
		}
		return cmd
	case focusResults:
		switch key {
		case "ctrl+e":
			return m.exportCmd("csv")
		case "ctrl+j":
			return m.exportCmd("json")
		case "ctrl+m":
			return m.exportCmd("md")
		case "c":
			text := m.results.CursorCell()
			if text == "" {
				return nil
			}
			m.setStatus(m.t.T(i18n.KeyStatusCopied), false)
			return tea.SetClipboard(text)
		}
		var cmd tea.Cmd
		m.results, cmd = m.results.Update(msg)
		return cmd
	}
	return nil
}

func (m *Model) cycleFocus(delta int) tea.Cmd {
	const count = 3
	m.completion.Close()
	m.focus = focus((int(m.focus) + delta + count) % count)
	m.editor.Blur()
	if m.focus == focusEditor {
		return m.editor.Focus()
	}
	return nil
}

func (m *Model) connectCmd(name string) tea.Cmd {
	connCfg := m.cfg.Find(name)
	if connCfg == nil {
		return func() tea.Msg {
			return connOpenedMsg{name: name, err: fmt.Errorf("connection %q not found", name)}
		}
	}
	m.setStatus(m.t.T(i18n.KeyStatusConnecting, name), false)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, err := db.Open(ctx, toDBConfig(connCfg))
		if err != nil {
			return connOpenedMsg{name: name, err: err}
		}
		if err := conn.Ping(ctx); err != nil {
			conn.Close()
			return connOpenedMsg{name: name, err: err}
		}
		return connOpenedMsg{name: name, conn: conn}
	}
}

func (m *Model) handleConnOpened(msg connOpenedMsg) tea.Cmd {
	if msg.err != nil {
		m.setStatus(m.t.T(i18n.KeyErrorConnect, msg.err), true)
		return nil
	}
	if m.conn != nil {
		m.conn.Close()
	}
	m.conn = msg.conn
	m.connName = msg.name
	m.executor = query.Executor{Conn: m.conn, MaxRows: m.cfg.Query.MaxRows}
	m.setStatus(m.t.T(i18n.KeyStatusConnected, msg.name), false)
	m.completer.Reset()
	m.tableSchemas = map[string]string{}
	m.completion.Close()

	cmds := []tea.Cmd{m.sidebar.SetConn(m.conn), m.loadCompletionTablesCmd()}
	if m.pendingScript != "" {
		path := m.pendingScript
		m.pendingScript = ""
		cmds = append(cmds, m.runScriptCmd(path))
	}
	return tea.Batch(cmds...)
}

func (m *Model) runQuery() tea.Cmd {
	stmt := strings.TrimSpace(m.editor.Value())
	if stmt == "" {
		m.setStatus(m.t.T(i18n.KeyStatusEmptyQuery), false)
		return nil
	}
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.querySeq++
	id := m.querySeq
	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.Query.Timeout.Std())
	m.cancel = cancel
	m.running = true
	m.editor.AddHistory(stmt)
	if store := m.history; store != nil {
		store.Add(stmt)
		go func() { _ = store.Save() }()
	}
	m.setStatus(m.t.T(i18n.KeyStatusRunning), false)

	executor := m.executor
	return func() tea.Msg {
		outcome := executor.Run(ctx, stmt)
		cancel()
		return queryDoneMsg{id: id, outcome: outcome}
	}
}

func (m *Model) cancelQuery() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.running = false
	m.setStatus(m.t.T(i18n.KeyStatusCancelled), false)
}

func (m *Model) handleQueryDone(msg queryDoneMsg) {
	if msg.id != m.querySeq {
		return
	}
	m.running = false
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	out := msg.outcome
	if out.Err != nil {
		switch {
		case errors.Is(out.Err, context.Canceled):
			m.setStatus(m.t.T(i18n.KeyStatusCancelled), false)
		case errors.Is(out.Err, context.DeadlineExceeded):
			m.setStatus(m.t.T(i18n.KeyStatusTimeout), true)
		default:
			m.setStatus(m.t.T(i18n.KeyErrorQuery, out.Err), true)
			m.results.SetError(out.Err.Error())
		}
		return
	}

	switch {
	case out.Result != nil:
		m.results.SetResult(out.Result)
		duration := humanDuration(out.Result.Elapsed)
		if out.Result.Truncated {
			m.setStatus(m.t.T(i18n.KeyStatusRowsTruncated, len(out.Result.Rows), duration), false)
		} else {
			m.setStatus(m.t.T(i18n.KeyStatusRows, len(out.Result.Rows), duration), false)
		}
	case out.Exec != nil:
		m.results.SetExec(out.Exec.Affected, out.Exec.Elapsed)
		m.setStatus(m.t.T(i18n.KeyStatusAffected, out.Exec.Affected, humanDuration(out.Exec.Elapsed)), false)
	}
}

func (m *Model) openTable(msg sidebar.SelectTableMsg) tea.Cmd {
	name := msg.Table
	if msg.Schema != "" && msg.Schema != "main" {
		name = msg.Schema + "." + msg.Table
	}
	m.editor.SetValue(fmt.Sprintf(m.t.T(i18n.KeyQuerySelectTable), name))
	m.focus = focusEditor
	return m.editor.Focus()
}

func (m *Model) exportCmd(kind string) tea.Cmd {
	rs := m.results.Result()
	if rs == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoResults), true)
		return nil
	}
	name := fmt.Sprintf("bdb-export-%s.%s", time.Now().Format("20060102-150405"), kind)
	path := filepath.Join(m.outDir, name)
	return func() tea.Msg {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
		if err != nil {
			return exportedMsg{err: err}
		}
		if kind == "csv" {
			err = export.CSV(f, rs)
		} else if kind == "json" {
			err = export.JSON(f, rs)
		} else {
			err = export.Markdown(f, rs)
		}
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return exportedMsg{err: err}
		}
		return exportedMsg{path: path}
	}
}

func (m *Model) layout() {
	if m.width < 20 || m.height < 6 {
		return
	}
	mainH := m.height - 2
	sideW := m.width / 4
	if sideW < 22 {
		sideW = 22
	}
	if sideW > 36 {
		sideW = 36
	}
	if m.width < 70 {
		sideW = m.width / 3
	}
	if sideW > m.width-10 {
		sideW = m.width - 10
	}
	rightW := m.width - sideW - 1

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

	m.sideW, m.rightW, m.editorH, m.resultsH = sideW, rightW, editorH, resultsH
	m.sidebar.SetSize(sideW, mainH)
	m.editor.SetSize(rightW, editorH-1)
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
	side := renderPane(m.t.T(i18n.KeyPaneConnections), m.t.T(i18n.KeyHintSidebar), m.sidebar.View(), m.sideW, mainH)
	editorView := renderPane(m.t.T(i18n.KeyPaneSQL), m.t.T(i18n.KeyHintEditor), m.editor.View(), m.rightW, m.editorH)
	resultsView := renderPane(m.t.T(i18n.KeyPaneResults), m.t.T(i18n.KeyHintResults), m.results.View(), m.rightW, m.resultsH)
	right := lipgloss.JoinVertical(lipgloss.Top, editorView, strings.Repeat("─", m.rightW), resultsView)
	separator := strings.Repeat("│\n", mainH-1) + "│"
	body := lipgloss.JoinHorizontal(lipgloss.Top, side, separator, right)

	m.statusbar.Width = m.width
	m.statusbar.Left = m.statusLeft()
	m.statusbar.Right = m.status
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

func (m *Model) statusLeft() string {
	parts := []string{m.t.T(i18n.KeyAppTitle)}
	switch m.focus {
	case focusSidebar:
		parts = append(parts, m.t.T(i18n.KeyPaneConnections))
	case focusEditor:
		parts = append(parts, m.t.T(i18n.KeyPaneSQL))
	case focusResults:
		parts = append(parts, m.t.T(i18n.KeyPaneResults))
	}
	if m.connName != "" {
		parts = append(parts, "● "+m.connName)
	}
	return strings.Join(parts, "  ")
}

func (m *Model) runMenuAction(msg menu.ActionMsg) tea.Cmd {
	switch msg.ID {
	case "add":
		m.completion.Close()
		m.connForm.Open()
	case "disconnect":
		m.disconnect()
	case "export.csv":
		return m.exportCmd("csv")
	case "export.json":
		return m.exportCmd("json")
	case "export.md":
		return m.exportCmd("md")
	case "help":
		m.showHelp = true
	case "quit":
		m.shutdown()
		return tea.Quit
	}
	return nil
}

func (m *Model) applyMenuChange(msg menu.ChangedMsg) {
	switch msg.ID {
	case "theme":
		names := theme.Names()
		if msg.Value < 0 || msg.Value >= len(names) {
			return
		}
		m.themeName = theme.Set(names[msg.Value])
		_ = config.SaveThemeState(m.themeName)
		m.setStatus(m.t.T(i18n.KeyStatusTheme, m.themeName), false)
	case "lang":
		langs := []string{"en", "zh"}
		if msg.Value < 0 || msg.Value >= len(langs) {
			return
		}
		m.t.SetLang(i18n.Lang(langs[msg.Value]))
		m.setStatus(m.t.T(i18n.KeyStatusLang, string(m.t.Lang())), false)
	case "timeout":
		values := timeoutChoices()
		if msg.Value < 0 || msg.Value >= len(values) {
			return
		}
		m.cfg.Query.Timeout = config.Duration(values[msg.Value])
		m.setStatus(m.t.T(i18n.KeyStatusTimeoutSet, values[msg.Value].String()), false)
	case "maxrows":
		values := maxRowChoices()
		if msg.Value < 0 || msg.Value >= len(values) {
			return
		}
		m.cfg.Query.MaxRows = values[msg.Value]
		m.executor.MaxRows = values[msg.Value]
		m.setStatus(m.t.T(i18n.KeyStatusMaxRowsSet, values[msg.Value]), false)
	}
}

func (m *Model) disconnect() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.running = false
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	m.connName = ""
	m.sidebar.ClearConn()
	m.results.Clear()
	m.setStatus(m.t.T(i18n.KeyStatusDisconnected), false)
}

func (m *Model) menuItems() []menu.Item {
	themes := theme.Names()
	themeValue := 0
	for i, name := range themes {
		if name == m.themeName {
			themeValue = i
			break
		}
	}
	langs := []string{"en", "zh"}
	langValue := 0
	if m.t.Lang() == i18n.Zh {
		langValue = 1
	}
	timeouts := timeoutChoices()
	timeoutValue := 0
	currentTimeout := m.cfg.Query.Timeout.Std()
	for i, d := range timeouts {
		if d <= currentTimeout {
			timeoutValue = i
		}
	}
	rows := maxRowChoices()
	rowValue := 2
	for i, v := range rows {
		if v == m.cfg.Query.MaxRows {
			rowValue = i
			break
		}
	}

	return []menu.Item{
		{Kind: menu.KindHeader, LabelKey: i18n.KeyMenuActions},
		{ID: "add", LabelKey: i18n.KeyCmdAdd, Kind: menu.KindAction},
		{ID: "disconnect", LabelKey: i18n.KeyCmdDisconnect, Kind: menu.KindAction},
		{ID: "export.csv", LabelKey: i18n.KeyMenuExportCSV, Kind: menu.KindAction},
		{ID: "export.json", LabelKey: i18n.KeyMenuExportJSON, Kind: menu.KindAction},
		{ID: "export.md", LabelKey: i18n.KeyMenuExportMarkdown, Kind: menu.KindAction},
		{ID: "help", LabelKey: i18n.KeyCmdHelp, Kind: menu.KindAction},
		{Kind: menu.KindHeader, LabelKey: i18n.KeyMenuSettings},
		{ID: "theme", LabelKey: i18n.KeyMenuTheme, Kind: menu.KindChoice, Choices: themes, Value: themeValue},
		{ID: "lang", LabelKey: i18n.KeyMenuLanguage, Kind: menu.KindChoice, Choices: langs, Value: langValue},
		{ID: "timeout", LabelKey: i18n.KeyMenuTimeout, Kind: menu.KindChoice, Choices: timeoutLabels(), Value: timeoutValue},
		{ID: "maxrows", LabelKey: i18n.KeyMenuMaxRows, Kind: menu.KindChoice, Choices: maxRowLabels(), Value: rowValue},
		{Kind: menu.KindHeader},
		{ID: "quit", LabelKey: i18n.KeyCmdQuit, Kind: menu.KindAction},
	}
}

func timeoutChoices() []time.Duration {
	return []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second, 120 * time.Second}
}

func timeoutLabels() []string {
	values := timeoutChoices()
	out := make([]string, len(values))
	for i, d := range values {
		out[i] = d.String()
	}
	return out
}

func maxRowChoices() []int { return []int{100, 500, 1000, 5000} }

func maxRowLabels() []string {
	values := maxRowChoices()
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strconv.Itoa(v)
	}
	return out
}

func (m *Model) openDetail(schema, table string) tea.Cmd {
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	m.detail.Open(schema, table)
	conn := m.conn
	title := m.detail.Title()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		columns, err := conn.Catalog().Columns(ctx, schema, table)
		if err != nil {
			return detailLoadedMsg{title: title, err: err}
		}
		indexes, err := conn.Catalog().Indexes(ctx, schema, table)
		if err != nil {
			return detailLoadedMsg{title: title, err: err}
		}
		fks, err := conn.Catalog().ForeignKeys(ctx, schema, table)
		if err != nil {
			return detailLoadedMsg{title: title, err: err}
		}
		return detailLoadedMsg{title: title, columns: columns, indexes: indexes, fks: fks}
	}
}

func (m *Model) runScriptCmd(path string) tea.Cmd {
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	m.cancel = cancel
	m.running = true
	m.setStatus(m.t.T(i18n.KeyStatusRunning), false)
	executor := m.executor

	return func() tea.Msg {
		start := time.Now()
		data, err := os.ReadFile(path)
		if err != nil {
			cancel()
			return scriptDoneMsg{err: err, elapsed: time.Since(start)}
		}
		statements, err := sqlscript.Split(string(data))
		if err != nil {
			cancel()
			return scriptDoneMsg{err: err, elapsed: time.Since(start)}
		}
		msg := scriptDoneMsg{total: len(statements)}
		for i, stmt := range statements {
			out := executor.Run(ctx, stmt)
			if out.Err != nil {
				msg.err = out.Err
				msg.failedAt = i + 1
				break
			}
			msg.executed++
			if out.Result != nil {
				msg.result = out.Result
			}
			if out.Exec != nil {
				msg.exec = out.Exec
			}
		}
		msg.elapsed = time.Since(start)
		cancel()
		return msg
	}
}

func (m *Model) handleScriptDone(msg scriptDoneMsg) {
	m.running = false
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if msg.err != nil {
		switch {
		case errors.Is(msg.err, context.Canceled):
			m.setStatus(m.t.T(i18n.KeyStatusCancelled), false)
		case errors.Is(msg.err, context.DeadlineExceeded):
			m.setStatus(m.t.T(i18n.KeyStatusTimeout), true)
		case msg.failedAt > 0:
			m.setStatus(m.t.T(i18n.KeyErrorScript, msg.failedAt, msg.err), true)
		default:
			m.setStatus(m.t.T(i18n.KeyErrorQuery, msg.err), true)
		}
		return
	}
	if msg.result != nil {
		m.results.SetResult(msg.result)
	} else if msg.exec != nil {
		m.results.SetExec(msg.exec.Affected, msg.exec.Elapsed)
	}
	m.setStatus(m.t.T(i18n.KeyStatusScript, msg.executed, msg.total, humanDuration(msg.elapsed)), false)
}

func (m *Model) SetHistory(store *history.Store) {
	m.history = store
	m.editor.SetHistory(store.Entries())
}

func (m *Model) SetInitialScript(path string) { m.pendingScript = path }

func (m *Model) refreshCompletion(force bool) tea.Cmd {
	if m.focus != focusEditor || m.conn == nil || m.completer == nil {
		m.completion.Close()
		return nil
	}
	word, _ := m.editor.WordBeforeCursor()
	if word == "" {
		m.completion.Close()
		return nil
	}
	qualifier, prefix := ac.SplitWord(word)
	if !force && len([]rune(prefix)) < 2 {
		m.completion.Close()
		return nil
	}
	if qualifier != "" && !m.completer.HasColumns(qualifier) {
		m.completion.Close()
		m.setStatus(m.t.T(i18n.KeyStatusCompletionLoading), false)
		return m.loadColumnsCmd(qualifier)
	}
	candidates := m.completer.Suggest(word)
	if len(candidates) == 0 {
		m.completion.Close()
		return nil
	}
	m.completion.Open(prefix, candidates)
	return nil
}

func (m *Model) loadColumnsCmd(qualifier string) tea.Cmd {
	conn := m.conn
	if conn == nil {
		return nil
	}
	schema, table := "", qualifier
	if i := strings.LastIndex(qualifier, "."); i >= 0 {
		schema, table = qualifier[:i], qualifier[i+1:]
	} else if s, ok := m.tableSchemas[strings.ToLower(table)]; ok {
		schema = s
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		columns, err := conn.Catalog().Columns(ctx, schema, table)
		if err != nil {
			return completionColumnsMsg{table: qualifier, err: err}
		}
		names := make([]string, 0, len(columns))
		for _, column := range columns {
			names = append(names, column.Name)
		}
		return completionColumnsMsg{table: qualifier, columns: names}
	}
}

func (m *Model) loadCompletionTablesCmd() tea.Cmd {
	conn := m.conn
	if conn == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		schemas, err := conn.Catalog().Schemas(ctx)
		if err != nil {
			return completionTablesMsg{err: err}
		}
		seen := map[string]bool{}
		var refs []tableRef
		for _, schema := range schemas {
			objects, err := conn.Catalog().Tables(ctx, schema)
			if err != nil {
				return completionTablesMsg{err: err}
			}
			for _, object := range objects {
				key := strings.ToLower(object.Name)
				if seen[key] {
					continue
				}
				seen[key] = true
				refs = append(refs, tableRef{Name: object.Name, Schema: object.Schema})
			}
		}
		return completionTablesMsg{tables: refs}
	}
}

func (m *Model) shutdown() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	if store := m.history; store != nil {
		_ = store.Save()
	}
}

func (m *Model) addConnection(conn config.Connection) tea.Cmd {
	if m.cfg.Find(conn.Name) != nil {
		m.connForm.SetError(m.t.T(i18n.KeyFormDuplicate, conn.Name))
		return nil
	}
	if err := config.AppendRuntimeConnection(conn); err != nil {
		m.connForm.SetError(m.t.T(i18n.KeyErrorSaveConnection, err))
		return nil
	}
	m.cfg.Connections = append(m.cfg.Connections, conn)
	m.sidebar.AddConnection(conn)
	m.setStatus(m.t.T(i18n.KeyStatusConnectionAdded, conn.Name), false)
	return m.connectCmd(conn.Name)
}

func (m *Model) setStatus(text string, isError bool) {
	m.status = text
	m.statusErr = isError
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

func toDBConfig(c *config.Connection) db.Config {
	return db.Config{
		Name: c.Name,
		Type: c.Type,
		Params: map[string]string{
			"host":     c.Host,
			"port":     strconv.Itoa(c.EffectivePort()),
			"user":     c.User,
			"password": c.Password,
			"database": c.Database,
			"sslmode":  c.SSLMode,
			"path":     c.Path,
			"addr":     c.Addr,
			"uri":      c.URI,
			"db":       strconv.Itoa(c.DB),
		},
	}
}

func humanDuration(d time.Duration) string {
	if d < time.Millisecond {
		return d.Round(time.Microsecond).String()
	}
	return d.Round(time.Millisecond).String()
}

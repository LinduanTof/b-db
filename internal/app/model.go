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

	ac "github.com/linduan-guan/b-db/internal/complete"
	"github.com/linduan-guan/b-db/internal/config"
	"github.com/linduan-guan/b-db/internal/db"
	_ "github.com/linduan-guan/b-db/internal/db/mongo"
	_ "github.com/linduan-guan/b-db/internal/db/redis"
	_ "github.com/linduan-guan/b-db/internal/db/sql"
	"github.com/linduan-guan/b-db/internal/export"
	"github.com/linduan-guan/b-db/internal/history"
	"github.com/linduan-guan/b-db/internal/i18n"
	"github.com/linduan-guan/b-db/internal/query"
	"github.com/linduan-guan/b-db/internal/sqlscript"
	"github.com/linduan-guan/b-db/internal/ui/celledit"
	uicomplete "github.com/linduan-guan/b-db/internal/ui/complete"
	"github.com/linduan-guan/b-db/internal/ui/confirm"
	"github.com/linduan-guan/b-db/internal/ui/connection"
	"github.com/linduan-guan/b-db/internal/ui/detail"
	"github.com/linduan-guan/b-db/internal/ui/editor"
	"github.com/linduan-guan/b-db/internal/ui/menu"
	"github.com/linduan-guan/b-db/internal/ui/results"
	"github.com/linduan-guan/b-db/internal/ui/sidebar"
	"github.com/linduan-guan/b-db/internal/ui/statusbar"
	"github.com/linduan-guan/b-db/internal/ui/textutil"
	"github.com/linduan-guan/b-db/internal/ui/textview"
	"github.com/linduan-guan/b-db/internal/ui/theme"
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

type editInfo struct {
	columns []db.Column
	primary []string
}

type editTarget struct {
	table    db.TableRef
	column   string
	dataType string
	key      map[string]any
}

type editInfoMsg struct {
	schema  string
	table   string
	action  string
	columns []db.Column
	primary []string
	err     error
}

type mutationDoneMsg struct {
	kind     string
	affected int64
	err      error
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

type ddlLoadedMsg struct {
	title string
	text  string
	err   error
}

type indexesLoadedMsg struct {
	title string
	text  string
	err   error
}

type Model struct {
	cfg *config.Config
	t   *i18n.Translator

	sidebar    sidebar.Model
	editor     editor.Model
	results    results.Model
	statusbar  statusbar.Model
	options    menu.Model
	tableMenu  menu.Model
	textView   textview.Model
	detail     detail.Model
	completion uicomplete.Model
	connForm   connection.Model
	cellEdit   celledit.Model
	confirm    confirm.Model

	pendingCmd    tea.Cmd
	pendingSource *db.TableRef
	editCache     map[string]editInfo
	editTarget    editTarget

	tableMenuSchema string
	tableMenuTable  string
	tableMenuKind   string

	completer    *ac.Engine
	tableSchemas map[string]string

	history       *history.Store
	pendingScript string
	themeName     string

	width, height                    int
	sideW, rightW, editorH, resultsH int

	focus focus

	focusMode  string
	zoom       bool
	autoPaused bool

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
	m.focusMode = cfg.UI.FocusMode
	if m.focusMode != config.FocusModeAuto {
		m.focusMode = config.FocusModeManual
	}
	if err := config.MergeRuntime(cfg); err != nil {
		m.setStatus(t.T(i18n.KeyErrorLoadConnections, err), true)
	}
	m.sidebar = sidebar.New(cfg.Connections, t)
	m.editor = editor.New(t)
	m.results = results.New(t)
	m.options = menu.New(t)
	m.tableMenu = menu.New(t)
	m.textView = textview.New(t)
	m.detail = detail.New(t)
	m.completion = uicomplete.New(t)
	m.connForm = connection.New(t)
	m.cellEdit = celledit.New(t)
	m.confirm = confirm.New(t)
	m.editCache = map[string]editInfo{}
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
		m.tableMenu.SetSize(msg.Width, msg.Height)
		m.textView.SetSize(msg.Width, msg.Height)
		m.detail.SetSize(msg.Width, msg.Height)
		m.completion.SetSize(msg.Width, msg.Height)
		m.connForm.SetSize(msg.Width, msg.Height)
		m.cellEdit.SetSize(msg.Width, msg.Height)
		m.confirm.SetSize(msg.Width, msg.Height)
		m.layout()
		return m, nil
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	case tea.MouseClickMsg:
		return m, m.handleMouseClick(msg)
	case tea.MouseWheelMsg:
		return m, m.handleMouseWheel(msg)
	case menu.ActionMsg:
		if strings.HasPrefix(msg.ID, "table.") {
			return m, m.runTableMenuAction(msg)
		}
		return m, m.runMenuAction(msg)
	case menu.ChangedMsg:
		m.applyMenuChange(msg)
		return m, nil
	case sidebar.TableMenuMsg:
		m.tableMenuSchema, m.tableMenuTable, m.tableMenuKind = msg.Schema, msg.Table, msg.Kind
		m.tableMenu.Open(qualifiedTable(msg.Schema, msg.Table), m.tableMenuItems())
		return m, nil
	case ddlLoadedMsg:
		if msg.err != nil {
			m.textView.Open(msg.title, m.t.T(i18n.KeyErrorDDL, msg.err))
		} else {
			m.textView.Open(msg.title, msg.text)
		}
		return m, nil
	case indexesLoadedMsg:
		if msg.err != nil {
			m.textView.Open(msg.title, m.t.T(i18n.KeyErrorIndexes, msg.err))
		} else {
			m.textView.Open(msg.title, msg.text)
		}
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
	case celledit.SubmittedMsg:
		return m, m.openUpdateConfirm(msg.Value)
	case confirm.ConfirmedMsg:
		cmd := m.pendingCmd
		m.pendingCmd = nil
		m.confirm.Close()
		return m, cmd
	case editInfoMsg:
		return m, m.handleEditInfo(msg)
	case mutationDoneMsg:
		return m, m.handleMutationDone(msg)
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
	v.MouseMode = tea.MouseModeCellMotion
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

func (m *Model) handleMouseClick(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Button != tea.MouseLeft || m.anyOverlayOpen() {
		return nil
	}
	x, y := msg.X, msg.Y
	if m.effectiveZoom() {
		if y < 0 || y >= m.height-2 {
			return nil
		}
		switch m.focus {
		case focusEditor:
			return m.editor.Focus()
		case focusResults:
			m.results.HandleClick(x, y-1)
		default:
			return m.sidebar.HandleClick(y - 1)
		}
		return nil
	}
	if x < m.sideW {
		if y < 0 || y >= m.height-2 {
			return nil
		}
		m.editor.Blur()
		m.focus = focusSidebar
		m.layout()
		if y == 0 {
			return nil
		}
		return m.sidebar.HandleClick(y - 1)
	}

	editorStart := m.sideW + 1
	if x < editorStart || x >= editorStart+m.rightW {
		return nil
	}
	relX := x - editorStart
	switch {
	case y >= 0 && y < m.editorH:
		m.focus = focusEditor
		m.layout()
		return m.editor.Focus()
	case y >= m.editorH+1 && y < m.editorH+1+m.resultsH:
		m.editor.Blur()
		m.focus = focusResults
		m.layout()
		relY := y - (m.editorH + 2)
		if relY >= 0 {
			m.results.HandleClick(relX, relY)
		}
	}
	return nil
}

func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) tea.Cmd {
	delta := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = -1
	case tea.MouseWheelDown:
		delta = 1
	default:
		return nil
	}

	if m.textView.IsOpen() {
		m.textView.Scroll(delta)
		return nil
	}
	if m.detail.IsOpen() {
		m.detail.Scroll(delta)
		return nil
	}
	if m.anyOverlayOpen() {
		return nil
	}

	if m.effectiveZoom() {
		switch m.focus {
		case focusResults:
			m.results.Scroll(delta)
		case focusSidebar:
			m.sidebar.Scroll(delta)
		}
		return nil
	}

	x, y := msg.X, msg.Y
	if x < m.sideW && y >= 0 && y < m.height-2 {
		m.sidebar.Scroll(delta)
		return nil
	}
	editorStart := m.sideW + 1
	if x >= editorStart && x < editorStart+m.rightW &&
		y >= m.editorH+2 && y < m.editorH+1+m.resultsH {
		m.results.Scroll(delta)
	}
	return nil
}

func (m *Model) anyOverlayOpen() bool {
	return m.options.IsOpen() || m.tableMenu.IsOpen() || m.textView.IsOpen() ||
		m.detail.IsOpen() || m.connForm.IsOpen() || m.completion.IsOpen() || m.showHelp
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
	if m.tableMenu.IsOpen() {
		if key == "q" {
			m.shutdown()
			return tea.Quit
		}
		var cmd tea.Cmd
		m.tableMenu, cmd = m.tableMenu.Update(msg)
		return cmd
	}
	if m.textView.IsOpen() {
		var cmd tea.Cmd
		m.textView, cmd = m.textView.Update(msg)
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
	if m.cellEdit.IsOpen() {
		var cmd tea.Cmd
		m.cellEdit, cmd = m.cellEdit.Update(msg)
		return cmd
	}
	if m.confirm.IsOpen() {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
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
		m.options.Open("", m.menuItems())
		return nil
	}
	if key == "f2" {
		m.toggleZoom()
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
		case "enter", "v":
			text := m.results.CursorText()
			if text == "" {
				return nil
			}
			title := fmt.Sprintf(m.t.T(i18n.KeyCellTitle), m.results.CursorColumnName())
			m.textView.Open(title, text)
			return nil
		case "e":
			return m.beginCellEdit()
		case "x", "delete":
			return m.beginDeleteRow()
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
	var cmd tea.Cmd
	if m.focus == focusEditor {
		cmd = m.editor.Focus()
	}
	m.layout()
	return cmd
}

func (m *Model) effectiveZoom() bool {
	if m.focusMode == config.FocusModeAuto {
		if m.autoPaused {
			return false
		}
		return m.focus != focusSidebar
	}
	return m.zoom
}

func (m *Model) toggleZoom() {
	if m.focusMode == config.FocusModeAuto {
		m.autoPaused = !m.autoPaused
	} else {
		m.zoom = !m.zoom
	}
	m.layout()
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
	m.executor = query.Executor{
		Conn:        m.conn,
		MaxRows:     m.cfg.Query.MaxRows,
		AlwaysQuery: m.conn.Type() == "redis" || m.conn.Type() == "mongo",
	}
	m.editCache = map[string]editInfo{}
	m.editTarget = editTarget{}
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
		_ = store.Save()
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
	defer func() {
		m.pendingSource = nil
	}()
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
		if m.pendingSource != nil {
			m.results.SetSource(m.pendingSource.Schema, m.pendingSource.Name)
		}
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
	if m.conn != nil && m.conn.Type() == "redis" {
		m.editor.SetValue(redisCommand(msg.Kind, msg.Table))
		m.focus = focusEditor
		m.layout()
		return m.editor.Focus()
	}
	if m.conn != nil && m.conn.Type() == "mongo" {
		m.editor.SetValue(mongoCommand(msg.Schema, msg.Table))
		m.focus = focusEditor
		m.layout()
		return m.editor.Focus()
	}
	m.editor.SetValue(fmt.Sprintf(m.t.T(i18n.KeyQuerySelectTable), qualifiedTable(msg.Schema, msg.Table)))
	m.focus = focusEditor
	m.layout()
	return m.editor.Focus()
}

func mongoCommand(schema, table string) string {
	return "find " + schema + "." + table + " {}"
}

func (m *Model) mongoTableData(schema, table string) tea.Cmd {
	m.editor.SetValue(mongoCommand(schema, table))
	m.focus = focusResults
	m.layout()
	return m.runQuery()
}

func redisCommand(kind, key string) string {
	quoted := quoteRedisArg(key)
	switch strings.ToLower(kind) {
	case "string":
		return "GET " + quoted
	case "hash":
		return "HGETALL " + quoted
	case "list":
		return "LRANGE " + quoted + " 0 99"
	case "set":
		return "SMEMBERS " + quoted
	case "zset":
		return "ZRANGE " + quoted + " 0 99 WITHSCORES"
	case "stream":
		return "XRANGE " + quoted + " - + COUNT 100"
	default:
		return "TYPE " + quoted
	}
}

func quoteRedisArg(value string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
}

func (m *Model) redisTableData(schema, table, kind string) tea.Cmd {
	m.editor.SetValue(redisCommand(kind, table))
	m.focus = focusResults
	m.layout()
	return m.runQuery()
}

func qualifiedTable(schema, table string) string {
	if schema != "" && schema != "main" {
		return schema + "." + table
	}
	return table
}

func (m *Model) tableData(schema, table string) tea.Cmd {
	m.editor.SetValue(fmt.Sprintf(m.t.T(i18n.KeyQuerySelectTable), qualifiedTable(schema, table)))
	m.focus = focusEditor
	m.layout()
	m.pendingSource = &db.TableRef{Schema: schema, Name: table}
	focus := m.editor.Focus()
	run := m.runQuery()
	return tea.Batch(focus, run)
}

func (m *Model) tableMenuItems() []menu.Item {
	if m.conn != nil && m.conn.Type() == "redis" {
		return []menu.Item{
			{ID: "table.data", LabelKey: i18n.KeyTableData, Kind: menu.KindAction},
			{ID: "table.select", LabelKey: i18n.KeyTableSelect, Kind: menu.KindAction},
		}
	}
	if m.conn != nil && m.conn.Type() == "mongo" {
		return []menu.Item{
			{ID: "table.data", LabelKey: i18n.KeyTableData, Kind: menu.KindAction},
			{ID: "table.structure", LabelKey: i18n.KeyTableStructure, Kind: menu.KindAction},
			{ID: "table.indexes", LabelKey: i18n.KeyTableIndexes, Kind: menu.KindAction},
			{ID: "table.select", LabelKey: i18n.KeyTableSelect, Kind: menu.KindAction},
		}
	}
	return []menu.Item{
		{ID: "table.data", LabelKey: i18n.KeyTableData, Kind: menu.KindAction},
		{ID: "table.ddl", LabelKey: i18n.KeyTableDDL, Kind: menu.KindAction},
		{ID: "table.structure", LabelKey: i18n.KeyTableStructure, Kind: menu.KindAction},
		{ID: "table.indexes", LabelKey: i18n.KeyTableIndexes, Kind: menu.KindAction},
		{ID: "table.select", LabelKey: i18n.KeyTableSelect, Kind: menu.KindAction},
		{ID: "table.insert", LabelKey: i18n.KeyMenuInsert, Kind: menu.KindAction},
	}
}

func (m *Model) runTableMenuAction(msg menu.ActionMsg) tea.Cmd {
	schema, table := m.tableMenuSchema, m.tableMenuTable
	if m.conn != nil && m.conn.Type() == "redis" {
		switch msg.ID {
		case "table.data":
			return m.redisTableData(schema, table, m.tableMenuKind)
		case "table.select":
			return m.openTable(sidebar.SelectTableMsg{Schema: schema, Table: table, Kind: m.tableMenuKind})
		}
		m.setStatus(m.t.T(i18n.KeyErrorUnsupportedFeature, "redis"), true)
		return nil
	}
	if m.conn != nil && m.conn.Type() == "mongo" {
		switch msg.ID {
		case "table.data":
			return m.mongoTableData(schema, table)
		case "table.select":
			return m.openTable(sidebar.SelectTableMsg{Schema: schema, Table: table, Kind: m.tableMenuKind})
		}
	}
	switch msg.ID {
	case "table.data":
		return m.tableData(schema, table)
	case "table.select":
		return m.openTable(sidebar.SelectTableMsg{Schema: schema, Table: table})
	case "table.structure":
		return m.openDetail(schema, table)
	case "table.ddl":
		return m.openDDL(schema, table)
	case "table.indexes":
		return m.openIndexes(schema, table)
	case "table.insert":
		return m.editInfoCmd(schema, table, "insert")
	}
	return nil
}

func (m *Model) openDDL(schema, table string) tea.Cmd {
	conn := m.conn
	if conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	title := fmt.Sprintf(m.t.T(i18n.KeyTableDDLTitle), qualifiedTable(schema, table))
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ddl, err := conn.Catalog().DDL(ctx, schema, table)
		return ddlLoadedMsg{title: title, text: ddl, err: err}
	}
}

func (m *Model) openIndexes(schema, table string) tea.Cmd {
	conn := m.conn
	if conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	title := fmt.Sprintf(m.t.T(i18n.KeyTableIndexesTitle), qualifiedTable(schema, table))
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		indexes, err := conn.Catalog().Indexes(ctx, schema, table)
		if err != nil {
			return indexesLoadedMsg{title: title, err: err}
		}
		return indexesLoadedMsg{title: title, text: formatIndexes(indexes)}
	}
}

func formatIndexes(indexes []db.Index) string {
	if len(indexes) == 0 {
		return "(none)"
	}
	lines := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		unique := ""
		if idx.Unique {
			unique = " UNIQUE"
		}
		lines = append(lines, fmt.Sprintf("%s%s (%s)", idx.Name, unique, strings.Join(idx.Columns, ", ")))
	}
	return strings.Join(lines, "\n")
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
		m.sidebar.SetSize(max(1, m.sideW), mainH)
		m.editor.SetSize(max(1, m.rightW), max(1, m.editorH-1))
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
	if m.tableMenu.IsOpen() {
		return textutil.Overlay(base, m.tableMenu.View(), m.width, m.height)
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
			body = renderPane(m.t.T(i18n.KeyPaneSQL), m.t.T(i18n.KeyHintEditor), m.editor.View(), m.width, mainH)
		case focusResults:
			body = renderPane(m.t.T(i18n.KeyPaneResults), m.t.T(i18n.KeyHintResults), m.results.View(), m.width, mainH)
		default:
			body = renderPane(m.t.T(i18n.KeyPaneConnections), m.t.T(i18n.KeyHintSidebar), m.sidebar.View(), m.width, mainH)
		}
	} else {
		side := renderPane(m.t.T(i18n.KeyPaneConnections), m.t.T(i18n.KeyHintSidebar), m.sidebar.View(), m.sideW, mainH)
		editorView := renderPane(m.t.T(i18n.KeyPaneSQL), m.t.T(i18n.KeyHintEditor), m.editor.View(), m.rightW, m.editorH)
		resultsView := renderPane(m.t.T(i18n.KeyPaneResults), m.t.T(i18n.KeyHintResults), m.results.View(), m.rightW, m.resultsH)
		right := lipgloss.JoinVertical(lipgloss.Top, editorView, strings.Repeat("─", m.rightW), resultsView)
		separator := strings.Repeat("│\n", mainH-1) + "│"
		body = lipgloss.JoinHorizontal(lipgloss.Top, side, separator, right)
	}

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
	case "zoom":
		m.toggleZoom()
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
		{ID: "zoom", LabelKey: i18n.KeyMenuLayout, Kind: menu.KindAction},
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

func (m *Model) readOnly() bool {
	c := m.cfg.Find(m.connName)
	return c != nil && c.ReadOnly
}

func editInfoKey(schema, table string) string {
	return schema + "." + table
}

func (m *Model) editInfoCmd(schema, table, action string) tea.Cmd {
	if info, ok := m.editCache[editInfoKey(schema, table)]; ok {
		return m.useEditInfo(schema, table, action, info)
	}
	conn := m.conn
	if conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		columns, err := conn.Catalog().Columns(ctx, schema, table)
		if err != nil {
			return editInfoMsg{schema: schema, table: table, action: action, err: err}
		}
		primary := make([]string, 0, len(columns))
		for _, column := range columns {
			if column.PrimaryKey {
				primary = append(primary, column.Name)
			}
		}
		return editInfoMsg{schema: schema, table: table, action: action, columns: columns, primary: primary}
	}
}

func (m *Model) handleEditInfo(msg editInfoMsg) tea.Cmd {
	if msg.err != nil {
		m.setStatus(m.t.T(i18n.KeyErrorEdit, msg.err), true)
		return nil
	}
	info := editInfo{columns: msg.columns, primary: msg.primary}
	m.editCache[editInfoKey(msg.schema, msg.table)] = info
	return m.useEditInfo(msg.schema, msg.table, msg.action, info)
}

func (m *Model) useEditInfo(schema, table, action string, info editInfo) tea.Cmd {
	switch action {
	case "insert":
		return m.insertTemplate(schema, table, info.columns)
	case "delete":
		m.openDeleteConfirm(schema, table, info)
	default:
		m.openCellEditor(schema, table, info)
	}
	return nil
}

func (m *Model) beginCellEdit() tea.Cmd {
	schema, table, ok := m.results.Source()
	if !ok {
		m.setStatus(m.t.T(i18n.KeyErrorEditSource), true)
		return nil
	}
	if m.readOnly() {
		m.setStatus(m.t.T(i18n.KeyErrorReadOnly), true)
		return nil
	}
	return m.editInfoCmd(schema, table, "edit")
}

func (m *Model) beginDeleteRow() tea.Cmd {
	schema, table, ok := m.results.Source()
	if !ok {
		m.setStatus(m.t.T(i18n.KeyErrorEditSource), true)
		return nil
	}
	if m.readOnly() {
		m.setStatus(m.t.T(i18n.KeyErrorReadOnly), true)
		return nil
	}
	return m.editInfoCmd(schema, table, "delete")
}

func (m *Model) openCellEditor(schema, table string, info editInfo) {
	if len(info.primary) == 0 {
		m.setStatus(m.t.T(i18n.KeyErrorNoPrimaryKey), true)
		return
	}
	key, ok := m.results.RowKey(info.primary)
	if !ok {
		m.setStatus(m.t.T(i18n.KeyErrorNoPrimaryKey), true)
		return
	}
	column := m.results.CursorColumnName()
	if column == "" {
		return
	}
	dataType := ""
	for _, c := range info.columns {
		if c.Name == column {
			dataType = c.DataType
			break
		}
	}
	m.editTarget = editTarget{
		table:    db.TableRef{Schema: schema, Name: table},
		column:   column,
		dataType: dataType,
		key:      key,
	}
	title := fmt.Sprintf(m.t.T(i18n.KeyCellEditTitle), qualifiedTable(schema, table)+"."+column)
	m.cellEdit.Open(title, m.results.CursorCell())
}

func (m *Model) openDeleteConfirm(schema, table string, info editInfo) {
	if len(info.primary) == 0 {
		m.setStatus(m.t.T(i18n.KeyErrorNoPrimaryKey), true)
		return
	}
	key, ok := m.results.RowKey(info.primary)
	if !ok {
		m.setStatus(m.t.T(i18n.KeyErrorNoPrimaryKey), true)
		return
	}
	mut := m.conn.Mutator()
	if mut == nil {
		m.setStatus(m.t.T(i18n.KeyErrorMutation, "mutations are not supported"), true)
		return
	}
	tableRef := db.TableRef{Schema: schema, Name: table}
	m.pendingCmd = m.mutationCmd("delete", func(ctx context.Context, mut db.Mutator) (int64, error) {
		return mut.DeleteRow(ctx, tableRef, key)
	})
	m.confirm.Open(m.t.T(i18n.KeyConfirmDelete), mut.PreviewDelete(tableRef, key))
}

func (m *Model) openUpdateConfirm(raw string) tea.Cmd {
	target := m.editTarget
	if target.column == "" || m.conn == nil {
		return nil
	}
	mut := m.conn.Mutator()
	if mut == nil {
		m.setStatus(m.t.T(i18n.KeyErrorMutation, "mutations are not supported"), true)
		return nil
	}
	value := parseCellValue(raw, target.dataType)
	m.pendingCmd = m.mutationCmd("update", func(ctx context.Context, mut db.Mutator) (int64, error) {
		return mut.UpdateCell(ctx, target.table, target.key, target.column, value)
	})
	m.confirm.Open(m.t.T(i18n.KeyConfirmUpdate), mut.PreviewUpdate(target.table, target.key, target.column, value))
	return nil
}

func (m *Model) mutationCmd(kind string, run func(context.Context, db.Mutator) (int64, error)) tea.Cmd {
	mut := m.conn.Mutator()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		affected, err := run(ctx, mut)
		return mutationDoneMsg{kind: kind, affected: affected, err: err}
	}
}

func (m *Model) handleMutationDone(msg mutationDoneMsg) tea.Cmd {
	if msg.err != nil {
		m.setStatus(m.t.T(i18n.KeyErrorMutation, msg.err), true)
		return nil
	}
	if msg.kind == "delete" {
		m.setStatus(m.t.T(i18n.KeyStatusDeleted), false)
	} else {
		m.setStatus(m.t.T(i18n.KeyStatusUpdated), false)
	}
	return m.refreshSource()
}

func (m *Model) refreshSource() tea.Cmd {
	schema, table, ok := m.results.Source()
	if !ok {
		return nil
	}
	stmt := fmt.Sprintf(m.t.T(i18n.KeyQuerySelectTable), qualifiedTable(schema, table))
	m.editor.SetValue(stmt)
	m.pendingSource = &db.TableRef{Schema: schema, Name: table}
	return m.runQuery()
}

func (m *Model) insertTemplate(schema, table string, columns []db.Column) tea.Cmd {
	names := make([]string, 0, len(columns))
	values := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, column.Name)
		values = append(values, "NULL")
	}
	stmt := fmt.Sprintf(m.t.T(i18n.KeyQueryInsertTemplate), qualifiedTable(schema, table), strings.Join(names, ", "), strings.Join(values, ", "))
	m.editor.SetValue(stmt)
	m.focus = focusEditor
	m.layout()
	return m.editor.Focus()
}

func parseCellValue(raw, dataType string) any {
	trimmed := strings.TrimSpace(raw)
	kind := strings.ToLower(dataType)
	switch {
	case strings.Contains(kind, "int"), strings.Contains(kind, "serial"):
		if value, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return value
		}
	case strings.Contains(kind, "numeric"), strings.Contains(kind, "decimal"),
		strings.Contains(kind, "real"), strings.Contains(kind, "double"), strings.Contains(kind, "float"):
		if value, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return value
		}
	case strings.Contains(kind, "bool"):
		if value, err := strconv.ParseBool(trimmed); err == nil {
			return value
		}
	}
	return raw
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

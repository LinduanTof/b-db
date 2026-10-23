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

	ac "github.com/LinduanTof/b-db/internal/complete"
	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/db"
	_ "github.com/LinduanTof/b-db/internal/db/mongo"
	_ "github.com/LinduanTof/b-db/internal/db/redis"
	_ "github.com/LinduanTof/b-db/internal/db/sql"
	"github.com/LinduanTof/b-db/internal/export"
	"github.com/LinduanTof/b-db/internal/history"
	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/query"
	"github.com/LinduanTof/b-db/internal/sqlscript"
	"github.com/LinduanTof/b-db/internal/ui/celledit"
	uicomplete "github.com/LinduanTof/b-db/internal/ui/complete"
	"github.com/LinduanTof/b-db/internal/ui/confirm"
	"github.com/LinduanTof/b-db/internal/ui/connection"
	"github.com/LinduanTof/b-db/internal/ui/detail"
	"github.com/LinduanTof/b-db/internal/ui/editor"
	"github.com/LinduanTof/b-db/internal/ui/menu"
	"github.com/LinduanTof/b-db/internal/ui/results"
	"github.com/LinduanTof/b-db/internal/ui/sidebar"
	"github.com/LinduanTof/b-db/internal/ui/statusbar"
	"github.com/LinduanTof/b-db/internal/ui/textview"
	"github.com/LinduanTof/b-db/internal/ui/theme"
)

type focus int

const (
	focusSidebar focus = iota
	focusEditor
	focusResults
)

type connOpenedMsg struct {
	id   uint64
	name string
	conn db.Conn
	err  error
}

type queryDoneMsg struct {
	id      uint64
	outcome query.Outcome
}

type tablePageDoneMsg struct {
	id      uint64
	outcome query.Outcome
	page    *pagination
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
	wasNull  bool
}

type editInfoMsg struct {
	id      uint64
	schema  string
	table   string
	action  string
	columns []db.Column
	primary []string
	err     error
}

type mutationDoneMsg struct {
	id       uint64
	kind     string
	affected int64
	err      error
}

type exportedMsg struct {
	path string
	err  error
}

type detailLoadedMsg struct {
	id      uint64
	title   string
	columns []db.Column
	indexes []db.Index
	fks     []db.ForeignKey
	err     error
}

type scriptDoneMsg struct {
	id       uint64
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
	id     uint64
	tables []tableRef
	err    error
}

type completionColumnsMsg struct {
	id      uint64
	table   string
	columns []string
	err     error
}

type ddlLoadedMsg struct {
	id    uint64
	title string
	text  string
	err   error
}

type indexesLoadedMsg struct {
	id    uint64
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

	pendingCmd tea.Cmd
	pager      *pagination
	editCache  map[string]editInfo
	editTarget editTarget

	tabs           []editorTab
	activeTab      int
	tabConnName    string
	tabStates      map[string]savedTabs
	tabSaveGen     uint64
	tabSavePending bool

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
	sessions map[string]*session
	executor query.Executor

	querySeq       uint64
	connectSeq     uint64
	cancel         context.CancelFunc
	mutationCancel context.CancelFunc
	running        bool

	status    string
	statusErr bool
	showHelp  bool
	outDir    string
}

type session struct {
	conn         db.Conn
	connName     string
	results      results.Model
	completer    *ac.Engine
	tableSchemas map[string]string
	editCache    map[string]editInfo
	pager        *pagination
	tabs         []editorTab
	activeTab    int
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
	m.sidebar.SetConfigPath(cfg.Path)
	m.tabStates, _ = loadTabStates()
	if m.tabStates == nil {
		m.tabStates = map[string]savedTabs{}
	}
	m.resetTabs()
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
	m.sessions = map[string]*session{}
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
		if msg.id != m.connectSeq {
			return m, nil
		}
		if msg.err != nil {
			m.textView.Open(msg.title, m.t.T(i18n.KeyErrorDDL, msg.err))
		} else {
			m.textView.Open(msg.title, msg.text)
		}
		return m, nil
	case indexesLoadedMsg:
		if msg.id != m.connectSeq {
			return m, nil
		}
		if msg.err != nil {
			m.textView.Open(msg.title, m.t.T(i18n.KeyErrorIndexes, msg.err))
		} else {
			m.textView.Open(msg.title, msg.text)
		}
		return m, nil
	case detailLoadedMsg:
		if msg.id != m.connectSeq {
			return m, nil
		}
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
		if msg.id != m.connectSeq {
			return m, nil
		}
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
		if msg.id != m.connectSeq {
			return m, nil
		}
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
		return m, m.scheduleTabSave()
	case connection.SubmittedMsg:
		return m, m.addConnection(msg.Conn)
	case connOpenedMsg:
		return m, m.handleConnOpened(msg)
	case queryDoneMsg:
		return m, m.handleQueryDone(msg)
	case tablePageDoneMsg:
		return m, m.handleTablePageDone(msg)
	case tabSaveMsg:
		if msg.gen != m.tabSaveGen {
			return m, nil
		}
		m.tabSavePending = false
		_ = m.saveTabStatesNow()
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
		return m, m.switchToConnection(msg.Name)
	case sidebar.SelectTableMsg:
		return m, m.openTable(msg)
	case sidebar.TableOpenMsg:
		return m, m.openTableData(msg.Schema, msg.Table, msg.Kind)
	case sidebar.SchemasLoadedMsg, sidebar.TablesLoadedMsg, sidebar.ColumnsLoadedMsg:
		if err := sidebar.MsgError(msg); err != nil {
			m.setStatus(m.t.T(i18n.KeyErrorCatalog, err), true)
		}
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		return m, cmd
	case tea.PasteMsg:
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		if m.focus == focusEditor {
			return m, tea.Batch(cmd, m.scheduleTabSave())
		}
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
		m.detail.IsOpen() || m.connForm.IsOpen() || m.cellEdit.IsOpen() ||
		m.confirm.IsOpen() || m.completion.IsOpen() || m.showHelp
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
		if !m.confirm.IsOpen() && cmd == nil {
			m.pendingCmd = nil
			m.cancelMutation()
		}
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
		switch key {
		case "ctrl+t":
			return m.newTab()
		case "ctrl+w":
			return m.closeTab()
		case "ctrl+tab":
			return m.switchTab(1)
		case "ctrl+shift+tab":
			return m.switchTab(-1)
		}
		if (key == "alt+left" || key == "alt+right") && len(m.tabs) > 1 {
			if key == "alt+left" {
				return m.switchTab(-1)
			}
			return m.switchTab(1)
		}
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
			return m.scheduleTabSave()
		case "ctrl+n":
			m.editor.BrowseHistory(false)
			return m.scheduleTabSave()
		}
		var cmd tea.Cmd
		before := m.editor.Value()
		m.editor, cmd = m.editor.Update(msg)
		var save tea.Cmd
		if m.editor.Value() != before {
			save = m.scheduleTabSave()
		}
		if key == "backspace" || msg.Text != "" {
			if refresh := m.refreshCompletion(false); refresh != nil {
				return tea.Batch(cmd, refresh, save)
			}
		} else if m.completion.IsOpen() {
			m.completion.Close()
		}
		return tea.Batch(cmd, save)
	case focusResults:
		switch key {
		case "n":
			return m.nextPage()
		case "p":
			return m.previousPage()
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
		case "o":
			if m.pager != nil {
				return m.toggleTableSort()
			}
		case "O":
			if m.pager != nil {
				return m.clearTableSort()
			}
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
	m.connectSeq++
	id := m.connectSeq
	connCfg := m.cfg.Find(name)
	if connCfg == nil {
		return func() tea.Msg {
			return connOpenedMsg{id: id, name: name, err: fmt.Errorf("connection %q not found", name)}
		}
	}
	m.setStatus(m.t.T(i18n.KeyStatusConnecting, name), false)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, err := db.Open(ctx, toDBConfig(connCfg))
		if err != nil {
			return connOpenedMsg{id: id, name: name, err: err}
		}
		if err := conn.Ping(ctx); err != nil {
			conn.Close()
			return connOpenedMsg{id: id, name: name, err: err}
		}
		return connOpenedMsg{id: id, name: name, conn: conn}
	}
}

func (m *Model) handleConnOpened(msg connOpenedMsg) tea.Cmd {
	if msg.id != m.connectSeq {
		if msg.conn != nil {
			msg.conn.Close()
		}
		return nil
	}
	if msg.err != nil {
		m.setStatus(m.t.T(i18n.KeyErrorConnect, msg.err), true)
		return nil
	}
	if m.conn != nil && m.connName == msg.name {
		m.conn.Close()
	}
	m.conn = msg.conn
	m.connName = msg.name
	m.tabConnName = msg.name
	m.restoreTabsFor(msg.name)
	m.syncSidebarSessions()
	m.executor = query.Executor{
		Conn:        m.conn,
		MaxRows:     m.cfg.Query.MaxRows,
		AlwaysQuery: m.conn.Type() == "redis" || m.conn.Type() == "mongo",
	}
	m.editCache = map[string]editInfo{}
	m.editTarget = editTarget{}
	m.pager = nil
	m.setStatus(m.t.T(i18n.KeyStatusConnected, msg.name), false)
	m.completer.Reset()
	m.tableSchemas = map[string]string{}
	m.completion.Close()

	cmds := []tea.Cmd{m.sidebar.SetConn(m.conn), m.loadCompletionTablesCmd(), m.scheduleTabSave()}
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
	return m.runStatement(stmt, true, 0)
}

func (m *Model) runTableCommand(stmt string) tea.Cmd {
	return m.runStatement(stmt, false, max(1, m.cfg.Query.MaxRows))
}

// runStatement uses a zero maxRows for an unlimited editor query. Table
// browsing passes query.max_rows explicitly.
func (m *Model) runStatement(stmt string, recordHistory bool, maxRows int) tea.Cmd {
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	if m.readOnly() && query.IsWriteFor(m.conn.Type(), stmt) {
		m.setStatus(m.t.T(i18n.KeyErrorReadOnly), true)
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
	var store *history.Store
	if recordHistory {
		m.editor.AddHistory(stmt)
		store = m.history
		if store != nil {
			store.Add(stmt)
		}
	}
	m.setStatus(m.t.T(i18n.KeyStatusRunning), false)

	executor := m.executor
	executor.MaxRows = maxRows
	return func() tea.Msg {
		outcome := executor.Run(ctx, stmt)
		cancel()
		if store != nil {
			_ = store.Save()
		}
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

func (m *Model) handleQueryDone(msg queryDoneMsg) tea.Cmd {
	if msg.id != m.querySeq {
		return nil
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
			m.pager = nil
			m.setStatus(m.t.T(i18n.KeyErrorQuery, out.Err), true)
			m.results.SetError(out.Err.Error())
		}
		return nil
	}

	switch {
	case out.Result != nil:
		m.pager = nil
		m.results.SetResult(out.Result)
		duration := humanDuration(out.Result.Elapsed)
		if out.Result.Truncated {
			m.setStatus(m.t.T(i18n.KeyStatusRowsTruncated, len(out.Result.Rows), duration), false)
		} else {
			m.setStatus(m.t.T(i18n.KeyStatusRows, len(out.Result.Rows), duration), false)
		}
	case out.Exec != nil:
		m.pager = nil
		m.results.SetExec(out.Exec.Affected, out.Exec.Elapsed)
		m.setStatus(m.t.T(i18n.KeyStatusAffected, out.Exec.Affected, humanDuration(out.Exec.Elapsed)), false)
	}
	return nil
}

func (m *Model) handleTablePageDone(msg tablePageDoneMsg) tea.Cmd {
	if msg.id != m.querySeq || msg.page == nil {
		return nil
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
		}
		return nil
	}
	if out.Result == nil {
		return nil
	}

	page := msg.page
	if page.offset > 0 && len(out.Result.Rows) == 0 {
		if page.refresh {
			previous := *page
			previous.offset -= previous.size
			if previous.offset < 0 {
				previous.offset = 0
			}
			previous.refresh = true
			return m.runPageQuery(&previous)
		}
		if m.pager != nil {
			m.pager.atEnd = true
		}
		m.setStatus(m.t.T(i18n.KeyStatusNoMoreRows), false)
		return nil
	}

	m.results.SetResult(out.Result)
	if page.sortColumn != "" {
		m.results.SetSort(m.results.ColumnIndex(page.sortColumn), page.sortAsc)
	} else {
		m.results.SetSort(-1, true)
	}
	m.results.SetSource(page.schema, page.table)
	page.atEnd = len(out.Result.Rows) < page.size
	page.refresh = false
	m.pager = page
	size := max(1, page.size)
	pageNumber := page.offset/size + 1
	m.setStatus(m.t.T(i18n.KeyStatusPage, pageNumber, len(out.Result.Rows), humanDuration(out.Result.Elapsed)), false)
	return nil
}

func (m *Model) openTable(msg sidebar.SelectTableMsg) tea.Cmd {
	if m.conn != nil && m.conn.Type() == "redis" {
		m.editor.SetValue(redisCommand(msg.Kind, msg.Table))
		m.focus = focusEditor
		m.layout()
		return tea.Batch(m.editor.Focus(), m.scheduleTabSave())
	}
	if m.conn != nil && m.conn.Type() == "mongo" {
		m.editor.SetValue(mongoCommand(msg.Schema, msg.Table))
		m.focus = focusEditor
		m.layout()
		return tea.Batch(m.editor.Focus(), m.scheduleTabSave())
	}
	m.editor.SetValue(m.selectTableStmt(msg.Schema, msg.Table))
	m.focus = focusEditor
	m.layout()
	return tea.Batch(m.editor.Focus(), m.scheduleTabSave())
}

func mongoCommand(schema, table string) string {
	return "find " + schema + "." + table + " {}"
}

func (m *Model) mongoTableData(schema, table string) tea.Cmd {
	m.focus = focusResults
	m.layout()
	return m.runTableCommand(mongoCommand(schema, table))
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
	m.focus = focusResults
	m.layout()
	return m.runTableCommand(redisCommand(kind, table))
}

func qualifiedTable(schema, table string) string {
	if schema != "" && schema != "main" {
		return schema + "." + table
	}
	return table
}

func (m *Model) selectTableStmt(schema, table string) string {
	qualified := qualifiedTable(schema, table)
	if m.cfg.Query.MaxRows > 0 {
		return fmt.Sprintf(m.t.T(i18n.KeyQuerySelectTable), qualified, m.cfg.Query.MaxRows)
	}
	return fmt.Sprintf("SELECT * FROM %s;", qualified)
}

func (m *Model) tableData(schema, table string) tea.Cmd {
	m.focus = focusResults
	m.layout()
	return m.runPageQuery(&pagination{
		schema: schema,
		table:  table,
		size:   max(1, m.cfg.Query.MaxRows),
	})
}

func (m *Model) openTableData(schema, table, kind string) tea.Cmd {
	if m.conn != nil && m.conn.Type() == "redis" {
		return m.redisTableData(schema, table, kind)
	}
	if m.conn != nil && m.conn.Type() == "mongo" {
		return m.mongoTableData(schema, table)
	}
	return m.tableData(schema, table)
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
	if msg.ID == "table.data" {
		return m.openTableData(schema, table, m.tableMenuKind)
	}
	if m.conn != nil && m.conn.Type() == "redis" {
		switch msg.ID {
		case "table.select":
			return m.openTable(sidebar.SelectTableMsg{Schema: schema, Table: table, Kind: m.tableMenuKind})
		}
		m.setStatus(m.t.T(i18n.KeyErrorUnsupportedFeature, "redis"), true)
		return nil
	}
	if m.conn != nil && m.conn.Type() == "mongo" {
		switch msg.ID {
		case "table.select":
			return m.openTable(sidebar.SelectTableMsg{Schema: schema, Table: table, Kind: m.tableMenuKind})
		}
	}
	switch msg.ID {
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
	id := m.connectSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ddl, err := conn.Catalog().DDL(ctx, schema, table)
		return ddlLoadedMsg{id: id, title: title, text: ddl, err: err}
	}
}

func (m *Model) openIndexes(schema, table string) tea.Cmd {
	conn := m.conn
	if conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	title := fmt.Sprintf(m.t.T(i18n.KeyTableIndexesTitle), qualifiedTable(schema, table))
	id := m.connectSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		indexes, err := conn.Catalog().Indexes(ctx, schema, table)
		if err != nil {
			return indexesLoadedMsg{id: id, title: title, err: err}
		}
		return indexesLoadedMsg{id: id, title: title, text: formatIndexes(indexes)}
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
	rs := m.results.OrderedResult()
	if rs == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoResults), true)
		return nil
	}
	name := fmt.Sprintf("bdb-export-%s.%s", time.Now().Format("20060102-150405.000000000"), kind)
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
			_ = os.Remove(path)
			return exportedMsg{err: err}
		}
		return exportedMsg{path: path}
	}
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

func (m *Model) syncSidebarSessions() {
	names := make([]string, 0, len(m.sessions)+1)
	for name := range m.sessions {
		names = append(names, name)
	}
	if m.connName != "" {
		names = append(names, m.connName)
	}
	m.sidebar.SetActiveSessions(names)
}

func (m *Model) disconnect() {
	m.connectSeq++
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.cancelMutation()
	m.running = false
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	if m.connName != "" {
		m.rememberCurrentTabs()
		delete(m.sessions, m.connName)
	}
	m.connName = ""
	m.completer.Reset()
	m.tableSchemas = map[string]string{}
	m.editCache = map[string]editInfo{}
	m.pager = nil
	m.sidebar.ClearConn()
	m.results.Clear()
	_ = m.saveTabStatesNow()
	m.syncSidebarSessions()
	m.setStatus(m.t.T(i18n.KeyStatusDisconnected), false)
}

func (m *Model) switchToConnection(name string) tea.Cmd {
	if name == m.connName {
		return nil
	}
	if saved, ok := m.sessions[name]; ok {
		m.saveCurrentSession()
		m.activateSession(name, saved)
		return m.sidebar.SetConn(saved.conn)
	}
	m.saveCurrentSession()
	m.resetSession()
	return m.connectCmd(name)
}

func (m *Model) saveCurrentSession() {
	if m.conn == nil || m.connName == "" {
		return
	}
	m.connectSeq++
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.cancelMutation()
	m.running = false
	m.querySeq++
	m.syncActiveTab()
	tabs := append([]editorTab(nil), m.tabs...)
	m.sessions[m.connName] = &session{
		conn:         m.conn,
		connName:     m.connName,
		results:      m.results,
		completer:    m.completer,
		tableSchemas: m.tableSchemas,
		editCache:    m.editCache,
		pager:        m.pager,
		tabs:         tabs,
		activeTab:    m.activeTab,
	}
	_ = m.saveTabStatesNow()
	m.syncSidebarSessions()
}

func (m *Model) activateSession(name string, saved *session) {
	m.conn = saved.conn
	m.connName = name
	m.tabConnName = name
	m.results = saved.results
	m.completer = saved.completer
	m.tableSchemas = saved.tableSchemas
	m.editCache = saved.editCache
	m.pager = saved.pager
	if len(saved.tabs) > 0 {
		m.tabs = append([]editorTab(nil), saved.tabs...)
		m.activeTab = saved.activeTab
		if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
			m.activeTab = 0
		}
		m.editor = m.tabs[m.activeTab].editor
		if m.history != nil {
			m.editor.SetHistory(m.history.Entries())
		}
	} else {
		m.resetTabs()
	}
	m.executor = query.Executor{
		Conn:        saved.conn,
		MaxRows:     m.cfg.Query.MaxRows,
		AlwaysQuery: saved.conn.Type() == "redis" || saved.conn.Type() == "mongo",
	}
	m.pendingCmd = nil
	m.running = false
	m.querySeq++
	delete(m.sessions, name)
	m.closeOverlays()
	m.layout()
	m.syncSidebarSessions()
	m.setStatus(m.t.T(i18n.KeyStatusConnected, name), false)
}

func (m *Model) resetSession() {
	m.conn = nil
	m.connName = ""
	m.running = false
	m.pendingCmd = nil
	m.tabConnName = ""
	m.resetTabs()
	m.results = results.New(m.t)
	m.completer = ac.New()
	m.tableSchemas = map[string]string{}
	m.editCache = map[string]editInfo{}
	m.pager = nil
	m.closeOverlays()
	m.sidebar.ClearConn()
	m.layout()
	m.syncSidebarSessions()
}

func (m *Model) closeOverlays() {
	m.options.Close()
	m.tableMenu.Close()
	m.textView.Close()
	m.detail.Close()
	m.connForm.Close()
	m.cellEdit.Close()
	m.confirm.Close()
	m.completion.Close()
	m.showHelp = false
	m.pendingCmd = nil
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

func (m *Model) openDetail(schema, table string) tea.Cmd {
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	m.detail.Open(schema, table)
	conn := m.conn
	title := m.detail.Title()
	id := m.connectSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		columns, err := conn.Catalog().Columns(ctx, schema, table)
		if err != nil {
			return detailLoadedMsg{id: id, title: title, err: err}
		}
		indexes, err := conn.Catalog().Indexes(ctx, schema, table)
		if err != nil {
			return detailLoadedMsg{id: id, title: title, err: err}
		}
		fks, err := conn.Catalog().ForeignKeys(ctx, schema, table)
		if err != nil {
			return detailLoadedMsg{id: id, title: title, err: err}
		}
		return detailLoadedMsg{id: id, title: title, columns: columns, indexes: indexes, fks: fks}
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
	m.querySeq++
	m.pager = nil
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	m.cancel = cancel
	m.running = true
	m.setStatus(m.t.T(i18n.KeyStatusRunning), false)
	executor := m.executor
	readOnly := m.readOnly()
	readOnlyErr := errors.New(m.t.T(i18n.KeyErrorReadOnly))
	dialect := m.conn.Type()
	id := m.connectSeq

	return func() tea.Msg {
		start := time.Now()
		data, err := os.ReadFile(path)
		if err != nil {
			cancel()
			return scriptDoneMsg{id: id, err: err, elapsed: time.Since(start)}
		}
		statements, err := sqlscript.SplitFor(string(data), dialect)
		if err != nil {
			cancel()
			return scriptDoneMsg{id: id, err: err, elapsed: time.Since(start)}
		}
		msg := scriptDoneMsg{id: id, total: len(statements)}
		for i, stmt := range statements {
			if readOnly && query.IsWriteFor(dialect, stmt) {
				msg.err = readOnlyErr
				msg.failedAt = i + 1
				break
			}
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
	if msg.id != m.connectSeq {
		return
	}
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
	id := m.connectSeq
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
			return completionColumnsMsg{id: id, table: qualifier, err: err}
		}
		names := make([]string, 0, len(columns))
		for _, column := range columns {
			names = append(names, column.Name)
		}
		return completionColumnsMsg{id: id, table: qualifier, columns: names}
	}
}

func (m *Model) loadCompletionTablesCmd() tea.Cmd {
	conn := m.conn
	if conn == nil {
		return nil
	}
	id := m.connectSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		schemas, err := conn.Catalog().Schemas(ctx)
		if err != nil {
			return completionTablesMsg{id: id, err: err}
		}
		seen := map[string]bool{}
		var refs []tableRef
		for _, schema := range schemas {
			objects, err := conn.Catalog().Tables(ctx, schema)
			if err != nil {
				return completionTablesMsg{id: id, err: err}
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
		return completionTablesMsg{id: id, tables: refs}
	}
}

func (m *Model) shutdown() {
	m.connectSeq++
	_ = m.saveTabStatesNow()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.cancelMutation()
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	for name, saved := range m.sessions {
		if saved.conn != nil {
			saved.conn.Close()
		}
		delete(m.sessions, name)
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
	resolved, err := config.ExpandConnection(conn)
	if err != nil {
		m.connForm.SetError(err.Error())
		return nil
	}
	if m.cfg.Find(resolved.Name) != nil {
		m.connForm.SetError(m.t.T(i18n.KeyFormDuplicate, resolved.Name))
		return nil
	}
	if err := config.AppendRuntimeConnection(conn); err != nil {
		m.connForm.SetError(m.t.T(i18n.KeyErrorSaveConnection, err))
		return nil
	}
	m.cfg.Connections = append(m.cfg.Connections, resolved)
	m.sidebar.AddConnection(resolved)
	m.setStatus(m.t.T(i18n.KeyStatusConnectionAdded, resolved.Name), false)
	if m.conn != nil {
		return m.switchToConnection(resolved.Name)
	}
	return m.connectCmd(resolved.Name)
}

func (m *Model) setStatus(text string, isError bool) {
	m.status = text
	m.statusErr = isError
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

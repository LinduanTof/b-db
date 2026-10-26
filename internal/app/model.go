package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	ac "github.com/LinduanTof/dbrix/internal/complete"
	"github.com/LinduanTof/dbrix/internal/config"
	"github.com/LinduanTof/dbrix/internal/db"
	_ "github.com/LinduanTof/dbrix/internal/db/mongo"
	_ "github.com/LinduanTof/dbrix/internal/db/redis"
	_ "github.com/LinduanTof/dbrix/internal/db/sql"
	"github.com/LinduanTof/dbrix/internal/export"
	"github.com/LinduanTof/dbrix/internal/history"
	"github.com/LinduanTof/dbrix/internal/i18n"
	"github.com/LinduanTof/dbrix/internal/query"
	"github.com/LinduanTof/dbrix/internal/sqlscript"
	"github.com/LinduanTof/dbrix/internal/ui/celledit"
	uicomplete "github.com/LinduanTof/dbrix/internal/ui/complete"
	"github.com/LinduanTof/dbrix/internal/ui/confirm"
	"github.com/LinduanTof/dbrix/internal/ui/connection"
	"github.com/LinduanTof/dbrix/internal/ui/detail"
	"github.com/LinduanTof/dbrix/internal/ui/editor"
	"github.com/LinduanTof/dbrix/internal/ui/form"
	"github.com/LinduanTof/dbrix/internal/ui/menu"
	"github.com/LinduanTof/dbrix/internal/ui/pathinput"
	"github.com/LinduanTof/dbrix/internal/ui/results"
	"github.com/LinduanTof/dbrix/internal/ui/sidebar"
	"github.com/LinduanTof/dbrix/internal/ui/statusbar"
	"github.com/LinduanTof/dbrix/internal/ui/textview"
	"github.com/LinduanTof/dbrix/internal/ui/theme"
)

type focus int

const (
	focusSidebar focus = iota
	focusEditor
	focusResults
)

type contextKind int

const (
	contextQuery contextKind = iota
	contextInfo
	contextTables
	contextData
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

type databasesLoadedMsg struct {
	id        uint64
	base      string
	databases []string
	err       error
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
	table    *db.TableRef
	err      error
}

type exportedMsg struct {
	path string
	err  error
}

type exportProgressMsg struct {
	label string
	done  int
	total int
	rows  int64
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

type overviewLoadedMsg struct {
	id   uint64
	rows [][]any
	err  error
}

type versionLoadedMsg struct {
	id      uint64
	version string
	err     error
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
	dirInput   pathinput.Model
	form       form.Model

	formTarget  db.TableRef
	formColumns []db.Column

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

	menuNode sidebar.NodeMenuMsg

	completer    *ac.Engine
	tableSchemas map[string]string

	history       *history.Store
	pendingScript string
	themeName     string

	width, height                    int
	sideW, rightW, editorH, resultsH int
	editorW, resultsW                int
	columnsActive                    bool

	focus focus

	focusMode  string
	layoutMode string
	zoom       bool
	autoPaused bool

	conn     db.Conn
	connName string
	sessions map[string]*session
	executor query.Executor

	querySeq       uint64
	connectSeq     uint64
	overviewSeq    uint64
	cancel         context.CancelFunc
	mutationCancel context.CancelFunc
	connectCancel  context.CancelFunc
	quitting       atomic.Bool
	running        bool

	status    string
	statusErr bool
	showHelp  bool
	context   contextKind
	outDir    string
	send      func(tea.Msg)
	exporting bool
	importing bool

	pendingImport      *importPlan
	pendingCreateTable string

	infoName    string
	infoStatus  string
	infoVersion string
	versionSeq  uint64

	overviewAll    [][]any
	overviewFilter string
	filterInput    textinput.Model
	filtering      bool
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
	context      contextKind
	overviewAll  [][]any
	filter       string
	cursor       string
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
	m.layoutMode = config.NormalizeLayout(cfg.UI.Layout)
	if saved := config.LoadLayoutState(); saved != "" {
		m.layoutMode = config.NormalizeLayout(saved)
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
	m.dirInput = pathinput.New(t)
	m.form = form.New(t)
	m.editCache = map[string]editInfo{}
	m.completer = ac.New()
	m.tableSchemas = map[string]string{}
	m.sessions = map[string]*session{}
	m.filterInput = textinput.New()
	m.filterInput.Prompt = ""
	m.filterInput.CharLimit = 64
	if saved, ok := config.LoadExportDirState(); ok {
		m.outDir = saved
	} else {
		m.outDir = cfg.UI.ExportDir
	}
	if m.outDir == "" {
		if wd, err := os.Getwd(); err == nil {
			m.outDir = wd
		}
	}
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

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
		m.dirInput.SetSize(msg.Width, msg.Height)
		m.form.SetSize(msg.Width, msg.Height)
		m.layout()
		return m, nil
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	case tea.MouseClickMsg:
		return m, m.handleMouseClick(msg)
	case tea.MouseWheelMsg:
		return m, m.handleMouseWheel(msg)
	case menu.ActionMsg:
		switch {
		case strings.HasPrefix(msg.ID, "table."):
			return m, m.runTableMenuAction(msg)
		case strings.HasPrefix(msg.ID, "conn."), strings.HasPrefix(msg.ID, "db."):
			return m, m.runNodeMenuAction(msg)
		default:
			return m, m.runMenuAction(msg)
		}
	case menu.ChangedMsg:
		m.applyMenuChange(msg)
		return m, nil
	case sidebar.NodeMenuMsg:
		m.menuNode = msg
		m.tableMenu.Open(m.nodeMenuTitle(msg), m.nodeMenuItems(msg))
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
	case databasesLoadedMsg:
		return m, m.handleDatabasesLoaded(msg)
	case celledit.SubmittedMsg:
		return m, m.openUpdateConfirm(msg.Value)
	case form.SubmittedMsg:
		return m, m.handleFormSubmit(msg)
	case pathinput.SubmittedMsg:
		m.applyExportDir(msg.Value)
		return m, nil
	case confirm.ConfirmedMsg:
		if m.pendingImport != nil {
			m.pendingCmd = m.importCmd(*m.pendingImport)
			m.pendingImport = nil
		}
		cmd := m.pendingCmd
		m.pendingCmd = nil
		m.confirm.Close()
		return m, cmd
	case editInfoMsg:
		return m, m.handleEditInfo(msg)
	case mutationDoneMsg:
		return m, m.handleMutationDone(msg)
	case exportedMsg:
		m.exporting = false
		if msg.err != nil {
			m.setStatus(m.t.T(i18n.KeyErrorExport, msg.err), true)
		} else {
			m.setStatus(m.t.T(i18n.KeyStatusExported, msg.path), false)
		}
		return m, nil
	case schemaDoneMsg:
		return m, m.handleSchemaDone(msg)
	case indexesMenuMsg:
		return m, m.handleIndexesMenu(msg)
	case importPreviewMsg:
		return m, m.handleImportPreview(msg)
	case importProgressMsg:
		m.setStatus(m.t.T(i18n.KeyStatusImportingRows, msg.label, fmt.Sprint(msg.rows)), false)
		return m, nil
	case importedMsg:
		m.importing = false
		m.cancelMutation()
		if msg.id != m.connectSeq {
			return m, nil
		}
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				m.setStatus(m.t.T(i18n.KeyStatusCancelled), false)
				return m, nil
			}
			m.setStatus(m.t.T(i18n.KeyErrorImport, msg.err), true)
			return m, nil
		}
		m.setStatus(m.t.T(i18n.KeyStatusImported, msg.rows), false)
		if m.pager != nil && m.pager.schema == msg.table.Schema && m.pager.table == msg.table.Name {
			return m, m.refreshSource()
		}
		return m, nil
	case exportProgressMsg:
		m.exporting = true
		switch {
		case msg.rows > 0:
			m.setStatus(m.t.T(i18n.KeyStatusExportingRows, msg.label, msg.rows), false)
		case msg.total > 0:
			m.setStatus(m.t.T(i18n.KeyStatusExportingCount, msg.label, msg.done, msg.total), false)
		default:
			m.setStatus(m.t.T(i18n.KeyStatusExporting, msg.label), false)
		}
		return m, nil
	case sidebar.ConnectMsg:
		return m, m.switchToConnection(msg.Name)
	case sidebar.DatabaseMsg:
		if msg.Session == m.connName {
			return m, m.loadOverviewCmd()
		}
		return m, m.switchToConnection(msg.Session)
	case sidebar.TableActivateMsg:
		return m, m.activateTable(msg.Schema, msg.Table, msg.Kind)
	case sidebar.EditorOpenMsg:
		return m, m.openTableTab(msg.Schema, msg.Table, msg.Kind)
	case sidebar.TableOpenMsg:
		return m, m.openTableData(msg.Schema, msg.Table, msg.Kind)
	case sidebar.ConnectionInfoMsg:
		return m, m.showConnectionInfo(msg.Name, m.t.T(i18n.KeyStatusConnected))
	case overviewLoadedMsg:
		return m, m.handleOverviewLoaded(msg)
	case versionLoadedMsg:
		m.handleVersionLoaded(msg)
		return m, nil
	case sidebar.SchemasLoadedMsg, sidebar.TablesLoadedMsg, sidebar.ColumnsLoadedMsg:
		if err := sidebar.MsgError(msg); err != nil {
			m.setStatus(m.t.T(i18n.KeyErrorCatalog, err), true)
		}
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		return m, cmd
	case tea.PasteMsg:
		return m, m.handlePaste(msg)
	}

	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "dbrix"
	// Lets terminals that support the Kitty keyboard protocol distinguish
	// ctrl+enter from plain enter.
	v.KeyboardEnhancements = tea.KeyboardEnhancements{
		ReportAlternateKeys:        true,
		ReportAllKeysAsEscapeCodes: true,
		ReportAssociatedText:       true,
	}
	return v
}

type paneKind int

const (
	paneNone paneKind = iota
	paneSidebar
	paneEditor
	paneResults
)

// paneAt maps a screen coordinate to the visible pane and returns the x/y
// relative to that pane. Zoom, columns and results-top layouts are resolved
// here so click and wheel handling always agree.
func (m *Model) paneAt(x, y int) (paneKind, int, int) {
	if y < 0 || y >= m.height-2 {
		return paneNone, 0, 0
	}
	if m.effectiveZoom() {
		switch m.focus {
		case focusEditor:
			return paneEditor, x, y
		case focusResults:
			return paneResults, x, y
		default:
			return paneSidebar, x, y
		}
	}
	if x < m.sideW {
		return paneSidebar, x, y
	}
	editorStart := m.sideW + 1
	if x < editorStart || x >= editorStart+m.rightW {
		return paneNone, 0, 0
	}
	relX := x - editorStart
	if m.columnsActive {
		switch {
		case relX < m.editorW:
			return paneEditor, relX, y
		case relX >= m.editorW+1:
			return paneResults, relX - m.editorW - 1, y
		}
		return paneNone, 0, 0
	}
	if m.layoutMode == config.LayoutResultsTop {
		switch {
		case y < m.resultsH:
			return paneResults, relX, y
		case y >= m.resultsH+1 && y < m.resultsH+1+m.editorH:
			return paneEditor, relX, y - m.resultsH - 1
		}
		return paneNone, 0, 0
	}
	switch {
	case y < m.editorH:
		return paneEditor, relX, y
	case y >= m.editorH+1 && y < m.editorH+1+m.resultsH:
		return paneResults, relX, y - m.editorH - 1
	}
	return paneNone, 0, 0
}

func (m *Model) handleMouseClick(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Button != tea.MouseLeft {
		return nil
	}
	// Clicking elsewhere ends filter editing but keeps the applied filter,
	// so keys go to the newly focused pane instead of the filter input.
	if m.filtering {
		m.filtering = false
		m.filterInput.Blur()
	}
	if m.options.IsOpen() {
		var cmd tea.Cmd
		m.options, cmd = m.options.HandleClick(msg.X, msg.Y, m.width, m.height)
		return cmd
	}
	if m.tableMenu.IsOpen() {
		var cmd tea.Cmd
		m.tableMenu, cmd = m.tableMenu.HandleClick(msg.X, msg.Y, m.width, m.height)
		return cmd
	}
	if m.anyOverlayOpen() {
		return nil
	}

	kind, relX, paneY := m.paneAt(msg.X, msg.Y)
	switch kind {
	case paneSidebar:
		m.editor.Blur()
		m.focus = focusSidebar
		m.layout()
		if paneY == 0 {
			return nil
		}
		return m.sidebar.HandleClick(paneY - 1)
	case paneEditor:
		m.focus = focusEditor
		m.layout()
		// The editor body starts below the pane title and the tab bar.
		if paneY >= 2 {
			m.editor.HandleClick(relX, paneY-2)
		}
		return m.editor.Focus()
	case paneResults:
		m.editor.Blur()
		m.focus = focusResults
		m.layout()
		if paneY >= 1 {
			m.results.HandleClick(relX, paneY-1)
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

	if m.options.IsOpen() {
		m.options.HandleWheel(delta)
		return nil
	}
	if m.tableMenu.IsOpen() {
		m.tableMenu.HandleWheel(delta)
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

	kind, _, _ := m.paneAt(msg.X, msg.Y)
	switch kind {
	case paneSidebar:
		m.sidebar.Scroll(delta)
	case paneEditor:
		m.editor.Scroll(delta)
	case paneResults:
		m.results.Scroll(delta)
	}
	return nil
}

// handlePaste routes pasted text to the topmost widget that accepts input.
// Without this, a paste while a modal is open silently edits the SQL editor
// behind it.
func (m *Model) handlePaste(msg tea.PasteMsg) tea.Cmd {
	switch {
	case m.form.IsOpen():
		return m.form.Paste(msg)
	case m.connForm.IsOpen():
		return m.connForm.Paste(msg)
	case m.cellEdit.IsOpen():
		return m.cellEdit.Paste(msg)
	case m.dirInput.IsOpen():
		return m.dirInput.Paste(msg)
	case m.focus == focusEditor && m.editor.Finding():
		m.editor.PasteFind(msg.Content)
		return nil
	}
	if m.options.IsOpen() || m.tableMenu.IsOpen() || m.textView.IsOpen() ||
		m.detail.IsOpen() || m.confirm.IsOpen() || m.completion.IsOpen() || m.showHelp {
		return nil
	}
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	if m.focus == focusEditor {
		return tea.Batch(cmd, m.scheduleTabSave())
	}
	return cmd
}

func (m *Model) anyOverlayOpen() bool {
	return m.options.IsOpen() || m.tableMenu.IsOpen() || m.textView.IsOpen() ||
		m.detail.IsOpen() || m.connForm.IsOpen() || m.cellEdit.IsOpen() ||
		m.confirm.IsOpen() || m.completion.IsOpen() || m.dirInput.IsOpen() ||
		m.form.IsOpen() || m.showHelp
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
	if m.dirInput.IsOpen() {
		var cmd tea.Cmd
		m.dirInput, cmd = m.dirInput.Update(msg)
		return cmd
	}
	if m.form.IsOpen() {
		var cmd tea.Cmd
		m.form, cmd = m.form.Update(msg)
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
			m.pendingImport = nil
			m.cancelMutation()
		}
		return cmd
	}
	if m.filtering {
		switch key {
		case "esc":
			m.filtering = false
			m.filterInput.SetValue("")
			m.filterInput.Blur()
			if m.context == contextTables {
				m.overviewFilter = ""
				m.rebuildOverview()
			} else {
				m.results.SetFilter("")
			}
			return nil
		case "enter":
			m.filtering = false
			m.filterInput.Blur()
			return nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		value := m.filterInput.Value()
		if m.context == contextTables {
			if value != m.overviewFilter {
				m.overviewFilter = value
				m.rebuildOverview()
			}
		} else if value != m.results.Filter() {
			m.results.SetFilter(value)
		}
		return cmd
	}
	if m.focus == focusEditor && m.editor.Finding() {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.UpdateFind(msg)
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
		if m.running || m.importing {
			m.cancelQuery()
			m.cancelMutation()
			return nil
		}
		m.completion.Close()
		m.options.OpenTabs("", []string{
			m.t.T(i18n.KeyMenuTabActions),
			m.t.T(i18n.KeyMenuTabSettings),
		}, m.menuItems())
		return nil
	}
	if key == "f2" {
		m.toggleZoom()
		return nil
	}
	if key == "ctrl+shift+w" || key == "alt+w" {
		if m.connName == "" {
			m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
			return nil
		}
		m.disconnect()
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
		case "ctrl+f":
			m.editor.OpenFind()
			return nil
		case "alt+e":
			return m.explainQuery()
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
			if m.context == contextTables {
				return m.openOverviewTable()
			}
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
		case "r":
			if m.context == contextTables {
				return m.loadOverviewCmd()
			}
		case "/":
			if m.context == contextTables {
				m.filtering = true
				m.filterInput.SetValue(m.overviewFilter)
				m.filterInput.Focus()
				return nil
			}
			if m.context == contextData || m.context == contextQuery {
				m.filtering = true
				m.filterInput.SetValue(m.results.Filter())
				m.filterInput.Focus()
				return nil
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
	if m.connectCancel != nil {
		m.connectCancel()
		m.connectCancel = nil
	}
	connCfg := m.cfg.Find(name)
	if connCfg == nil {
		return func() tea.Msg {
			return connOpenedMsg{id: id, name: name, err: fmt.Errorf("connection %q not found", name)}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	m.connectCancel = cancel
	m.setStatus(m.t.T(i18n.KeyStatusConnecting, name), false)
	return func() tea.Msg {
		defer cancel()
		conn, err := db.Open(ctx, toDBConfig(connCfg))
		if err != nil {
			return connOpenedMsg{id: id, name: name, err: err}
		}
		if err := conn.Ping(ctx); err != nil {
			conn.Close()
			return connOpenedMsg{id: id, name: name, err: err}
		}
		// The program may have exited while the connection was being
		// established; nobody would receive the message, so close it here.
		if m.quitting.Load() {
			conn.Close()
			return connOpenedMsg{id: id, name: name, err: context.Canceled}
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
		m.pendingCreateTable = ""
		m.setStatus(m.t.T(i18n.KeyErrorConnect, msg.err), true)
		return m.showConnectionInfo(msg.name, msg.err.Error())
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
	m.setStatus(m.t.T(i18n.KeyStatusReady), false)
	m.completer.Reset()
	m.tableSchemas = map[string]string{}
	m.completion.Close()

	cmds := []tea.Cmd{m.sidebarConnCmd(m.conn, msg.name), m.loadCompletionTablesCmd(), m.scheduleTabSave()}
	if sidebar.SupportsDatabases(m.conn.Type()) {
		cmds = append(cmds, m.loadDatabasesCmd(sessionBase(msg.name), m.conn))
	}
	if sessionDatabase(msg.name) != "" && sidebar.SupportsDatabases(m.conn.Type()) {
		cmds = append(cmds, m.loadOverviewCmd())
	} else {
		cmds = append(cmds, m.showConnectionInfo(msg.name, m.t.T(i18n.KeyStatusConnected)))
	}
	if m.pendingScript != "" {
		path := m.pendingScript
		m.pendingScript = ""
		cmds = append(cmds, m.runScriptCmd(path))
	}
	if pending := m.pendingCreateTable; pending != "" {
		m.pendingCreateTable = ""
		if pending == msg.name {
			cmds = append(cmds, m.openCreateTableForm(pending))
		}
	}
	return tea.Batch(cmds...)
}

func sessionBase(name string) string {
	if index := strings.Index(name, "/"); index >= 0 {
		return name[:index]
	}
	return name
}

func (m *Model) sidebarConnCmd(conn db.Conn, session string) tea.Cmd {
	if !sidebar.SupportsDatabases(conn.Type()) {
		return m.sidebar.SetConn(conn)
	}
	base := sessionBase(session)
	database := ""
	if connection := m.cfg.Find(session); connection != nil {
		database = connection.Database
	} else if index := strings.Index(session, "/"); index >= 0 {
		database = session[index+1:]
	}
	if database == "" {
		return m.sidebar.SetDatabaseRoot(conn, base)
	}
	return m.sidebar.SetDatabaseConn(conn, base, database, session)
}

func (m *Model) loadDatabasesCmd(base string, conn db.Conn) tea.Cmd {
	id := m.connectSeq
	return func() tea.Msg {
		lister, ok := conn.Catalog().(db.DatabaseLister)
		if !ok {
			return databasesLoadedMsg{id: id, base: base}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		names, err := lister.Databases(ctx)
		return databasesLoadedMsg{id: id, base: base, databases: names, err: err}
	}
}

func (m *Model) handleDatabasesLoaded(msg databasesLoadedMsg) tea.Cmd {
	if msg.id != m.connectSeq {
		return nil
	}
	base := m.cfg.Find(msg.base)
	if base == nil {
		return nil
	}
	if msg.err != nil {
		m.setStatus(m.t.T(i18n.KeyErrorCatalog, msg.err), true)
		return nil
	}

	entries := make([]sidebar.DatabaseEntry, 0, len(msg.databases)+1)
	seen := make(map[string]bool, len(msg.databases)+1)
	add := func(database, session string) {
		if database == "" || session == "" || seen[database] {
			return
		}
		seen[database] = true
		entries = append(entries, sidebar.DatabaseEntry{Name: database, Session: session})
	}
	for _, database := range msg.databases {
		if database == "" {
			continue
		}
		if database == base.Database {
			add(database, msg.base)
			continue
		}
		logical := msg.base + "/" + database
		clone := *base
		clone.Name = logical
		clone.Database = database
		if existing := m.cfg.Find(logical); existing != nil {
			*existing = clone
		} else {
			m.cfg.Connections = append(m.cfg.Connections, clone)
		}
		add(database, logical)
	}
	if base.Database != "" {
		add(base.Database, msg.base)
	}
	if current := m.cfg.Find(m.connName); current != nil && current.Database != "" {
		add(current.Database, m.connName)
	}
	m.sidebar.SetDatabases(msg.base, entries)
	m.syncSidebarSessions()
	return nil
}

func (m *Model) runQuery() tea.Cmd {
	stmt := strings.TrimSpace(m.editor.Value())
	if stmt == "" {
		m.setStatus(m.t.T(i18n.KeyStatusEmptyQuery), false)
		return nil
	}
	m.setContext(contextQuery)
	return m.runStatement(stmt, true, 0)
}

// explainQuery runs the first statement in the editor through the dialect's
// EXPLAIN form. It never uses EXPLAIN ANALYZE, so the statement is not executed.
func (m *Model) explainQuery() tea.Cmd {
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	stmt := strings.TrimSpace(m.editor.Value())
	if stmt == "" {
		m.setStatus(m.t.T(i18n.KeyStatusEmptyQuery), false)
		return nil
	}
	typ := m.conn.Type()
	var prefix string
	switch typ {
	case config.TypePostgres:
		prefix = "EXPLAIN (FORMAT TEXT) "
	case config.TypeMySQL:
		prefix = "EXPLAIN "
	case config.TypeSQLite:
		prefix = "EXPLAIN QUERY PLAN "
	default:
		m.setStatus(m.t.T(i18n.KeyErrorExplain, typ), true)
		return nil
	}
	stmts, err := sqlscript.SplitFor(stmt, typ)
	if err != nil {
		m.setStatus(m.t.T(i18n.KeyErrorQuery, err), true)
		return nil
	}
	if len(stmts) == 0 {
		m.setStatus(m.t.T(i18n.KeyStatusEmptyQuery), false)
		return nil
	}
	m.setContext(contextQuery)
	return m.runStatement(prefix+stmts[0], false, 0)
}

func (m *Model) runTableCommand(stmt string) tea.Cmd {
	m.setContext(contextData)
	return m.runStatement(stmt, false, max(1, m.cfg.Query.MaxRows))
}

// queryContext returns a context honoring query.timeout. A non-positive
// timeout disables the client-side deadline.
func (m *Model) queryContext() (context.Context, context.CancelFunc) {
	if timeout := m.cfg.Query.Timeout.Std(); timeout > 0 {
		return context.WithTimeout(context.Background(), timeout)
	}
	return context.WithCancel(context.Background())
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
	ctx, cancel := m.queryContext()
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

	preserveFilter := ""
	if m.pager != nil && m.pager.schema == page.schema && m.pager.table == page.table {
		preserveFilter = m.results.Filter()
	}
	m.results.SetResult(out.Result)
	if preserveFilter != "" {
		m.results.SetFilter(preserveFilter)
	}
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
	info := m.t.T(i18n.KeyResultsPageInfo, pageNumber, len(out.Result.Rows), humanDuration(out.Result.Elapsed))
	if page.totalKnown {
		info = m.t.T(i18n.KeyResultsCount, page.total) + " · " + info
	}
	m.results.SetPage(page.stmt, info)
	m.setStatus(m.t.T(i18n.KeyStatusReady), false)
	return nil
}

func (m *Model) activateTable(schema, table, kind string) tea.Cmd {
	cmd := m.openTableTab(schema, table, kind)
	if m.conn != nil && (m.conn.Type() == "redis" || m.conn.Type() == "mongo") {
		m.setContext(contextData)
		return cmd
	}
	return tea.Batch(cmd, m.tableData(schema, table))
}

func (m *Model) tableStmt(schema, table, kind string) string {
	switch {
	case m.conn != nil && m.conn.Type() == "redis":
		return redisCommand(kind, table)
	case m.conn != nil && m.conn.Type() == "mongo":
		return mongoCommand(schema, table)
	default:
		return m.selectTableStmt(schema, table)
	}
}

func (m *Model) openTableTab(schema, table, kind string) tea.Cmd {
	m.syncActiveTab()
	m.completion.Close()
	source := tableSource{schema: schema, table: table}
	for i := range m.tabs {
		if m.tabs[i].source == source {
			m.activeTab = i
			m.editor = m.tabs[i].editor
			m.focus = focusEditor
			m.layout()
			return tea.Batch(m.editor.Focus(), m.scheduleTabSave())
		}
	}
	ed := editor.New(m.t)
	if m.history != nil {
		ed.SetHistory(m.history.Entries())
	}
	ed.SetValue(m.tableStmt(schema, table, kind))
	m.tabs = append(m.tabs, editorTab{title: qualifiedTable(schema, table), editor: ed, source: source})
	m.activeTab = len(m.tabs) - 1
	m.editor = ed
	m.focus = focusEditor
	m.layout()
	return tea.Batch(m.editor.Focus(), m.scheduleTabSave())
}

func sessionDatabase(name string) string {
	if index := strings.Index(name, "/"); index >= 0 {
		return name[index+1:]
	}
	return ""
}

func (m *Model) showConnectionInfo(name, status string) tea.Cmd {
	m.infoName, m.infoStatus = name, status
	m.infoVersion = "-"
	var cmd tea.Cmd
	if m.conn != nil && sessionBase(name) == sessionBase(m.connName) {
		if _, ok := m.conn.(db.Versioner); ok {
			m.infoVersion = ""
			cmd = m.versionCmd()
		}
	}
	m.renderConnectionInfo()
	return cmd
}

func (m *Model) renderConnectionInfo() {
	cfg := m.cfg.Find(m.infoName)
	if cfg == nil {
		return
	}
	text := func(value string) any {
		if value == "" {
			return m.t.T(i18n.KeyInfoNone)
		}
		return value
	}
	host := cfg.Host
	if cfg.Port > 0 {
		host = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	}
	switch cfg.Type {
	case config.TypeSQLite:
		host = cfg.Path
	case config.TypeRedis:
		host = cfg.Addr
	case config.TypeMongo:
		if cfg.URI != "" {
			host = cfg.URI
		}
	}
	database := cfg.Database
	if database == "" {
		database = m.t.T(i18n.KeyInfoAllDatabases)
	}
	readonly := m.t.T(i18n.KeyInfoNo)
	if cfg.ReadOnly {
		readonly = m.t.T(i18n.KeyInfoYes)
	}
	version := m.infoVersion
	if version == "" {
		version = "…"
	}
	rows := [][]any{
		{m.t.T(i18n.KeyFormName), text(cfg.Name)},
		{m.t.T(i18n.KeyFormType), text(cfg.Type)},
		{m.t.T(i18n.KeyFormHost), text(host)},
		{m.t.T(i18n.KeyFormUser), text(cfg.User)},
		{m.t.T(i18n.KeyFormDatabase), database},
	}
	if cfg.Type == config.TypePostgres {
		rows = append(rows, []any{m.t.T(i18n.KeyFormSSLMode), text(cfg.SSLMode)})
	}
	rows = append(rows,
		[]any{m.t.T(i18n.KeyInfoVersion), version},
		[]any{m.t.T(i18n.KeyInfoReadOnly), readonly},
		[]any{m.t.T(i18n.KeyInfoStatus), m.infoStatus},
	)
	m.results.SetResult(&db.ResultSet{
		Columns: []db.ColumnMeta{
			{Name: m.t.T(i18n.KeyInfoField)},
			{Name: m.t.T(i18n.KeyInfoValue)},
		},
		Rows: rows,
	})
	m.pager = nil
	m.setContext(contextInfo)
}

func (m *Model) versionCmd() tea.Cmd {
	if m.conn == nil {
		return nil
	}
	versioner, ok := m.conn.(db.Versioner)
	if !ok {
		return nil
	}
	m.versionSeq++
	id := m.versionSeq
	ctx, cancel := m.queryContext()
	return func() tea.Msg {
		defer cancel()
		version, err := versioner.Version(ctx)
		return versionLoadedMsg{id: id, version: version, err: err}
	}
}

func (m *Model) handleVersionLoaded(msg versionLoadedMsg) {
	if msg.id != m.versionSeq {
		return
	}
	if msg.err != nil || msg.version == "" {
		m.infoVersion = "-"
	} else {
		m.infoVersion = msg.version
	}
	if m.context == contextInfo {
		m.renderConnectionInfo()
	}
}

func (m *Model) loadOverviewCmd() tea.Cmd {
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	database := sessionDatabase(m.connName)
	if database == "" {
		if cfg := m.cfg.Find(m.connName); cfg != nil {
			database = cfg.Database
		}
	}
	if database == "" {
		m.setStatus(m.t.T(i18n.KeyStatusNoDatabase), false)
		return nil
	}
	m.overviewSeq++
	id := m.overviewSeq
	conn := m.conn
	typ := conn.Type()
	ctx, cancel := m.queryContext()
	return func() tea.Msg {
		defer cancel()
		rows, err := overviewRows(ctx, conn, typ, database)
		return overviewLoadedMsg{id: id, rows: rows, err: err}
	}
}

func overviewRows(ctx context.Context, conn db.Conn, typ, database string) ([][]any, error) {
	catalog := conn.Catalog()
	provider, _ := conn.(db.TableStatProvider)
	var schemas []string
	if typ == config.TypeMySQL || typ == config.TypeMongo {
		schemas = []string{database}
	} else {
		found, err := catalog.Schemas(ctx)
		if err != nil {
			return nil, err
		}
		schemas = found
	}
	var rows [][]any
	var firstErr error
	for _, schema := range schemas {
		objects, err := catalog.Tables(ctx, schema)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		stats := map[string]db.TableStat{}
		if provider != nil {
			if found, err := provider.TableStats(ctx, schema); err == nil {
				for _, stat := range found {
					stats[stat.Name] = stat
				}
			}
		}
		for _, obj := range objects {
			stat, known := stats[obj.Name]
			comment := "-"
			if known {
				comment = stat.Comment
			}
			rows = append(rows, []any{
				schema, obj.Name, obj.Kind,
				formatStat(stat.Rows, known),
				formatSize(stat.Size, known),
				formatStat(stat.Columns, known),
				comment,
			})
		}
	}
	if len(rows) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return rows, nil
}

func formatStat(value int64, known bool) string {
	if !known || value < 0 {
		return "-"
	}
	return strconv.FormatInt(value, 10)
}

func formatSize(value int64, known bool) string {
	if !known || value < 0 {
		return "-"
	}
	return humanBytes(value)
}

func humanBytes(value int64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := int64(unit), 0
	for rest := value / unit; rest >= unit && exp < 4; rest /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(value)/float64(div), "KMGTPE"[exp])
}

func typeHasSchema(typ string) bool {
	switch typ {
	case config.TypePostgres, config.TypeSQLite:
		return true
	default:
		return false
	}
}

func (m *Model) overviewColumns() []db.ColumnMeta {
	columns := make([]db.ColumnMeta, 0, 7)
	if m.conn == nil || typeHasSchema(m.conn.Type()) {
		columns = append(columns, db.ColumnMeta{Name: m.t.T(i18n.KeyOverviewSchema)})
	}
	return append(columns,
		db.ColumnMeta{Name: m.t.T(i18n.KeyOverviewTable)},
		db.ColumnMeta{Name: m.t.T(i18n.KeyOverviewType)},
		db.ColumnMeta{Name: m.t.T(i18n.KeyOverviewRows)},
		db.ColumnMeta{Name: m.t.T(i18n.KeyOverviewSize)},
		db.ColumnMeta{Name: m.t.T(i18n.KeyOverviewColumns)},
		db.ColumnMeta{Name: m.t.T(i18n.KeyOverviewComment)},
	)
}

func (m *Model) rebuildOverview() {
	rows := m.overviewAll
	if filter := strings.ToLower(strings.TrimSpace(m.overviewFilter)); filter != "" {
		filtered := make([][]any, 0, len(rows))
		for _, row := range rows {
			fields := make([]string, 0, 3)
			for i := 0; i < 3 && i < len(row); i++ {
				fields = append(fields, fmt.Sprint(row[i]))
			}
			if strings.Contains(strings.ToLower(strings.Join(fields, " ")), filter) {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	display := rows
	if m.conn != nil && !typeHasSchema(m.conn.Type()) {
		display = make([][]any, len(rows))
		for i, row := range rows {
			display[i] = row[1:]
		}
	}
	m.results.SetResult(&db.ResultSet{Columns: m.overviewColumns(), Rows: display})
	m.pager = nil
	m.setContext(contextTables)
	m.setStatus(m.t.T(i18n.KeyStatusTables, len(rows)), false)
}

func (m *Model) resetOverviewState() {
	m.overviewSeq++
	m.versionSeq++
	m.overviewAll = nil
	m.overviewFilter = ""
	m.filtering = false
	m.filterInput.SetValue("")
	m.filterInput.Blur()
}

func (m *Model) setContext(kind contextKind) {
	m.context = kind
	m.results.SetRowMode(kind == contextTables)
}

func (m *Model) openOverviewTable() tea.Cmd {
	row, ok := m.results.CursorRow()
	if !ok || m.conn == nil || len(row) < 2 {
		return nil
	}
	var schema, table, kind string
	if typeHasSchema(m.conn.Type()) {
		if len(row) < 3 {
			return nil
		}
		schema, _ = row[0].(string)
		table, _ = row[1].(string)
		kind, _ = row[2].(string)
	} else {
		schema = sessionDatabase(m.connName)
		table, _ = row[0].(string)
		kind, _ = row[1].(string)
	}
	if table == "" {
		return nil
	}
	return m.openTableData(schema, table, kind)
}

func (m *Model) handleOverviewLoaded(msg overviewLoadedMsg) tea.Cmd {
	if msg.id != m.overviewSeq {
		return nil
	}
	if msg.err != nil {
		m.overviewAll = nil
		m.results.SetError(m.t.T(i18n.KeyErrorOverview, msg.err))
		m.pager = nil
		m.setContext(contextTables)
		m.setStatus(m.t.T(i18n.KeyErrorOverview, msg.err), true)
		return nil
	}
	m.overviewAll = msg.rows
	m.rebuildOverview()
	return nil
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
		{ID: "table.insertrow", LabelKey: i18n.KeyMenuInsertRow, Kind: menu.KindAction},
		{ID: "table.import", LabelKey: i18n.KeyMenuImport, Kind: menu.KindAction},
		{ID: "table.manage", LabelKey: i18n.KeyMenuManageTable, Kind: menu.KindAction},
	}
}

func (m *Model) tableMenuItemsWithExport() []menu.Item {
	items := m.tableMenuItems()
	if m.conn != nil && sqlExportable(m.conn.Type()) {
		items = append(items, exportMenuItems("table")...)
	}
	return items
}

func (m *Model) nodeMenuTitle(n sidebar.NodeMenuMsg) string {
	switch n.Kind {
	case sidebar.KindConnection:
		return n.ConnName
	case sidebar.KindDatabase:
		return n.Label
	default:
		return qualifiedTable(n.Schema, n.Table)
	}
}

func (m *Model) nodeMenuItems(n sidebar.NodeMenuMsg) []menu.Item {
	switch n.Kind {
	case sidebar.KindConnection:
		if n.ConnName != sessionBase(m.connName) {
			return []menu.Item{
				{ID: "conn.connect", LabelKey: i18n.KeyMenuConnect, Kind: menu.KindAction},
			}
		}
		return []menu.Item{
			{ID: "conn.info", LabelKey: i18n.KeyMenuInfo, Kind: menu.KindAction},
			{ID: "conn.refresh", LabelKey: i18n.KeyMenuRefresh, Kind: menu.KindAction},
			{ID: "conn.disconnect", LabelKey: i18n.KeyMenuDisconnect, Kind: menu.KindAction},
		}
	case sidebar.KindDatabase:
		items := []menu.Item{
			{ID: "db.open", LabelKey: i18n.KeyMenuOpen, Kind: menu.KindAction},
			{ID: "db.refresh", LabelKey: i18n.KeyMenuRefresh, Kind: menu.KindAction},
			{ID: "db.close", LabelKey: i18n.KeyMenuCloseSession, Kind: menu.KindAction},
		}
		if sqlExportable(m.sessionType(n.SessionName)) {
			items = append(items, menu.Item{
				Kind: menu.KindHeader, LabelKey: i18n.KeyMenuManageTable,
			}, menu.Item{
				ID: "db.createtable", LabelKey: i18n.KeyMenuCreateTable, Kind: menu.KindAction,
			})
			items = append(items, exportMenuItems("db")...)
		}
		return items
	default:
		items := m.tableMenuItemsWithExport()
		if viewKind(n.Sub) {
			filtered := items[:0]
			for _, item := range items {
				switch item.ID {
				case "table.insert", "table.insertrow", "table.import", "table.manage":
					continue
				}
				filtered = append(filtered, item)
			}
			items = filtered
		}
		return items
	}
}

// viewKind reports whether a sidebar table node is a view, which does not
// support INSERT/import or table DDL.
func viewKind(sub string) bool {
	switch strings.ToLower(sub) {
	case "view", "materialized view":
		return true
	default:
		return false
	}
}

func (m *Model) sessionType(session string) string {
	if cfg := m.cfg.Find(session); cfg != nil {
		return cfg.Type
	}
	if m.conn != nil {
		return m.conn.Type()
	}
	return ""
}

func exportMenuItems(prefix string) []menu.Item {
	return []menu.Item{
		{Kind: menu.KindHeader, LabelKey: i18n.KeyMenuExport},
		{ID: prefix + ".export.sql", LabelKey: i18n.KeyMenuExportSQL, Kind: menu.KindAction},
		{ID: prefix + ".export.ddl", LabelKey: i18n.KeyMenuExportDDL, Kind: menu.KindAction},
		{ID: prefix + ".export.xlsx", LabelKey: i18n.KeyMenuExportXLSX, Kind: menu.KindAction},
		{ID: prefix + ".export.csv", LabelKey: i18n.KeyMenuExportCSV, Kind: menu.KindAction},
		{ID: prefix + ".export.json", LabelKey: i18n.KeyMenuExportJSON, Kind: menu.KindAction},
	}
}

func (m *Model) runNodeMenuAction(msg menu.ActionMsg) tea.Cmd {
	n := m.menuNode
	if strings.HasPrefix(msg.ID, "db.export.") {
		return m.exportDatabaseCmd(n.SessionName, strings.TrimPrefix(msg.ID, "db.export."))
	}
	switch msg.ID {
	case "conn.connect":
		return m.switchToConnection(n.ConnName)
	case "conn.info":
		return m.showConnectionInfo(n.ConnName, m.t.T(i18n.KeyStatusConnected))
	case "conn.refresh", "db.refresh":
		return m.sidebar.ReloadSelected()
	case "conn.disconnect":
		m.disconnectConnection(n.ConnName)
		return nil
	case "db.open":
		if n.SessionName == m.connName {
			return m.loadOverviewCmd()
		}
		return m.switchToConnection(n.SessionName)
	case "db.close":
		m.closeSession(n.SessionName)
		return nil
	case "db.createtable":
		return m.openCreateTableForm(n.SessionName)
	}
	return nil
}

func (m *Model) disconnectConnection(base string) {
	if sessionBase(m.connName) == base {
		m.disconnect()
	}
	for name, saved := range m.sessions {
		if sessionBase(name) == base {
			_ = saved.conn.Close()
			delete(m.sessions, name)
		}
	}
	m.sidebar.CollapseSession(base)
	m.syncSidebarSessions()
}

func (m *Model) closeSession(name string) {
	if name == "" {
		return
	}
	if name == m.connName {
		m.disconnect()
		return
	}
	if saved, ok := m.sessions[name]; ok {
		_ = saved.conn.Close()
		delete(m.sessions, name)
	}
	m.sidebar.CollapseSession(name)
	m.syncSidebarSessions()
	m.setStatus(m.t.T(i18n.KeyStatusSessionClosed, name), false)
}

func (m *Model) runTableMenuAction(msg menu.ActionMsg) tea.Cmd {
	schema, table := m.menuNode.Schema, m.menuNode.Table
	kind := m.menuNode.Sub
	if strings.HasPrefix(msg.ID, "table.dropcolumn.") {
		return m.confirmDropColumn(schema, table, strings.TrimPrefix(msg.ID, "table.dropcolumn."))
	}
	if strings.HasPrefix(msg.ID, "table.dropindex.") {
		return m.confirmDropIndex(schema, table, strings.TrimPrefix(msg.ID, "table.dropindex."))
	}
	if strings.HasPrefix(msg.ID, "table.export.") {
		return m.exportTableCmd(schema, table, strings.TrimPrefix(msg.ID, "table.export."))
	}
	if msg.ID == "table.data" {
		return m.openTableData(schema, table, kind)
	}
	if m.conn != nil && m.conn.Type() == "redis" {
		switch msg.ID {
		case "table.select":
			return m.openTableTab(schema, table, kind)
		}
		m.setStatus(m.t.T(i18n.KeyErrorUnsupportedFeature, "redis"), true)
		return nil
	}
	if m.conn != nil && m.conn.Type() == "mongo" {
		switch msg.ID {
		case "table.select":
			return m.openTableTab(schema, table, kind)
		}
	}
	switch msg.ID {
	case "table.select":
		return m.openTableTab(schema, table, kind)
	case "table.structure":
		return m.openDetail(schema, table)
	case "table.ddl":
		return m.openDDL(schema, table)
	case "table.indexes":
		return m.openIndexes(schema, table)
	case "table.insert":
		return m.editInfoCmd(schema, table, "insert")
	case "table.insertrow":
		return m.editInfoCmd(schema, table, "rowform")
	case "table.import":
		return m.editInfoCmd(schema, table, "import")
	case "table.manage":
		m.openManageMenu(schema, table)
		return nil
	case "table.droptable":
		return m.confirmDropTable(schema, table)
	case "table.truncate":
		return m.confirmTruncateTable(schema, table)
	case "table.rename":
		return m.openRenameTableForm(schema, table)
	case "table.addcolumn":
		return m.openAddColumnForm(schema, table)
	case "table.dropcolumn":
		return m.editInfoCmd(schema, table, "dropcolumnmenu")
	case "table.createindex":
		return m.openCreateIndexForm(schema, table)
	case "table.dropindex":
		return m.openDropIndexMenu(schema, table)
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
	name := fmt.Sprintf("dbrix-export-%s.%s", time.Now().Format("20060102-150405.000000000"), kind)
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
	case "exportdir":
		m.completion.Close()
		m.dirInput.Open(m.outDir)
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
		label := values[msg.Value].String()
		if values[msg.Value] <= 0 {
			label = m.t.T(i18n.KeyMenuUnlimited)
		}
		m.setStatus(m.t.T(i18n.KeyStatusTimeoutSet, label), false)
	case "maxrows":
		values := maxRowChoices()
		if msg.Value < 0 || msg.Value >= len(values) {
			return
		}
		m.cfg.Query.MaxRows = values[msg.Value]
		m.executor.MaxRows = values[msg.Value]
		m.setStatus(m.t.T(i18n.KeyStatusMaxRowsSet, values[msg.Value]), false)
	case "layout":
		names := m.layoutNames()
		if msg.Value < 0 || msg.Value >= len(names) {
			return
		}
		m.layoutMode = names[msg.Value]
		_ = config.SaveLayoutState(m.layoutMode)
		m.layout()
		m.setStatus(m.t.T(i18n.KeyStatusLayout, m.layoutLabels()[msg.Value]), false)
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
	session := m.connName
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
	m.setContext(contextQuery)
	m.resetOverviewState()
	m.sidebar.ClearConn()
	if session != "" {
		m.sidebar.CollapseSession(session)
	}
	m.results.Clear()
	_ = m.saveTabStatesNow()
	m.syncSidebarSessions()
	m.setStatus(m.t.T(i18n.KeyStatusDisconnected), false)
}

func (m *Model) switchToConnection(name string) tea.Cmd {
	if name == m.connName {
		return nil
	}
	if name != m.pendingCreateTable {
		m.pendingCreateTable = ""
	}
	if saved, ok := m.sessions[name]; ok {
		m.saveCurrentSession()
		m.activateSession(name, saved)
		cmds := make([]tea.Cmd, 0, 3)
		if cmd := m.sidebarConnCmd(saved.conn, name); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if saved.conn.Type() == config.TypePostgres {
			cmds = append(cmds, m.loadDatabasesCmd(sessionBase(name), saved.conn))
		}
		if sessionDatabase(name) != "" && sidebar.SupportsDatabases(saved.conn.Type()) && saved.context == contextTables {
			cmds = append(cmds, m.loadOverviewCmd())
		}
		return tea.Batch(cmds...)
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
	m.overviewSeq++
	m.versionSeq++
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
		context:      m.context,
		overviewAll:  m.overviewAll,
		filter:       m.overviewFilter,
		cursor:       m.sidebar.CursorPath(),
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
	m.setContext(saved.context)
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
	m.resetOverviewState()
	m.overviewAll = saved.overviewAll
	m.overviewFilter = saved.filter
	m.filterInput.SetValue(saved.filter)
	m.sidebar.RestoreCursor(saved.cursor)
	delete(m.sessions, name)
	m.closeOverlays()
	m.layout()
	m.syncSidebarSessions()
	m.setStatus(m.t.T(i18n.KeyStatusReady), false)
}

func (m *Model) resetSession() {
	m.conn = nil
	m.connName = ""
	m.running = false
	m.pendingCmd = nil
	m.tabConnName = ""
	m.setContext(contextQuery)
	m.resetOverviewState()
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
	m.dirInput.Close()
	m.form.Close()
	m.completion.Close()
	m.showHelp = false
	m.pendingCmd = nil
	m.pendingImport = nil
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
	timeoutValue := len(timeouts) - 1
	currentTimeout := m.cfg.Query.Timeout.Std()
	if currentTimeout > 0 {
		timeoutValue = 0
		for i, d := range timeouts {
			if d > 0 && d <= currentTimeout {
				timeoutValue = i
			}
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
	layoutNames := m.layoutNames()
	layoutValue := 0
	for i, name := range layoutNames {
		if name == m.layoutMode {
			layoutValue = i
			break
		}
	}

	return []menu.Item{
		{ID: "add", LabelKey: i18n.KeyCmdAdd, Kind: menu.KindAction},
		{ID: "disconnect", LabelKey: i18n.KeyCmdDisconnect, Kind: menu.KindAction},
		{ID: "help", LabelKey: i18n.KeyCmdHelp, Kind: menu.KindAction},
		{ID: "quit", LabelKey: i18n.KeyCmdQuit, Kind: menu.KindAction},
		{ID: "theme", LabelKey: i18n.KeyMenuTheme, Kind: menu.KindChoice, Choices: themes, Value: themeValue, Tab: 1},
		{ID: "lang", LabelKey: i18n.KeyMenuLanguage, Kind: menu.KindChoice, Choices: langs, Value: langValue, Tab: 1},
		{ID: "timeout", LabelKey: i18n.KeyMenuTimeout, Kind: menu.KindChoice, Choices: timeoutLabels(m.t), Value: timeoutValue, Tab: 1},
		{ID: "maxrows", LabelKey: i18n.KeyMenuMaxRows, Kind: menu.KindChoice, Choices: maxRowLabels(), Value: rowValue, Tab: 1},
		{ID: "layout", LabelKey: i18n.KeyMenuLayoutMode, Kind: menu.KindChoice, Choices: m.layoutLabels(), Value: layoutValue, Tab: 1},
		{ID: "exportdir", LabelKey: i18n.KeyMenuExportDir, Kind: menu.KindAction, Tab: 1},
	}
}

func (m *Model) layoutNames() []string {
	return []string{config.LayoutStacked, config.LayoutColumns, config.LayoutResultsTop}
}

func (m *Model) layoutLabels() []string {
	return []string{
		m.t.T(i18n.KeyLayoutStacked),
		m.t.T(i18n.KeyLayoutColumns),
		m.t.T(i18n.KeyLayoutResultsTop),
	}
}

func timeoutChoices() []time.Duration {
	return []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second, 120 * time.Second, 0}
}

func timeoutLabels(t *i18n.Translator) []string {
	values := timeoutChoices()
	out := make([]string, len(values))
	for i, d := range values {
		if d <= 0 {
			out[i] = t.T(i18n.KeyMenuUnlimited)
			continue
		}
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
	m.setContext(contextQuery)
	m.pager = nil
	m.setStatus(m.t.T(i18n.KeyStatusScript, msg.executed, msg.total, humanDuration(msg.elapsed)), false)
}

func (m *Model) SetHistory(store *history.Store) {
	m.history = store
	m.editor.SetHistory(store.Entries())
}

// SetSender connects the model to the Bubble Tea program so background work
// can report progress.
func (m *Model) SetSender(send func(tea.Msg)) { m.send = send }

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
	m.quitting.Store(true)
	m.connectSeq++
	if m.connectCancel != nil {
		m.connectCancel()
		m.connectCancel = nil
	}
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

// Shutdown closes every open session and database connection. It is safe to
// call after the Bubble Tea program has exited, and more than once.
func (m *Model) Shutdown() { m.shutdown() }

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

package sidebar

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/dbrix/internal/config"
	"github.com/LinduanTof/dbrix/internal/db"
	"github.com/LinduanTof/dbrix/internal/i18n"
	"github.com/LinduanTof/dbrix/internal/ui/textutil"
	"github.com/LinduanTof/dbrix/internal/ui/theme"
)

type NodeKind int

const (
	KindConnection NodeKind = iota
	KindDatabase
	KindSchema
	KindTable
	KindColumn
)

type Node struct {
	Kind        NodeKind
	Label       string
	Sub         string
	ConnName    string
	SessionName string
	Schema      string
	Table       string
	Depth       int
	Expanded    bool
	Loading     bool
	Children    []*Node
}

type DatabaseEntry struct {
	Name    string
	Session string
}

type SchemasLoadedMsg struct {
	Epoch   uint64
	Node    *Node
	Schemas []string
	Err     error
}

type TablesLoadedMsg struct {
	Epoch   uint64
	Node    *Node
	Objects []db.Object
	Err     error
}

type ColumnsLoadedMsg struct {
	Epoch   uint64
	Node    *Node
	Columns []db.Column
	Err     error
}

type ConnectMsg struct{ Name string }

type DatabaseMsg struct{ Session string }

type TableActivateMsg struct{ Schema, Table, Kind string }

type EditorOpenMsg struct{ Schema, Table, Kind string }

type ConnectionInfoMsg struct{ Name string }

type TableOpenMsg struct{ Schema, Table, Kind string }

type NodeMenuMsg struct {
	Kind        NodeKind
	Label       string
	ConnName    string
	SessionName string
	Schema      string
	Table       string
	Sub         string
}

func MsgError(msg any) error {
	switch m := msg.(type) {
	case SchemasLoadedMsg:
		return m.Err
	case TablesLoadedMsg:
		return m.Err
	case ColumnsLoadedMsg:
		return m.Err
	}
	return nil
}

type Model struct {
	t *i18n.Translator

	width, height int
	roots         []*Node
	flat          []*Node
	cursor        int
	pendingCursor string
	offset        int

	lastClickIndex int
	lastClickAt    time.Time

	conn     db.Conn
	connName string
	epoch    uint64

	activeSessions map[string]bool
	currentLabel   string

	configPath string
}

func New(connections []config.Connection, t *i18n.Translator) Model {
	roots := make([]*Node, 0, len(connections))
	for _, c := range connections {
		roots = append(roots, &Node{
			Kind:     KindConnection,
			Label:    c.Name,
			Sub:      c.Type,
			ConnName: c.Name,
		})
	}
	m := Model{t: t, roots: roots}
	m.rebuild()
	return m
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) SetConfigPath(path string) {
	m.configPath = path
}

func (m *Model) SetActiveSessions(names []string) {
	m.activeSessions = make(map[string]bool, len(names))
	for _, name := range names {
		if name != "" {
			m.activeSessions[name] = true
		}
	}
}

func (m *Model) isActive(name string) bool {
	if name == "" {
		return false
	}
	if name == m.connName || m.activeSessions[name] {
		return true
	}
	prefix := name + "/"
	if strings.HasPrefix(m.connName, prefix) {
		return true
	}
	for session := range m.activeSessions {
		if strings.HasPrefix(session, prefix) {
			return true
		}
	}
	return false
}

func (m *Model) isCurrentRoot(name string) bool {
	return name != "" && (name == m.connName || strings.HasPrefix(m.connName, name+"/"))
}

func (m *Model) isSessionActive(name string) bool {
	return name != "" && (name == m.connName || m.activeSessions[name])
}

func (m *Model) sessionCount() int {
	seen := make(map[string]bool, len(m.activeSessions)+1)
	for name := range m.activeSessions {
		if name != "" {
			seen[name] = true
		}
	}
	if m.connName != "" {
		seen[m.connName] = true
	}
	return len(seen)
}

func (m *Model) SetConn(conn db.Conn) tea.Cmd {
	m.epoch++
	epoch := m.epoch
	m.conn = conn
	m.connName = conn.Name()
	m.currentLabel = ""
	for _, root := range m.roots {
		m.deactivateRoot(root)
	}
	for _, root := range m.roots {
		if root.ConnName == conn.Name() {
			root.Expanded = true
			root.Loading = true
			m.rebuild()
			return loadSchemas(conn, root, epoch)
		}
	}
	m.rebuild()
	return nil
}

func (m *Model) deactivateRoot(root *Node) {
	if SupportsDatabases(root.Sub) {
		for _, child := range root.Children {
			if child.Kind == KindDatabase {
				child.Expanded = false
				child.Loading = false
				child.Children = nil
			}
		}
		root.Loading = false
		return
	}
	root.Children = nil
	root.Expanded = false
	root.Loading = false
}

// SupportsDatabases reports whether the connection type uses one session per
// database and shows a database list under the connection.
func SupportsDatabases(typ string) bool {
	switch typ {
	case config.TypePostgres, config.TypeMySQL, config.TypeMongo:
		return true
	default:
		return false
	}
}

func databaseIsSchema(typ string) bool {
	return typ == config.TypeMySQL || typ == config.TypeMongo
}

func (m *Model) SetDatabaseConn(conn db.Conn, base, database, session string) tea.Cmd {
	m.epoch++
	epoch := m.epoch
	m.conn = conn
	m.connName = session
	m.currentLabel = database

	var root *Node
	for _, candidate := range m.roots {
		m.deactivateRoot(candidate)
		candidate.Expanded = candidate.ConnName == base
		if candidate.ConnName == base {
			root = candidate
		}
	}
	if root == nil {
		m.rebuild()
		return nil
	}
	var dbNode *Node
	for _, child := range root.Children {
		if child.Kind == KindDatabase && child.SessionName == session {
			dbNode = child
			break
		}
	}
	if dbNode == nil {
		dbNode = &Node{Kind: KindDatabase, ConnName: session, SessionName: session}
		root.Children = append(root.Children, dbNode)
	}
	dbNode.Label = database
	dbNode.ConnName = session
	dbNode.SessionName = session
	dbNode.Schema = database
	dbNode.Expanded = true
	dbNode.Loading = true
	root.Expanded = true
	m.rebuild()
	if databaseIsSchema(conn.Type()) {
		return loadTables(conn, dbNode, epoch)
	}
	return loadSchemas(conn, dbNode, epoch)
}

func (m *Model) SetDatabaseRoot(conn db.Conn, base string) tea.Cmd {
	m.epoch++
	m.conn = conn
	m.connName = base
	m.currentLabel = ""
	for _, root := range m.roots {
		m.deactivateRoot(root)
		root.Expanded = root.ConnName == base
	}
	m.rebuild()
	return nil
}

func (m *Model) SetDatabases(base string, entries []DatabaseEntry) {
	var root *Node
	for _, candidate := range m.roots {
		if candidate.ConnName == base {
			root = candidate
			break
		}
	}
	if root == nil {
		return
	}
	existing := make(map[string]*Node, len(root.Children))
	for _, child := range root.Children {
		if child.Kind == KindDatabase {
			existing[child.SessionName] = child
		}
	}
	nodes := make([]*Node, 0, len(entries)+len(existing))
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.Name == "" || entry.Session == "" || seen[entry.Session] {
			continue
		}
		seen[entry.Session] = true
		node := existing[entry.Session]
		if node == nil {
			node = &Node{Kind: KindDatabase, ConnName: entry.Session, SessionName: entry.Session}
		}
		node.Label = entry.Name
		node.ConnName = entry.Session
		node.SessionName = entry.Session
		node.Schema = entry.Name
		if !m.isSessionActive(entry.Session) {
			node.Expanded = false
			node.Loading = false
			node.Children = nil
		}
		nodes = append(nodes, node)
	}
	for session, node := range existing {
		if !seen[session] && m.isSessionActive(session) {
			nodes = append(nodes, node)
		}
	}
	root.Children = nodes
	root.Expanded = true
	m.rebuild()
}

func (m *Model) AddConnection(c config.Connection) {
	for _, root := range m.roots {
		if root.ConnName == c.Name {
			return
		}
	}
	m.roots = append(m.roots, &Node{
		Kind:     KindConnection,
		Label:    c.Name,
		Sub:      c.Type,
		ConnName: c.Name,
	})
	m.rebuild()
}

func (m *Model) connectionLabel(name string) string {
	for _, root := range m.roots {
		if root.ConnName == name {
			return root.Label
		}
	}
	return name
}

func (m *Model) SelectedTable() (schema, table string, ok bool) {
	n := m.selected()
	if n == nil || n.Kind != KindTable {
		return "", "", false
	}
	return n.Schema, n.Table, true
}

// CollapseSession collapses the node representing session: the connection
// root for a base session, or the database node for a derived session.
func (m *Model) CollapseSession(session string) {
	if session == "" {
		return
	}
	base, database := session, ""
	if index := strings.Index(session, "/"); index >= 0 {
		base, database = session[:index], session[index+1:]
	}
	for _, root := range m.roots {
		if root.ConnName != base {
			continue
		}
		if database == "" {
			root.Expanded = false
			for _, child := range root.Children {
				if child.Kind == KindDatabase {
					child.Expanded = false
				}
			}
			continue
		}
		for _, child := range root.Children {
			if child.Kind == KindDatabase && child.SessionName == session {
				child.Expanded = false
			}
		}
	}
	m.rebuild()
}

func (m *Model) ClearConn() {
	m.epoch++
	m.conn = nil
	m.connName = ""
	m.currentLabel = ""
	m.pendingCursor = ""
	for _, root := range m.roots {
		m.deactivateRoot(root)
	}
	m.rebuild()
}

func (m *Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return *m, m.handleKey(msg)
	case SchemasLoadedMsg:
		if msg.Epoch != m.epoch {
			return *m, nil
		}
		msg.Node.Loading = false
		if msg.Err == nil {
			children := make([]*Node, 0, len(msg.Schemas))
			for _, schema := range msg.Schemas {
				children = append(children, &Node{
					Kind:     KindSchema,
					Label:    schema,
					ConnName: msg.Node.ConnName,
					Schema:   schema,
				})
			}
			msg.Node.Children = children
			msg.Node.Expanded = true
		}
		m.rebuild()
		return *m, nil
	case TablesLoadedMsg:
		if msg.Epoch != m.epoch {
			return *m, nil
		}
		msg.Node.Loading = false
		if msg.Err == nil {
			children := make([]*Node, 0, len(msg.Objects))
			for _, obj := range msg.Objects {
				children = append(children, &Node{
					Kind:     KindTable,
					Label:    obj.Name,
					Sub:      obj.Kind,
					ConnName: msg.Node.ConnName,
					Schema:   msg.Node.Schema,
					Table:    obj.Name,
				})
			}
			msg.Node.Children = children
			msg.Node.Expanded = true
		}
		m.rebuild()
		return *m, nil
	case ColumnsLoadedMsg:
		if msg.Epoch != m.epoch {
			return *m, nil
		}
		msg.Node.Loading = false
		if msg.Err == nil {
			children := make([]*Node, 0, len(msg.Columns))
			for _, col := range msg.Columns {
				sub := col.DataType
				if col.PrimaryKey {
					sub += " PK"
				}
				children = append(children, &Node{
					Kind:     KindColumn,
					Label:    col.Name,
					Sub:      sub,
					ConnName: msg.Node.ConnName,
					Schema:   msg.Node.Schema,
					Table:    msg.Node.Table,
				})
			}
			msg.Node.Children = children
			msg.Node.Expanded = true
		}
		m.rebuild()
		return *m, nil
	}
	return *m, nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "right", "l":
		if n := m.selected(); n != nil {
			return m.toggle(n)
		}
	case "left", "h":
		if n := m.selected(); n != nil && n.Expanded {
			n.Expanded = false
			m.rebuild()
		}
	case "enter":
		return m.activate()
	case "m":
		n := m.selected()
		if n != nil {
			switch n.Kind {
			case KindConnection, KindDatabase, KindTable:
				return func() tea.Msg {
					return NodeMenuMsg{
						Kind:        n.Kind,
						Label:       n.Label,
						ConnName:    n.ConnName,
						SessionName: n.SessionName,
						Schema:      n.Schema,
						Table:       n.Table,
						Sub:         n.Sub,
					}
				}
			}
		}
	case "o":
		n := m.selected()
		if n != nil && n.Kind == KindTable {
			return func() tea.Msg {
				return EditorOpenMsg{Schema: n.Schema, Table: n.Table, Kind: n.Sub}
			}
		}
	case "s":
		n := m.selected()
		if n != nil && n.Kind == KindTable {
			return func() tea.Msg {
				return TableOpenMsg{Schema: n.Schema, Table: n.Table, Kind: n.Sub}
			}
		}
	case "r":
		if n := m.selected(); n != nil {
			return m.reload(n)
		}
	}
	return nil
}

func (m *Model) activate() tea.Cmd {
	n := m.selected()
	if n == nil {
		return nil
	}
	if n.Kind == KindConnection {
		if SupportsDatabases(n.Sub) && len(n.Children) == 0 && !m.isCurrentRoot(n.ConnName) {
			name := n.ConnName
			return func() tea.Msg { return ConnectMsg{Name: name} }
		}
		if !SupportsDatabases(n.Sub) && !m.isCurrentRoot(n.ConnName) {
			name := n.ConnName
			return func() tea.Msg { return ConnectMsg{Name: name} }
		}
		cmd := m.toggle(n)
		if m.isCurrentRoot(n.ConnName) {
			return withMsg(cmd, ConnectionInfoMsg{Name: n.ConnName})
		}
		return cmd
	}
	if n.Kind == KindDatabase {
		session := n.SessionName
		open := func() tea.Msg { return DatabaseMsg{Session: session} }
		if session == m.connName {
			return withMsg(m.toggle(n), open())
		}
		return open
	}
	if n.Kind == KindTable {
		schema, table, kind := n.Schema, n.Table, n.Sub
		return func() tea.Msg { return TableActivateMsg{Schema: schema, Table: table, Kind: kind} }
	}
	return m.toggle(n)
}

func withMsg(cmd tea.Cmd, msg tea.Msg) tea.Cmd {
	wrapped := func() tea.Msg { return msg }
	if cmd == nil {
		return wrapped
	}
	return tea.Batch(cmd, wrapped)
}

func (m *Model) HandleClick(row int) tea.Cmd {
	row -= m.headerHeight()
	if row < 0 {
		m.lastClickIndex = -1
		return nil
	}
	index := m.offset + row
	if index < 0 || index >= len(m.flat) {
		m.lastClickIndex = -1
		return nil
	}
	double := index == m.lastClickIndex && time.Since(m.lastClickAt) < 400*time.Millisecond
	if double {
		m.lastClickIndex = -1
	} else {
		m.lastClickIndex = index
		m.lastClickAt = time.Now()
	}
	m.cursor = index
	node := m.flat[index]
	if node.Kind == KindTable {
		if double {
			schema, table, kind := node.Schema, node.Table, node.Sub
			return func() tea.Msg {
				return TableActivateMsg{Schema: schema, Table: table, Kind: kind}
			}
		}
		return nil
	}
	if node.Kind == KindDatabase {
		if double {
			return m.activate()
		}
		return nil
	}
	if double {
		return nil
	}
	return m.activate()
}

func (m *Model) Scroll(delta int) {
	m.move(delta)
}

func (m *Model) selected() *Node {
	if m.cursor < 0 || m.cursor >= len(m.flat) {
		return nil
	}
	return m.flat[m.cursor]
}

func (m *Model) move(delta int) {
	if len(m.flat) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.flat)-1)
}

func (m *Model) toggle(n *Node) tea.Cmd {
	if n.Kind == KindConnection && SupportsDatabases(n.Sub) {
		if len(n.Children) == 0 && !m.isCurrentRoot(n.ConnName) {
			name := n.ConnName
			return func() tea.Msg { return ConnectMsg{Name: name} }
		}
		n.Expanded = !n.Expanded
		m.rebuild()
		return nil
	}
	if n.Kind == KindDatabase && n.SessionName != m.connName {
		session := n.SessionName
		return func() tea.Msg { return DatabaseMsg{Session: session} }
	}
	if n.Expanded {
		n.Expanded = false
		m.rebuild()
		return nil
	}
	n.Expanded = true
	m.rebuild()
	if n.Loading || len(n.Children) > 0 {
		return nil
	}
	if m.conn == nil || m.conn.Name() != n.ConnName {
		return nil
	}
	switch n.Kind {
	case KindConnection:
		n.Loading = true
		m.rebuild()
		return loadSchemas(m.conn, n, m.epoch)
	case KindDatabase:
		n.Loading = true
		m.rebuild()
		if databaseIsSchema(m.conn.Type()) {
			return loadTables(m.conn, n, m.epoch)
		}
		return loadSchemas(m.conn, n, m.epoch)
	case KindSchema:
		n.Loading = true
		m.rebuild()
		return loadTables(m.conn, n, m.epoch)
	case KindTable:
		n.Loading = true
		m.rebuild()
		return loadColumns(m.conn, n, m.epoch)
	}
	return nil
}

func (m *Model) reload(n *Node) tea.Cmd {
	switch n.Kind {
	case KindConnection:
		if !m.isCurrentRoot(n.ConnName) {
			return nil
		}
		if SupportsDatabases(n.Sub) {
			n.Expanded = true
			m.rebuild()
			return nil
		}
	case KindDatabase, KindSchema, KindTable:
	default:
		return nil
	}
	n.Children = nil
	n.Expanded = false
	m.rebuild()
	return m.toggle(n)
}

func (m *Model) rebuild() {
	m.flat = m.flat[:0]
	var walk func(nodes []*Node, depth int)
	walk = func(nodes []*Node, depth int) {
		for _, n := range nodes {
			n.Depth = depth
			m.flat = append(m.flat, n)
			if n.Expanded {
				walk(n.Children, depth+1)
			}
		}
	}
	walk(m.roots, 0)
	if len(m.flat) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(m.flat) {
		m.cursor = len(m.flat) - 1
	}
	if m.pendingCursor != "" {
		for i, n := range m.flat {
			if nodePath(n) == m.pendingCursor {
				m.cursor = i
				m.pendingCursor = ""
				break
			}
		}
	}
}

func nodePath(n *Node) string {
	return fmt.Sprintf("%d|%s|%s|%s|%s|%s", n.Kind, n.ConnName, n.SessionName, n.Schema, n.Table, n.Label)
}

// CursorPath identifies the selected node so it can be restored after the
// tree is rebuilt for another session.
func (m *Model) CursorPath() string {
	n := m.selected()
	if n == nil {
		return ""
	}
	return nodePath(n)
}

// RestoreCursor selects path once it appears in the tree. A path that does not
// match yet is kept, because table and column nodes load asynchronously.
func (m *Model) RestoreCursor(path string) {
	m.pendingCursor = path
}

// ReloadSession reloads the database node that owns a session, which refreshes
// its table list after a schema change. It falls back to the connection root
// when the session is not open as a separate node.
func (m *Model) ReloadSession(session string) tea.Cmd {
	var find func(nodes []*Node) *Node
	find = func(nodes []*Node) *Node {
		for _, n := range nodes {
			switch {
			case n.Kind == KindDatabase && n.SessionName == session:
				return n
			case n.Kind == KindConnection && n.ConnName == session:
				return n
			}
			if found := find(n.Children); found != nil {
				return found
			}
		}
		return nil
	}
	if n := find(m.roots); n != nil {
		return m.reload(n)
	}
	return nil
}

// ReloadSelected reloads the selected node, if it belongs to the current
// connection.
func (m *Model) ReloadSelected() tea.Cmd {
	if n := m.selected(); n != nil {
		return m.reload(n)
	}
	return nil
}

func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	header := m.renderHeader()
	bodyH := m.height - m.headerHeight()
	if bodyH < 1 {
		return textutil.FitLine(header, m.width)
	}
	if len(m.flat) == 0 {
		message := textutil.FitLines(theme.Muted.Render(m.t.T(i18n.KeyNoConnections, m.configPath)), m.width, bodyH)
		return textutil.FitLines(header+"\n"+message, m.width, m.height)
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+bodyH {
		m.offset = m.cursor - bodyH + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	maxOffset := max(0, len(m.flat)-bodyH)
	if m.offset > maxOffset {
		m.offset = maxOffset
	}

	lines := make([]string, 0, m.height)
	lines = append(lines, header)
	labelWidth := m.connectionLabelWidth()
	for i := m.offset; i < len(m.flat) && len(lines) < m.height; i++ {
		lines = append(lines, m.renderNode(m.flat[i], i == m.cursor, labelWidth))
	}
	return textutil.FitLines(strings.Join(lines, "\n"), m.width, m.height)
}

func (m *Model) headerHeight() int {
	return 1
}

func (m *Model) renderHeader() string {
	name := m.t.T(i18n.KeyStatusNoConnection)
	typ := ""
	dot := "○"
	styledDot := theme.Muted.Render(dot)
	styledName := theme.Muted.Render(name)
	if m.connName != "" {
		name = m.connectionLabel(m.connName)
		if m.currentLabel != "" {
			name = m.currentLabel
		}
		dot = "●"
		styledDot = theme.Title.Render(dot)
		styledName = theme.Title.Render(name)
		if m.conn != nil {
			typ = m.conn.Type()
		} else {
			for _, root := range m.roots {
				if root.ConnName == m.connName {
					typ = root.Sub
					break
				}
			}
		}
	}
	plain := " " + dot + " " + name
	styled := " " + styledDot + " " + styledName
	count := fmt.Sprintf(m.t.T(i18n.KeySidebarActive), m.sessionCount())
	available := max(0, m.width-textutil.Width(count)-2)
	if typ != "" && textutil.Width(plain)+2+textutil.Width(typ) <= available {
		plain += "  " + typ
		styled += "  " + theme.Muted.Render(typ)
	}
	if textutil.Width(plain) > available {
		plain = textutil.Truncate(plain, available)
		styled = textutil.Truncate(styled, available)
	}
	gap := max(0, m.width-textutil.Width(plain)-textutil.Width(count)-2)
	line := styled + " " + theme.Muted.Render(strings.Repeat("─", gap)) + " " + theme.Muted.Render(count)
	return textutil.FitLine(line, m.width)
}

func (m *Model) connectionLabelWidth() int {
	width := 0
	for _, root := range m.roots {
		width = max(width, textutil.Width(root.Label))
	}
	return max(width, 8)
}

func (m *Model) renderNode(n *Node, selected bool, labelWidth int) string {
	plain, styled := m.nodeLine(n, labelWidth)
	plain = textutil.FitLine(plain, m.width)
	if selected {
		return theme.Selected.Render(plain)
	}
	return textutil.FitLine(styled, m.width)
}

func (m *Model) nodeLine(n *Node, labelWidth int) (string, string) {
	indent := strings.Repeat("  ", n.Depth)
	var plain, styled string
	subGap := 0
	switch n.Kind {
	case KindConnection:
		caret := "▸"
		if n.Expanded {
			caret = "▾"
		}
		dot := "○"
		styledDot := theme.Muted.Render(dot)
		styledName := n.Label
		if m.isCurrentRoot(n.ConnName) {
			dot, styledDot = "●", theme.Title.Render("●")
			styledName = theme.Title.Render(n.Label)
		} else if m.isActive(n.ConnName) {
			dot, styledDot = "◉", theme.Header.Render("◉")
			styledName = theme.Header.Render(n.Label)
		}
		icon := typeIcon(n.Sub)
		plain = indent + caret + " " + dot + " " + icon + " " + pad(n.Label, labelWidth)
		styled = theme.Muted.Render(indent+caret) + " " + styledDot + " " +
			theme.Header.Render(icon) + " " + pad(styledName, labelWidth)
		subGap = 2
	case KindDatabase:
		caret := "▸"
		if n.Expanded {
			caret = "▾"
		}
		current := n.SessionName == m.connName
		active := current || m.isSessionActive(n.SessionName)
		dot := "○"
		styledDot := theme.Muted.Render(dot)
		styledName := n.Label
		if current {
			dot, styledDot = "●", theme.Title.Render("●")
			styledName = theme.Title.Render(n.Label)
		} else if active {
			dot, styledDot = "◉", theme.Header.Render("◉")
			styledName = theme.Header.Render(n.Label)
		}
		plain = indent + caret + " " + dot + " ▤ " + n.Label
		styled = theme.Muted.Render(indent+caret) + " " + styledDot + " " +
			theme.Header.Render("▤") + " " + styledName
	case KindSchema:
		caret := "▸"
		if n.Expanded {
			caret = "▾"
		}
		plain = indent + caret + " " + n.Label
		styled = theme.Muted.Render(indent+caret) + " " + theme.Header.Render(n.Label)
	case KindTable:
		caret := "▸"
		if n.Expanded {
			caret = "▾"
		}
		icon := tableIcon(n.Sub)
		plain = indent + caret + " " + icon + " " + n.Label
		styled = theme.Muted.Render(indent+caret) + " " + theme.Header.Render(icon) + " " + n.Label
	case KindColumn:
		plain = indent + "· " + n.Label
		styled = theme.Muted.Render(indent + "· " + n.Label)
	}
	if n.Loading {
		plain += " …"
		styled += " " + theme.Muted.Render("…")
	}
	if n.Sub != "" {
		gap := subGap
		if gap == 0 {
			gap = max(1, m.width-textutil.Width(plain)-textutil.Width(n.Sub)-1)
		}
		plain += strings.Repeat(" ", gap) + n.Sub
		styled += strings.Repeat(" ", gap) + theme.Muted.Render(n.Sub)
	}
	return plain, styled
}

func pad(s string, width int) string {
	if d := width - textutil.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func (m *Model) PreferredWidth() int {
	labelWidth := m.connectionLabelWidth()
	subWidth := 0
	for _, root := range m.roots {
		subWidth = max(subWidth, textutil.Width(root.Sub))
	}
	return 8 + labelWidth + 2 + subWidth
}

func typeIcon(typ string) string {
	switch typ {
	case config.TypePostgres:
		return "◉"
	case config.TypeMySQL:
		return "◈"
	case config.TypeSQLite:
		return "▣"
	case config.TypeRedis:
		return "◇"
	case config.TypeMongo:
		return "◍"
	default:
		return "▤"
	}
}

func tableIcon(kind string) string {
	switch strings.ToLower(kind) {
	case "view":
		return "◇"
	case "materialized view":
		return "◆"
	default:
		return "▦"
	}
}

func loadSchemas(conn db.Conn, node *Node, epoch uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		schemas, err := conn.Catalog().Schemas(ctx)
		return SchemasLoadedMsg{Epoch: epoch, Node: node, Schemas: schemas, Err: err}
	}
}

func loadTables(conn db.Conn, node *Node, epoch uint64) tea.Cmd {
	schema := node.Schema
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		objects, err := conn.Catalog().Tables(ctx, schema)
		return TablesLoadedMsg{Epoch: epoch, Node: node, Objects: objects, Err: err}
	}
}

func loadColumns(conn db.Conn, node *Node, epoch uint64) tea.Cmd {
	schema, table := node.Schema, node.Table
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cols, err := conn.Catalog().Columns(ctx, schema, table)
		return ColumnsLoadedMsg{Epoch: epoch, Node: node, Columns: cols, Err: err}
	}
}

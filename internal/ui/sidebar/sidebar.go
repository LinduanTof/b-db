package sidebar

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/linduan/bdb/internal/config"
	"github.com/linduan/bdb/internal/db"
	"github.com/linduan/bdb/internal/i18n"
	"github.com/linduan/bdb/internal/ui/textutil"
	"github.com/linduan/bdb/internal/ui/theme"
)

type NodeKind int

const (
	KindConnection NodeKind = iota
	KindSchema
	KindTable
	KindColumn
)

type Node struct {
	Kind     NodeKind
	Label    string
	Sub      string
	ConnName string
	Schema   string
	Table    string
	Depth    int
	Expanded bool
	Loading  bool
	Children []*Node
}

type SchemasLoadedMsg struct {
	Node    *Node
	Schemas []string
	Err     error
}

type TablesLoadedMsg struct {
	Node    *Node
	Objects []db.Object
	Err     error
}

type ColumnsLoadedMsg struct {
	Node    *Node
	Columns []db.Column
	Err     error
}

type ConnectMsg struct{ Name string }

type SelectTableMsg struct{ Schema, Table string }

type TableMenuMsg struct{ Schema, Table string }

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
	offset        int

	conn     db.Conn
	connName string
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

func (m *Model) SetConn(conn db.Conn) tea.Cmd {
	m.conn = conn
	m.connName = conn.Name()
	for _, root := range m.roots {
		root.Children = nil
		root.Expanded = false
		root.Loading = false
	}
	for _, root := range m.roots {
		if root.ConnName == conn.Name() {
			root.Expanded = true
			root.Loading = true
			m.rebuild()
			return loadSchemas(conn, root)
		}
	}
	m.rebuild()
	return nil
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

func (m *Model) SelectedTable() (schema, table string, ok bool) {
	n := m.selected()
	if n == nil || n.Kind != KindTable {
		return "", "", false
	}
	return n.Schema, n.Table, true
}

func (m *Model) ClearConn() {
	m.conn = nil
	m.connName = ""
	for _, root := range m.roots {
		root.Children = nil
		root.Expanded = false
		root.Loading = false
	}
	m.rebuild()
}

func (m *Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return *m, m.handleKey(msg)
	case SchemasLoadedMsg:
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
		n := m.selected()
		if n == nil {
			return nil
		}
		if n.Kind == KindConnection && n.ConnName != m.connName {
			name := n.ConnName
			return func() tea.Msg { return ConnectMsg{Name: name} }
		}
		if n.Kind == KindTable {
			schema, table := n.Schema, n.Table
			return func() tea.Msg { return TableMenuMsg{Schema: schema, Table: table} }
		}
		return m.toggle(n)
	case "s":
		n := m.selected()
		if n != nil && n.Kind == KindTable {
			return func() tea.Msg {
				return SelectTableMsg{Schema: n.Schema, Table: n.Table}
			}
		}
	case "r":
		if n := m.selected(); n != nil {
			return m.reload(n)
		}
	}
	return nil
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
		return loadSchemas(m.conn, n)
	case KindSchema:
		n.Loading = true
		m.rebuild()
		return loadTables(m.conn, n)
	case KindTable:
		n.Loading = true
		m.rebuild()
		return loadColumns(m.conn, n)
	}
	return nil
}

func (m *Model) reload(n *Node) tea.Cmd {
	switch n.Kind {
	case KindConnection:
		if m.conn == nil || m.conn.Name() != n.ConnName {
			return nil
		}
	case KindSchema, KindTable:
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
}

func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if len(m.flat) == 0 {
		return textutil.FitLines(theme.Muted.Render(m.t.T(i18n.KeyNoConnections)), m.width, m.height)
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	maxOffset := max(0, len(m.flat)-m.height)
	if m.offset > maxOffset {
		m.offset = maxOffset
	}

	lines := make([]string, 0, m.height)
	for i := m.offset; i < len(m.flat) && len(lines) < m.height; i++ {
		lines = append(lines, m.renderNode(m.flat[i], i == m.cursor))
	}
	return textutil.FitLines(strings.Join(lines, "\n"), m.width, m.height)
}

func (m *Model) renderNode(n *Node, selected bool) string {
	icon := "  "
	switch n.Kind {
	case KindConnection:
		if n.ConnName == m.connName {
			icon = "● "
		} else {
			icon = "○ "
		}
	case KindSchema, KindTable:
		if n.Expanded {
			icon = "▾ "
		} else {
			icon = "▸ "
		}
	case KindColumn:
		icon = "· "
	}
	text := strings.Repeat("  ", n.Depth) + icon + n.Label
	if n.Sub != "" {
		text += "  " + n.Sub
	}
	if n.Loading {
		text += " …"
	}
	text = textutil.FitLine(text, m.width)
	if selected {
		return theme.Selected.Render(text)
	}
	if n.Kind == KindColumn {
		return theme.Muted.Render(text)
	}
	return text
}

func loadSchemas(conn db.Conn, node *Node) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		schemas, err := conn.Catalog().Schemas(ctx)
		return SchemasLoadedMsg{Node: node, Schemas: schemas, Err: err}
	}
}

func loadTables(conn db.Conn, node *Node) tea.Cmd {
	schema := node.Schema
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		objects, err := conn.Catalog().Tables(ctx, schema)
		return TablesLoadedMsg{Node: node, Objects: objects, Err: err}
	}
}

func loadColumns(conn db.Conn, node *Node) tea.Cmd {
	schema, table := node.Schema, node.Table
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cols, err := conn.Catalog().Columns(ctx, schema, table)
		return ColumnsLoadedMsg{Node: node, Columns: cols, Err: err}
	}
}

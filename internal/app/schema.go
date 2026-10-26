package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/db/ddl"
	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/ui/form"
	"github.com/LinduanTof/b-db/internal/ui/menu"
)

const schemaTimeout = 30 * time.Second

type schemaDoneMsg struct {
	id      uint64
	message string
	reload  bool
	err     error
}

type indexesMenuMsg struct {
	id      uint64
	schema  string
	table   string
	indexes []db.Index
	err     error
}

// handleFormSubmit routes the generic form by the id it was opened with.
func (m *Model) handleFormSubmit(msg form.SubmittedMsg) tea.Cmd {
	switch msg.ID {
	case "insertrow":
		return m.submitInsertRow(msg.Values)
	case "import":
		return m.submitImportForm(msg.Values, msg.Choices)
	case "rename":
		return m.submitRenameTable(msg.Values)
	case "addcolumn":
		return m.submitAddColumn(msg.Values, msg.Choices)
	case "createindex":
		return m.submitCreateIndex(msg.Values, msg.Choices)
	case "createtable":
		return m.submitCreateTable(msg.Values)
	}
	return nil
}

// ddlAllowed rejects schema changes for read-only sessions and non-SQL
// connections, so the generated statements are always valid for the dialect.
func (m *Model) ddlAllowed() bool {
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return false
	}
	if m.readOnly() {
		m.setStatus(m.t.T(i18n.KeyErrorReadOnly), true)
		return false
	}
	if !sqlExportable(m.conn.Type()) {
		m.setStatus(m.t.T(i18n.KeyErrorUnsupportedFeature, m.conn.Type()), true)
		return false
	}
	return true
}

func (m *Model) runSchemaChange(stmt, message string, reload bool) tea.Cmd {
	conn := m.conn
	id := m.connectSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), schemaTimeout)
		defer cancel()
		_, err := conn.Querier().Exec(ctx, stmt)
		return schemaDoneMsg{id: id, message: message, reload: reload, err: err}
	}
}

func (m *Model) handleSchemaDone(msg schemaDoneMsg) tea.Cmd {
	if msg.id != m.connectSeq {
		return nil
	}
	if msg.err != nil {
		m.setStatus(m.t.T(i18n.KeyErrorDDLOperation, msg.err), true)
		return nil
	}
	m.setStatus(msg.message, false)
	if !msg.reload {
		return nil
	}
	var cmds []tea.Cmd
	if m.context == contextTables {
		cmds = append(cmds, m.loadOverviewCmd())
	}
	if cmd := m.sidebar.ReloadSession(m.connName); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

// openManageMenu keeps schema-changing actions in a submenu so the table menu
// stays short on small terminals.
func (m *Model) openManageMenu(schema, table string) {
	if !m.ddlAllowed() {
		return
	}
	m.tableMenu.Open(fmt.Sprintf("%s · %s", m.t.T(i18n.KeyMenuManageTable), table), []menu.Item{
		{ID: "table.droptable", LabelKey: i18n.KeyMenuDropTable, Kind: menu.KindAction},
		{ID: "table.truncate", LabelKey: i18n.KeyMenuTruncateTable, Kind: menu.KindAction},
		{ID: "table.rename", LabelKey: i18n.KeyMenuRenameTable, Kind: menu.KindAction},
		{ID: "table.addcolumn", LabelKey: i18n.KeyMenuAddColumn, Kind: menu.KindAction},
		{ID: "table.dropcolumn", LabelKey: i18n.KeyMenuDropColumn, Kind: menu.KindAction},
		{ID: "table.createindex", LabelKey: i18n.KeyMenuCreateIndex, Kind: menu.KindAction},
		{ID: "table.dropindex", LabelKey: i18n.KeyMenuDropIndex, Kind: menu.KindAction},
	})
}

func (m *Model) confirmDropTable(schema, table string) tea.Cmd {
	if !m.ddlAllowed() {
		return nil
	}
	stmt := ddl.DropTable(m.conn.Type(), schema, table)
	m.pendingCmd = m.runSchemaChange(stmt, m.t.T(i18n.KeyStatusTableDropped, table), true)
	m.confirm.Open(m.t.T(i18n.KeyConfirmDropTable), stmt)
	return nil
}

func (m *Model) confirmTruncateTable(schema, table string) tea.Cmd {
	if !m.ddlAllowed() {
		return nil
	}
	stmt := ddl.TruncateTable(m.conn.Type(), schema, table)
	m.pendingCmd = m.runSchemaChange(stmt, m.t.T(i18n.KeyStatusTableTruncated, table), true)
	m.confirm.Open(m.t.T(i18n.KeyConfirmTruncateTable), stmt)
	return nil
}

func (m *Model) confirmDropColumn(schema, table, column string) tea.Cmd {
	if !m.ddlAllowed() {
		return nil
	}
	stmt := ddl.DropColumn(m.conn.Type(), schema, table, column)
	m.pendingCmd = m.runSchemaChange(stmt, m.t.T(i18n.KeyStatusColumnDropped, column), true)
	m.confirm.Open(m.t.T(i18n.KeyConfirmDropColumn), stmt)
	return nil
}

func (m *Model) confirmDropIndex(schema, table, name string) tea.Cmd {
	if !m.ddlAllowed() {
		return nil
	}
	stmt := ddl.DropIndex(m.conn.Type(), schema, table, name)
	m.pendingCmd = m.runSchemaChange(stmt, m.t.T(i18n.KeyStatusIndexDropped, name), true)
	m.confirm.Open(m.t.T(i18n.KeyConfirmDropIndex), stmt)
	return nil
}

func (m *Model) openRenameTableForm(schema, table string) tea.Cmd {
	if !m.ddlAllowed() {
		return nil
	}
	m.formTarget = db.TableRef{Schema: schema, Name: table}
	m.form.Open("rename", fmt.Sprintf(m.t.T(i18n.KeyMenuRenameTable)+" · %s", table), "", []form.Field{
		{Label: m.t.T(i18n.KeyFormNewName), Value: table},
	})
	return nil
}

func (m *Model) submitRenameTable(values []string) tea.Cmd {
	name := strings.TrimSpace(firstValue(values))
	if name == "" || name == m.formTarget.Name {
		m.form.SetError(m.t.T(i18n.KeyFormNewName) + ": " + name)
		return nil
	}
	stmt := ddl.RenameTable(m.conn.Type(), m.formTarget.Schema, m.formTarget.Name, name)
	m.pendingCmd = m.runSchemaChange(stmt, m.t.T(i18n.KeyStatusTableRenamed, name), true)
	m.confirm.Open(m.t.T(i18n.KeyMenuRenameTable), stmt)
	return nil
}

func (m *Model) openAddColumnForm(schema, table string) tea.Cmd {
	if !m.ddlAllowed() {
		return nil
	}
	m.formTarget = db.TableRef{Schema: schema, Name: table}
	m.form.Open("addcolumn", fmt.Sprintf("%s · %s", m.t.T(i18n.KeyMenuAddColumn), table), "", []form.Field{
		{Label: m.t.T(i18n.KeyFormColumnName)},
		{Label: m.t.T(i18n.KeyFormColumnType), Placeholder: "TEXT"},
		{Label: m.t.T(i18n.KeyFormColumnNullable), Choices: []string{
			m.t.T(i18n.KeyImportYes), m.t.T(i18n.KeyImportNo),
		}},
		{Label: m.t.T(i18n.KeyFormColumnDefault)},
	})
	return nil
}

func (m *Model) submitAddColumn(values []string, choices []int) tea.Cmd {
	name := strings.TrimSpace(valueAt(values, 0))
	dataType := strings.TrimSpace(valueAt(values, 1))
	if name == "" || dataType == "" {
		m.form.SetError(m.t.T(i18n.KeyErrorRequiredColumn, m.t.T(i18n.KeyFormColumnName)))
		return nil
	}
	nullable := len(choices) > 2 && choices[2] == 0
	stmt := ddl.AddColumn(m.conn.Type(), m.formTarget.Schema, m.formTarget.Name, name, dataType, nullable, valueAt(values, 3))
	m.pendingCmd = m.runSchemaChange(stmt, m.t.T(i18n.KeyStatusColumnAdded, name), true)
	m.confirm.Open(m.t.T(i18n.KeyMenuAddColumn), stmt)
	return nil
}

func (m *Model) openDropColumnMenu(schema, table string, columns []db.Column) tea.Cmd {
	if !m.ddlAllowed() {
		return nil
	}
	items := []menu.Item{{Kind: menu.KindHeader, LabelKey: i18n.KeyMenuDropColumn}}
	for _, column := range columns {
		items = append(items, menu.Item{
			ID:    "table.dropcolumn." + column.Name,
			Label: column.Name,
			Kind:  menu.KindAction,
		})
	}
	m.tableMenu.Open(fmt.Sprintf("%s · %s", m.t.T(i18n.KeyMenuDropColumn), table), items)
	return nil
}

func (m *Model) openCreateIndexForm(schema, table string) tea.Cmd {
	if !m.ddlAllowed() {
		return nil
	}
	m.formTarget = db.TableRef{Schema: schema, Name: table}
	m.form.Open("createindex", fmt.Sprintf("%s · %s", m.t.T(i18n.KeyMenuCreateIndex), table), "", []form.Field{
		{Label: m.t.T(i18n.KeyFormIndexName)},
		{Label: m.t.T(i18n.KeyFormIndexColumns), Placeholder: "col1, col2"},
		{Label: m.t.T(i18n.KeyFormIndexUnique), Choices: []string{
			m.t.T(i18n.KeyImportYes), m.t.T(i18n.KeyImportNo),
		}},
	})
	return nil
}

func (m *Model) submitCreateIndex(values []string, choices []int) tea.Cmd {
	name := strings.TrimSpace(valueAt(values, 0))
	columns := splitColumnList(valueAt(values, 1))
	if name == "" || len(columns) == 0 {
		m.form.SetError(m.t.T(i18n.KeyErrorRequiredColumn, m.t.T(i18n.KeyFormIndexColumns)))
		return nil
	}
	unique := len(choices) > 2 && choices[2] == 0
	stmt := ddl.CreateIndex(m.conn.Type(), m.formTarget.Schema, m.formTarget.Name, name, columns, unique)
	m.pendingCmd = m.runSchemaChange(stmt, m.t.T(i18n.KeyStatusIndexCreated, name), true)
	m.confirm.Open(m.t.T(i18n.KeyMenuCreateIndex), stmt)
	return nil
}

func (m *Model) openDropIndexMenu(schema, table string) tea.Cmd {
	if !m.ddlAllowed() {
		return nil
	}
	conn := m.conn
	id := m.connectSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), schemaTimeout)
		defer cancel()
		indexes, err := conn.Catalog().Indexes(ctx, schema, table)
		return indexesMenuMsg{id: id, schema: schema, table: table, indexes: indexes, err: err}
	}
}

func (m *Model) handleIndexesMenu(msg indexesMenuMsg) tea.Cmd {
	if msg.id != m.connectSeq {
		return nil
	}
	if msg.err != nil {
		m.setStatus(m.t.T(i18n.KeyErrorIndexes, msg.err), true)
		return nil
	}
	if len(msg.indexes) == 0 {
		m.setStatus(m.t.T(i18n.KeyMenuDropIndex)+": -", false)
		return nil
	}
	items := []menu.Item{{Kind: menu.KindHeader, LabelKey: i18n.KeyMenuDropIndex}}
	for _, index := range msg.indexes {
		items = append(items, menu.Item{
			ID:    "table.dropindex." + index.Name,
			Label: index.Name,
			Kind:  menu.KindAction,
		})
	}
	m.tableMenu.Open(fmt.Sprintf("%s · %s", m.t.T(i18n.KeyMenuDropIndex), msg.table), items)
	return nil
}

func (m *Model) openCreateTableForm(session string) tea.Cmd {
	if !m.ddlAllowed() {
		return nil
	}
	typ := ""
	switch {
	case m.conn != nil && session == m.connName:
		typ = m.conn.Type()
	case m.cfg.Find(sessionBase(session)) != nil:
		typ = m.cfg.Find(sessionBase(session)).Type
	}
	schema := "public"
	switch typ {
	case config.TypeMySQL:
		schema = sessionDatabase(session)
	case config.TypeSQLite:
		schema = "main"
	}
	m.formTarget = db.TableRef{Schema: schema}
	m.form.Open("createtable", m.t.T(i18n.KeyMenuCreateTable), "", []form.Field{
		{Label: m.t.T(i18n.KeyFormTableName), Placeholder: "users"},
		{Label: m.t.T(i18n.KeyFormTableColumns), Placeholder: "id INTEGER PRIMARY KEY, name TEXT"},
	})
	return nil
}

func (m *Model) submitCreateTable(values []string) tea.Cmd {
	name := strings.TrimSpace(valueAt(values, 0))
	body := strings.TrimSpace(valueAt(values, 1))
	if name == "" || body == "" {
		m.form.SetError(m.t.T(i18n.KeyErrorRequiredColumn, m.t.T(i18n.KeyFormTableName)))
		return nil
	}
	stmt := ddl.CreateTable(m.conn.Type(), m.formTarget.Schema, name, body)
	m.pendingCmd = m.runSchemaChange(stmt, m.t.T(i18n.KeyStatusTableCreated, name), true)
	m.confirm.Open(m.t.T(i18n.KeyMenuCreateTable), stmt)
	return nil
}

func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func valueAt(values []string, index int) string {
	if index < 0 || index >= len(values) {
		return ""
	}
	return values[index]
}

func splitColumnList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if column := strings.TrimSpace(part); column != "" {
			out = append(out, column)
		}
	}
	return out
}

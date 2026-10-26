package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/dbrix/internal/db"
	"github.com/LinduanTof/dbrix/internal/i18n"
	"github.com/LinduanTof/dbrix/internal/ui/form"
)

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
	id := m.connectSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		columns, err := conn.Catalog().Columns(ctx, schema, table)
		if err != nil {
			return editInfoMsg{id: id, schema: schema, table: table, action: action, err: err}
		}
		primary := make([]string, 0, len(columns))
		for _, column := range columns {
			if column.PrimaryKey {
				primary = append(primary, column.Name)
			}
		}
		return editInfoMsg{id: id, schema: schema, table: table, action: action, columns: columns, primary: primary}
	}
}

func (m *Model) handleEditInfo(msg editInfoMsg) tea.Cmd {
	if msg.id != m.connectSeq {
		return nil
	}
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
	case "rowform":
		return m.openInsertRowForm(schema, table, info.columns)
	case "import":
		m.formColumns = info.columns
		return m.openImportForm(schema, table)
	case "dropcolumnmenu":
		return m.openDropColumnMenu(schema, table, info.columns)
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
	raw, ok := m.results.CursorValue()
	if !ok {
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
		wasNull:  raw == nil,
	}
	title := fmt.Sprintf(m.t.T(i18n.KeyCellEditTitle), qualifiedTable(schema, table)+"."+column)
	m.cellEdit.Open(title, editableText(raw))
}

func editableText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(x)
	}
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
	value, err := cellEditValue(raw, target)
	if err != nil {
		m.setStatus(m.t.T(i18n.KeyErrorInvalidValue, strings.TrimSpace(raw), target.dataType), true)
		return nil
	}
	m.pendingCmd = m.mutationCmd("update", func(ctx context.Context, mut db.Mutator) (int64, error) {
		return mut.UpdateCell(ctx, target.table, target.key, target.column, value)
	})
	m.confirm.Open(m.t.T(i18n.KeyConfirmUpdate), mut.PreviewUpdate(target.table, target.key, target.column, value))
	return nil
}

func cellEditValue(raw string, target editTarget) (any, error) {
	if target.wasNull && raw == "" {
		return nil, nil
	}
	if strings.EqualFold(strings.TrimSpace(raw), "NULL") {
		return nil, nil
	}
	return parseCellValue(raw, target.dataType)
}

func (m *Model) mutationCmd(kind string, run func(context.Context, db.Mutator) (int64, error)) tea.Cmd {
	mut := m.conn.Mutator()
	id := m.connectSeq
	ctx, cancel := context.WithCancel(context.Background())
	m.mutationCancel = cancel
	return func() tea.Msg {
		defer cancel()
		ctx, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
		defer cancelTimeout()
		affected, err := run(ctx, mut)
		return mutationDoneMsg{id: id, kind: kind, affected: affected, err: err}
	}
}

func (m *Model) cancelMutation() {
	if m.mutationCancel != nil {
		m.mutationCancel()
		m.mutationCancel = nil
	}
}

func (m *Model) handleMutationDone(msg mutationDoneMsg) tea.Cmd {
	if msg.id != m.connectSeq {
		return nil
	}
	m.cancelMutation()
	if msg.err != nil {
		if msg.kind == "insert" {
			m.setStatus(m.t.T(i18n.KeyErrorInsert, msg.err), true)
		} else {
			m.setStatus(m.t.T(i18n.KeyErrorMutation, msg.err), true)
		}
		return nil
	}
	switch msg.kind {
	case "delete":
		m.setStatus(m.t.T(i18n.KeyStatusDeleted), false)
	case "insert":
		m.setStatus(m.t.T(i18n.KeyStatusInserted, msg.affected), false)
		if msg.table != nil && m.pager != nil &&
			m.pager.schema == msg.table.Schema && m.pager.table == msg.table.Name {
			return m.refreshSource()
		}
		return nil
	default:
		m.setStatus(m.t.T(i18n.KeyStatusUpdated), false)
	}
	return m.refreshSource()
}

func (m *Model) refreshSource() tea.Cmd {
	schema, table, ok := m.results.Source()
	if !ok {
		return nil
	}
	if m.pager != nil && m.pager.schema == schema && m.pager.table == table {
		page := *m.pager
		page.refresh = true
		page.totalKnown = false
		return m.runPageQuery(&page)
	}
	return m.runPageQuery(&pagination{
		schema: schema,
		table:  table,
		size:   max(1, m.cfg.Query.MaxRows),
	})
}

func (m *Model) insertTemplate(schema, table string, columns []db.Column) tea.Cmd {
	names := make([]string, 0, len(columns))
	values := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, column.Name)
		values = append(values, "NULL")
	}
	stmt := fmt.Sprintf(m.t.T(i18n.KeyQueryInsertTemplate),
		qualifiedTable(schema, table),
		strings.Join(names, ",\n  "),
		strings.Join(values, ",\n  "),
	)
	m.editor.SetValue(stmt)
	m.focus = focusEditor
	m.layout()
	return tea.Batch(m.editor.Focus(), m.scheduleTabSave())
}

func parseCellValue(raw, dataType string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	kind := strings.ToLower(dataType)
	switch {
	case isIntegerType(kind), strings.Contains(kind, "serial"):
		value, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return nil, err
		}
		return value, nil
	case strings.Contains(kind, "numeric"), strings.Contains(kind, "decimal"),
		strings.Contains(kind, "real"), strings.Contains(kind, "double"), strings.Contains(kind, "float"):
		value, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return nil, err
		}
		return value, nil
	case strings.Contains(kind, "bool"):
		value, err := strconv.ParseBool(trimmed)
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	return raw, nil
}

func isIntegerType(kind string) bool {
	return strings.Contains(kind, "int") && !strings.Contains(kind, "interval")
}

// openInsertRowForm builds one field per column. Leaving a field empty omits
// the column so its default applies, which keeps DEFAULT and nullable columns
// out of the generated INSERT.
func (m *Model) openInsertRowForm(schema, table string, columns []db.Column) tea.Cmd {
	if m.readOnly() {
		m.setStatus(m.t.T(i18n.KeyErrorReadOnly), true)
		return nil
	}
	m.formTarget = db.TableRef{Schema: schema, Name: table}
	m.formColumns = columns
	fields := make([]form.Field, len(columns))
	for i, column := range columns {
		note := column.DataType
		if column.Default != "" {
			note += " default " + column.Default
		}
		if !column.Nullable {
			note += " not null"
		}
		fields[i] = form.Field{Label: column.Name, Placeholder: note}
	}
	title := fmt.Sprintf(m.t.T(i18n.KeyInsertRowTitle), qualifiedTable(schema, table))
	m.form.Open("insertrow", title, m.t.T(i18n.KeyInsertRowNote), fields)
	return nil
}

func (m *Model) submitInsertRow(values []string) tea.Cmd {
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	inserter, ok := m.conn.Mutator().(db.Inserter)
	if !ok {
		m.setStatus(m.t.T(i18n.KeyErrorUnsupportedFeature, m.conn.Type()), true)
		return nil
	}
	primary := 0
	for _, column := range m.formColumns {
		if column.PrimaryKey {
			primary++
		}
	}
	names := make([]string, 0, len(m.formColumns))
	row := make([]any, 0, len(m.formColumns))
	for i, column := range m.formColumns {
		raw := ""
		if i < len(values) {
			raw = values[i]
		}
		value, include, err := insertValue(raw, column)
		if err != nil {
			m.form.SetError(m.t.T(i18n.KeyErrorInvalidValue, strings.TrimSpace(raw), column.DataType))
			return nil
		}
		if !include {
			// A single integer primary key is generated by the database when
			// it is left empty, so it is not required in the form.
			autoKey := primary == 1 && column.PrimaryKey &&
				isIntegerType(strings.ToLower(column.DataType))
			if !column.Nullable && column.Default == "" && !autoKey {
				m.form.SetError(m.t.T(i18n.KeyErrorRequiredColumn, column.Name))
				return nil
			}
			continue
		}
		names = append(names, column.Name)
		row = append(row, value)
	}
	if len(names) == 0 {
		m.form.SetError(m.t.T(i18n.KeyErrorNoColumns))
		return nil
	}
	table := m.formTarget
	rows := [][]any{row}
	m.pendingCmd = m.insertCmd(table, names, rows, time.Minute)
	m.confirm.Open(m.t.T(i18n.KeyConfirmInsert), inserter.PreviewInsert(table, names, rows))
	return nil
}

// insertValue reports whether the value should be sent to the database.
// An empty input omits the column; NULL and ” are explicit values.
func insertValue(raw string, column db.Column) (any, bool, error) {
	trimmed := strings.TrimSpace(raw)
	switch {
	case trimmed == "":
		return nil, false, nil
	case strings.EqualFold(trimmed, "NULL"):
		return nil, true, nil
	case trimmed == `''`:
		return "", true, nil
	}
	value, err := parseCellValue(trimmed, column.DataType)
	return value, true, err
}

func (m *Model) insertCmd(table db.TableRef, columns []string, rows [][]any, timeout time.Duration) tea.Cmd {
	conn := m.conn
	id := m.connectSeq
	ctx, cancel := context.WithCancel(context.Background())
	m.mutationCancel = cancel
	return func() tea.Msg {
		defer cancel()
		ctx, cancelTimeout := context.WithTimeout(ctx, timeout)
		defer cancelTimeout()
		inserter, ok := conn.Mutator().(db.Inserter)
		if !ok {
			return mutationDoneMsg{id: id, kind: "insert", err: fmt.Errorf("insert is not supported for %s", conn.Type())}
		}
		affected, err := inserter.InsertRows(ctx, table, columns, db.NewSliceRows(rows), nil)
		target := table
		return mutationDoneMsg{id: id, kind: "insert", affected: affected, table: &target, err: err}
	}
}

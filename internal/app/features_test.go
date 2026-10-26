package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/ui/sidebar"
)

func queryInt(t *testing.T, m *Model, stmt string) int64 {
	t.Helper()
	rs, err := m.conn.Querier().Query(context.Background(), stmt, 1)
	if err != nil {
		t.Fatalf("query %q: %v", stmt, err)
	}
	if rs == nil || len(rs.Rows) == 0 || len(rs.Rows[0]) == 0 {
		t.Fatalf("query %q returned no rows", stmt)
	}
	value, ok := rs.Rows[0][0].(int64)
	if !ok {
		t.Fatalf("query %q returned %T", stmt, rs.Rows[0][0])
	}
	return value
}

func TestInsertRowForm(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	m = runCmd(t, m, m.editInfoCmd("main", "users", "rowform"))
	if !m.form.IsOpen() {
		t.Fatalf("insert form should open: %s", m.status)
	}
	if len(m.formColumns) == 0 {
		t.Fatal("form columns were not loaded")
	}
	values := make([]string, len(m.formColumns))
	for i, column := range m.formColumns {
		switch column.Name {
		case "name":
			values[i] = "New User"
		case "email":
			values[i] = "new@example.com"
		case "note":
			values[i] = "hello"
		}
	}
	m.submitInsertRow(values)
	if !m.confirm.IsOpen() {
		t.Fatalf("insert should ask for confirmation: %s", m.status)
	}
	if preview := ansi.Strip(m.confirm.View()); !strings.Contains(preview, "INSERT INTO") {
		t.Fatalf("confirm preview should show the INSERT:\n%s", preview)
	}
	m = runCmd(t, m, m.pendingCmd)
	m.pendingCmd = nil

	if got := queryInt(t, m, "SELECT COUNT(*) FROM users WHERE email = 'new@example.com'"); got != 1 {
		t.Fatalf("inserted rows = %d, want 1 (status: %s)", got, m.status)
	}
	if got := queryInt(t, m, "SELECT COUNT(*) FROM users WHERE email = 'new@example.com' AND note = 'hello'"); got != 1 {
		t.Fatalf("note value was not inserted (status: %s)", m.status)
	}
}

func TestInsertRowRejectsMissingRequiredColumn(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	m = runCmd(t, m, m.editInfoCmd("main", "users", "rowform"))
	m.submitInsertRow(make([]string, len(m.formColumns)))
	if m.confirm.IsOpen() {
		t.Fatal("missing required columns must not open the confirm dialog")
	}
	if !m.form.IsOpen() {
		t.Fatal("the form should stay open with an error")
	}
	if !strings.Contains(ansi.Strip(m.form.View()), "required") {
		t.Fatalf("expected a required-field error:\n%s", ansi.Strip(m.form.View()))
	}
}

func TestImportCSVFlow(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	path := filepath.Join(t.TempDir(), "users.csv")
	if err := os.WriteFile(path, []byte("name,email,active\nImport One,one@example.com,1\nImport Two,two@example.com,0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m = runCmd(t, m, m.editInfoCmd("main", "users", "import"))
	if !m.form.IsOpen() {
		t.Fatalf("import form should open: %s", m.status)
	}
	cmd := m.submitImportForm([]string{path, "", ""}, []int{0, 0, 0})
	m.form.Close() // the real form closes itself on submit
	m = runCmd(t, m, cmd)
	if !m.confirm.IsOpen() {
		t.Fatalf("import should ask for confirmation: %s", m.status)
	}
	if preview := ansi.Strip(m.confirm.View()); !strings.Contains(preview, "one@example.com") {
		t.Fatalf("confirm preview should show the rows:\n%s", preview)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := queryInt(t, m, "SELECT COUNT(*) FROM users WHERE email IN ('one@example.com', 'two@example.com')"); got != 2 {
		t.Fatalf("imported rows = %d, want 2 (status: %s)", got, m.status)
	}
	if got := queryInt(t, m, "SELECT COUNT(*) FROM users WHERE email = 'two@example.com' AND active = 0"); got != 1 {
		t.Fatalf("active value was not converted (status: %s)", m.status)
	}
}

func TestImportRejectsUnknownColumn(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	path := filepath.Join(t.TempDir(), "bad.csv")
	if err := os.WriteFile(path, []byte("name,nope\nalice,x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m = runCmd(t, m, m.editInfoCmd("main", "users", "import"))
	cmd := m.submitImportForm([]string{path, "", ""}, []int{0, 0, 0})
	m.form.Close() // the real form closes itself on submit
	m = runCmd(t, m, cmd)
	if m.confirm.IsOpen() {
		t.Fatal("unknown columns must not open the confirm dialog")
	}
	if !strings.Contains(m.status, "nope") {
		t.Fatalf("status should name the unknown column: %s", m.status)
	}
}

func TestDropTableFlow(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	if _, err := m.conn.Querier().Exec(context.Background(), "CREATE TABLE tmp_drop (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	m.confirmDropTable("main", "tmp_drop")
	if !m.confirm.IsOpen() {
		t.Fatalf("drop should ask for confirmation: %s", m.status)
	}
	if preview := ansi.Strip(m.confirm.View()); !strings.Contains(preview, "DROP TABLE") {
		t.Fatalf("confirm preview should show the DDL:\n%s", preview)
	}
	m = runCmd(t, m, m.pendingCmd)
	m.pendingCmd = nil

	if got := queryInt(t, m, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'tmp_drop'"); got != 0 {
		t.Fatalf("table still exists after drop (status: %s)", m.status)
	}
}

func TestTruncateTableFlow(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	m.confirmTruncateTable("main", "orders")
	if !m.confirm.IsOpen() {
		t.Fatalf("truncate should ask for confirmation: %s", m.status)
	}
	m = runCmd(t, m, m.pendingCmd)
	m.pendingCmd = nil

	if got := queryInt(t, m, "SELECT COUNT(*) FROM orders"); got != 0 {
		t.Fatalf("orders rows = %d, want 0 (status: %s)", got, m.status)
	}
}

func TestRenameTableForm(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	m.openRenameTableForm("main", "orders")
	if !m.form.IsOpen() {
		t.Fatalf("rename form should open: %s", m.status)
	}
	m.submitRenameTable([]string{"orders_renamed"})
	if !m.confirm.IsOpen() {
		t.Fatalf("rename should ask for confirmation: %s", m.status)
	}
	if preview := ansi.Strip(m.confirm.View()); !strings.Contains(preview, "RENAME TO") {
		t.Fatalf("confirm preview should show the DDL:\n%s", preview)
	}
	m = runCmd(t, m, m.pendingCmd)
	m.pendingCmd = nil
	if got := queryInt(t, m, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'orders_renamed'"); got != 1 {
		t.Fatalf("renamed table missing (status: %s)", m.status)
	}
}

func TestCreateIndexForm(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	m.openCreateIndexForm("main", "orders")
	m.submitCreateIndex([]string{"idx_orders_amount", "amount", ""}, []int{0, 0, 1})
	if !m.confirm.IsOpen() {
		t.Fatalf("create index should ask for confirmation: %s", m.status)
	}
	m = runCmd(t, m, m.pendingCmd)
	m.pendingCmd = nil
	if got := queryInt(t, m, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_orders_amount'"); got != 1 {
		t.Fatalf("index missing (status: %s)", m.status)
	}
}

func TestSchemaChangeRejectedReadOnly(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()
	m.cfg.Connections[0].ReadOnly = true

	m.confirmDropTable("main", "orders")
	if m.confirm.IsOpen() {
		t.Fatal("read-only connections must not open the confirm dialog")
	}
	if got := queryInt(t, m, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'orders'"); got != 1 {
		t.Fatal("the table must remain")
	}
}

func TestResultFilterKeys(t *testing.T) {
	m := testModel(t, "en")
	m.results.SetResult(&db.ResultSet{
		Columns: []db.ColumnMeta{{Name: "name"}},
		Rows:    [][]any{{"alice"}, {"bob"}, {"alicia"}},
	})
	m.context = contextData
	m.focus = focusResults

	m = sendKey(t, m, tea.KeyPressMsg{Text: "/", Code: '/'})
	if !m.filtering {
		t.Fatal("slash should start filtering results")
	}
	for _, r := range "ali" {
		m = sendKey(t, m, tea.KeyPressMsg{Text: string(r), Code: r})
	}
	if rs := m.results.OrderedResult(); len(rs.Rows) != 2 {
		t.Fatalf("filtered rows = %d, want 2", len(rs.Rows))
	}
	if !strings.Contains(ansi.Strip(m.results.View()), "2/3") {
		t.Fatalf("filter count missing from the results footer:\n%s", ansi.Strip(m.results.View()))
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.filtering {
		t.Fatal("esc should stop filtering")
	}
	if rs := m.results.OrderedResult(); len(rs.Rows) != 3 {
		t.Fatalf("filter should be cleared, rows = %d", len(rs.Rows))
	}
}

func TestExplainQuery(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	m.editor.SetValue("SELECT * FROM users")
	m.focus = focusEditor
	m = sendKey(t, m, tea.KeyPressMsg{Code: 'e', Mod: tea.ModAlt})
	rs := m.results.Result()
	if rs == nil {
		t.Fatalf("explain returned no result: %s", m.status)
	}
	if len(rs.Rows) == 0 {
		t.Fatalf("explain returned no rows: %s", m.status)
	}
	if m.context != contextQuery {
		t.Fatalf("context = %v, want query", m.context)
	}
}

func TestEditorFindKeys(t *testing.T) {
	m := testModel(t, "en")
	m.focus = focusEditor
	m.editor.SetValue("select 1\nselect 2")
	m = sendKey(t, m, tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if !m.editor.Finding() {
		t.Fatal("ctrl+f should open the find prompt")
	}
	for _, r := range "select" {
		m = sendKey(t, m, tea.KeyPressMsg{Text: string(r), Code: r})
	}
	if _, current, total := m.editor.FindStatus(); current != 1 || total != 2 {
		t.Fatalf("find status = %d/%d, want 1/2", current, total)
	}
	if hint := ansi.Strip(m.View().Content); !strings.Contains(hint, "Find: select") {
		t.Fatalf("find prompt missing from the editor hint:\n%s", hint)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.editor.Finding() {
		t.Fatal("esc should close the find prompt")
	}
}

func TestFormOverlayIsRendered(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	m.openCreateTableForm(m.connName)
	if !m.form.IsOpen() {
		t.Fatalf("form should open: %s", m.status)
	}
	content := ansi.Strip(m.View().Content)
	// A form that is open but not rendered swallows every key press, which
	// looks like a frozen UI.
	if !strings.Contains(content, "Create table") {
		t.Fatalf("open form is missing from the rendered view:\n%s", content)
	}
	if !strings.Contains(content, "Table") || !strings.Contains(content, "Columns") {
		t.Fatalf("form fields are missing from the view:\n%s", content)
	}
}

func TestFormOverlayCloses(t *testing.T) {
	m := testModel(t, "en")
	m.openCreateTableForm(m.connName)
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.form.IsOpen() {
		t.Fatal("esc should close the form")
	}
	if strings.Contains(ansi.Strip(m.View().Content), "Columns") {
		t.Fatal("form contents should disappear after closing")
	}
}

func TestInsertTemplateIsMultiline(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	m = runCmd(t, m, m.editInfoCmd("main", "users", "insert"))
	value := m.editor.Value()
	if !strings.Contains(value, "\n") {
		t.Fatalf("insert template should be multiline: %q", value)
	}
	if !strings.Contains(value, "INSERT INTO") || !strings.Contains(value, "id,") {
		t.Fatalf("unexpected template: %q", value)
	}
	if !strings.Contains(value, "\n  id,\n  name,") {
		t.Fatalf("columns should each be on their own line: %q", value)
	}
	wantLines := 2*len(m.editCache[editInfoKey("main", "users")].columns) + 3
	if got := len(strings.Split(value, "\n")); got != wantLines {
		t.Fatalf("template lines = %d, want %d: %q", got, wantLines, value)
	}
	if !strings.Contains(ansi.Strip(m.editor.View()), "NULL") {
		t.Fatalf("editor should render the template:\n%s", ansi.Strip(m.editor.View()))
	}
}

func TestMouseClickEditorFocuses(t *testing.T) {
	m := testModel(t, "en")
	m.editor.SetValue("select 1\nselect 2")
	m.focus = focusSidebar
	m.layout()

	x := m.sideW + 1 + 2
	model, _ := m.Update(tea.MouseClickMsg{X: x, Y: 2, Button: tea.MouseLeft})
	m = model.(*Model)
	if m.focus != focusEditor {
		t.Fatalf("focus = %v, want editor", m.focus)
	}
	if strings.Contains(ansi.Strip(m.View().Content), "0/") {
		t.Fatal("unexpected render")
	}
}

func TestMouseClickColumnsLayout(t *testing.T) {
	m := testModel(t, "en")
	m.layoutMode = config.LayoutColumns
	m.layout()
	if !m.columnsActive {
		t.Skip("terminal is not wide enough for columns")
	}
	x := m.sideW + 1 + m.editorW + 1 + 2
	model, _ := m.Update(tea.MouseClickMsg{X: x, Y: 2, Button: tea.MouseLeft})
	m = model.(*Model)
	if m.focus != focusResults {
		t.Fatalf("focus = %v, want results", m.focus)
	}

	x = m.sideW + 1 + 2
	model, _ = m.Update(tea.MouseClickMsg{X: x, Y: 2, Button: tea.MouseLeft})
	m = model.(*Model)
	if m.focus != focusEditor {
		t.Fatalf("focus = %v, want editor", m.focus)
	}
}

func TestMouseClickResultsTopLayout(t *testing.T) {
	m := testModel(t, "en")
	m.layoutMode = config.LayoutResultsTop
	m.layout()

	x := m.sideW + 1 + 2
	y := m.resultsH + 1 + 2
	model, _ := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = model.(*Model)
	if m.focus != focusEditor {
		t.Fatalf("focus = %v, want editor", m.focus)
	}
}

func TestMouseWheelScrollsEditor(t *testing.T) {
	m := testModel(t, "en")
	m.editor.SetValue(strings.Repeat("line\n", 30))
	m.focus = focusEditor
	m.layout()
	_ = m.View()
	before := ansi.Strip(m.editor.View())

	model, _ := m.Update(tea.MouseWheelMsg{X: m.sideW + 2, Y: 2, Button: tea.MouseWheelUp})
	m = model.(*Model)
	if m.focus != focusEditor {
		t.Fatalf("wheel changed focus to %v", m.focus)
	}
	if after := ansi.Strip(m.editor.View()); after == before {
		t.Fatal("wheel did not scroll the editor")
	}
}

func TestImportCancelClearsState(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	path := filepath.Join(t.TempDir(), "u.csv")
	if err := os.WriteFile(path, []byte("name,email\na,a@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m = runCmd(t, m, m.editInfoCmd("main", "users", "import"))
	cmd := m.submitImportForm([]string{path, "", ""}, []int{0, 0, 0})
	m.form.Close()
	m = runCmd(t, m, cmd)
	if !m.confirm.IsOpen() {
		t.Fatalf("confirm should open: %s", m.status)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.importing {
		t.Fatal("cancelling must clear the importing flag")
	}
	if strings.Contains(m.status, "Importing") {
		t.Fatalf("status still shows importing: %q", m.status)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if !m.options.IsOpen() {
		t.Fatal("esc should open the options menu after cancelling the import")
	}
}

func TestPasteIntoForm(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()
	m.openCreateTableForm(m.connName)
	if !m.form.IsOpen() {
		t.Fatalf("form should open: %s", m.status)
	}
	model, _ := m.Update(tea.PasteMsg{Content: "users_copy"})
	m = model.(*Model)
	if !strings.Contains(ansi.Strip(m.form.View()), "users_copy") {
		t.Fatalf("pasted text should reach the form:\n%s", ansi.Strip(m.form.View()))
	}
	if m.editor.Value() != "" {
		t.Fatalf("paste leaked into the editor: %q", m.editor.Value())
	}
}

func TestPasteIntoFind(t *testing.T) {
	m := testModel(t, "en")
	m.focus = focusEditor
	m.editor.SetValue("select 1\nselect 2")
	m = sendKey(t, m, tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	model, _ := m.Update(tea.PasteMsg{Content: "select"})
	m = model.(*Model)
	if _, _, total := m.editor.FindStatus(); total != 2 {
		t.Fatalf("pasted find query matched %d, want 2", total)
	}
}

func TestFilterOnQueryResult(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	m.editor.SetValue("SELECT 'alice' AS name UNION ALL SELECT 'bob'")
	m.focus = focusEditor
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	if m.context != contextQuery {
		t.Fatalf("context = %v, want query", m.context)
	}
	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Text: "/", Code: '/'})
	if !m.filtering {
		t.Fatal("slash should start filtering editor query results")
	}
	for _, r := range "bob" {
		m = sendKey(t, m, tea.KeyPressMsg{Text: string(r), Code: r})
	}
	rs := m.results.OrderedResult()
	if len(rs.Rows) != 1 || rs.Rows[0][0] != "bob" {
		t.Fatalf("filtered rows = %#v", rs.Rows)
	}
}

func TestFilterEndsOnMouseClick(t *testing.T) {
	m := testModel(t, "en")
	m.results.SetResult(&db.ResultSet{
		Columns: []db.ColumnMeta{{Name: "name"}},
		Rows:    [][]any{{"alice"}, {"bob"}},
	})
	m.context = contextData
	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Text: "/", Code: '/'})
	m = sendKey(t, m, tea.KeyPressMsg{Text: "a", Code: 'a'})
	if !m.filtering {
		t.Fatal("setup failed")
	}
	m.editor.SetValue("")
	x := m.sideW + 1 + 2
	model, _ := m.Update(tea.MouseClickMsg{X: x, Y: 2, Button: tea.MouseLeft})
	m = model.(*Model)
	if m.filtering {
		t.Fatal("mouse click should stop filter editing")
	}
	m = sendKey(t, m, tea.KeyPressMsg{Text: "S", Code: 'S'})
	if m.editor.Value() != "S" {
		t.Fatalf("keys should go to the editor, got editor=%q filter=%q", m.editor.Value(), m.results.Filter())
	}
	if m.results.Filter() != "a" {
		t.Fatalf("the applied filter should be kept, got %q", m.results.Filter())
	}
}

func TestFilterSurvivesPageRefresh(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()
	m = runCmd(t, m, m.tableData("main", "users"))
	if m.pager == nil {
		t.Fatalf("pager not set: %s", m.status)
	}
	m.results.SetFilter("Alice")
	if got := len(m.results.OrderedResult().Rows); got != 1 {
		t.Fatalf("filtered rows = %d, want 1", got)
	}
	m = runCmd(t, m, m.refreshSource())
	if m.results.Filter() != "Alice" {
		t.Fatalf("filter lost after refresh: %q", m.results.Filter())
	}
	if got := len(m.results.OrderedResult().Rows); got != 1 {
		t.Fatalf("filtered rows after refresh = %d, want 1", got)
	}
}

func TestEditCacheInvalidatedAfterDDL(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()

	m = runCmd(t, m, m.editInfoCmd("main", "users", "rowform"))
	before := len(m.formColumns)
	m.form.Close()

	m.openAddColumnForm("main", "users")
	m.submitAddColumn([]string{"extra", "TEXT", "", ""}, []int{0, 0, 0})
	if !m.confirm.IsOpen() {
		t.Fatalf("confirm not open: %s", m.status)
	}
	m = runCmd(t, m, m.pendingCmd)
	m.pendingCmd = nil

	m = runCmd(t, m, m.editInfoCmd("main", "users", "rowform"))
	if after := len(m.formColumns); after != before+1 {
		t.Fatalf("columns after DDL = %d, want %d", after, before+1)
	}
}

func TestCreateIndexDefaultsToNonUnique(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()
	m.openCreateIndexForm("main", "users")
	if !m.form.IsOpen() {
		t.Fatalf("form should open: %s", m.status)
	}
	if !strings.Contains(ansi.Strip(m.form.View()), "‹ No ›") {
		t.Fatalf("unique index should default to No:\n%s", ansi.Strip(m.form.View()))
	}
}

func TestCreateTableSwitchesToTargetSession(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()

	other := config.Connection{Name: "other", Type: config.TypeSQLite, Path: filepath.Join(t.TempDir(), "other.db")}
	m.cfg.Connections = append(m.cfg.Connections, other)
	m.sidebar.AddConnection(other)
	m.sidebar.SetSize(m.sideW, max(1, m.height-3))

	cmd := m.openCreateTableForm("other")
	if m.pendingCreateTable != "other" {
		t.Fatalf("pendingCreateTable = %q", m.pendingCreateTable)
	}
	m = runCmd(t, m, cmd)
	if m.connName != "other" {
		t.Fatalf("connName = %q, want other (status: %s)", m.connName, m.status)
	}
	if !m.form.IsOpen() {
		t.Fatalf("create table form should open on the target session: %s", m.status)
	}
}

func TestViewMenuHidesWriteActions(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer m.conn.Close()
	items := m.nodeMenuItems(sidebar.NodeMenuMsg{
		Kind: sidebar.KindTable, Schema: "main", Table: "active_users", Sub: "view",
	})
	for _, item := range items {
		switch item.ID {
		case "table.insert", "table.insertrow", "table.import", "table.manage":
			t.Fatalf("view menu should not offer %s", item.ID)
		}
	}
	found := false
	for _, item := range items {
		if item.ID == "table.data" {
			found = true
		}
	}
	if !found {
		t.Fatalf("view menu should still offer data preview: %#v", items)
	}
}

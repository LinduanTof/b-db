package detail

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/linduan/bdb/internal/db"
	"github.com/linduan/bdb/internal/i18n"
	"github.com/linduan/bdb/internal/ui/textutil"
	"github.com/linduan/bdb/internal/ui/theme"
)

type Model struct {
	t *i18n.Translator

	width, height int
	open          bool
	loading       bool
	title         string
	columns       []db.Column
	indexes       []db.Index
	fks           []db.ForeignKey
	errText       string
	scroll        int
}

func New(t *i18n.Translator) Model {
	return Model{t: t}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Open(schema, table string) {
	m.open = true
	m.loading = true
	m.errText = ""
	m.columns = nil
	m.indexes = nil
	m.fks = nil
	m.scroll = 0
	m.title = table
	if schema != "" && schema != "main" {
		m.title = schema + "." + table
	}
}

func (m *Model) SetData(columns []db.Column, indexes []db.Index, fks []db.ForeignKey, err error) {
	m.loading = false
	if err != nil {
		m.errText = err.Error()
		return
	}
	m.columns = columns
	m.indexes = indexes
	m.fks = fks
}

func (m *Model) Close() { m.open = false }

func (m Model) IsOpen() bool { return m.open }

func (m *Model) Scroll(delta int) {
	m.scroll += delta * 3
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m Model) Title() string { return m.title }

func (m *Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.open = false
	case "up", "k":
		m.scroll--
	case "down", "j":
		m.scroll++
	case "pgup":
		m.scroll -= m.pageSize()
	case "pgdown":
		m.scroll += m.pageSize()
	case "g":
		m.scroll = 0
	case "G":
		m.scroll = max(0, len(m.contentLines())-m.pageSize())
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	return *m, nil
}

func (m *Model) View() string {
	if !m.open || m.width <= 0 || m.height <= 0 {
		return ""
	}
	lines := m.contentLines()
	size := m.pageSize()
	maxScroll := max(0, len(lines)-size)
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	end := min(len(lines), m.scroll+size)

	out := make([]string, 0, m.height)
	out = append(out, theme.Title.Render(" "+m.t.T(i18n.KeyDetailTitle)+": "+m.title), "")
	out = append(out, lines[m.scroll:end]...)
	for len(out) < m.height-1 {
		out = append(out, "")
	}
	out = append(out, theme.Muted.Render(" "+m.t.T(i18n.KeyDetailHint)))
	return textutil.FitLines(strings.Join(out, "\n"), m.width, m.height)
}

func (m *Model) pageSize() int {
	return max(1, m.height-3)
}

func (m *Model) contentLines() []string {
	if m.errText != "" {
		return []string{"  " + theme.Error.Render(m.errText)}
	}
	if m.loading {
		return []string{"  " + theme.Muted.Render(m.t.T(i18n.KeyStatusLoading))}
	}

	var out []string
	out = append(out, theme.Header.Render(m.t.T(i18n.KeyDetailColumns)))
	if len(m.columns) == 0 {
		out = append(out, "  "+theme.Muted.Render(m.t.T(i18n.KeyDetailNone)))
	}
	for _, c := range m.columns {
		flags := ""
		if c.PrimaryKey {
			flags += " PK"
		}
		if !c.Nullable {
			flags += " NOT NULL"
		}
		def := ""
		if c.Default != "" {
			def = "  default " + c.Default
		}
		out = append(out, "  "+pad(c.Name, 22)+" "+pad(c.DataType, 20)+flags+def)
	}

	out = append(out, "", theme.Header.Render(m.t.T(i18n.KeyDetailIndexes)))
	if len(m.indexes) == 0 {
		out = append(out, "  "+theme.Muted.Render(m.t.T(i18n.KeyDetailNone)))
	}
	for _, idx := range m.indexes {
		kind := ""
		if idx.Unique {
			kind = " UNIQUE"
		}
		out = append(out, fmt.Sprintf("  %s%s (%s)", idx.Name, kind, strings.Join(idx.Columns, ", ")))
	}

	out = append(out, "", theme.Header.Render(m.t.T(i18n.KeyDetailForeignKeys)))
	if len(m.fks) == 0 {
		out = append(out, "  "+theme.Muted.Render(m.t.T(i18n.KeyDetailNone)))
	}
	for _, fk := range m.fks {
		out = append(out, fmt.Sprintf("  %s: %s -> %s(%s)", fk.Name, fk.Column, fk.RefTable, fk.RefColumn))
	}
	return out
}

func pad(s string, width int) string {
	if d := width - textutil.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

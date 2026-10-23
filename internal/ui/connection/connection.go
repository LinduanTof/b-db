package connection

import (
	"errors"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/ui/textutil"
	"github.com/LinduanTof/b-db/internal/ui/theme"
)

type SubmittedMsg struct {
	Conn config.Connection
}

type fieldKind int

const (
	fieldType fieldKind = iota
	fieldName
	fieldHost
	fieldPort
	fieldUser
	fieldPassword
	fieldDatabase
	fieldSSLMode
	fieldPath
	fieldAddr
	fieldDB
	fieldURI
)

type field struct {
	kind  fieldKind
	input textinput.Model
}

type Model struct {
	t *i18n.Translator

	width, height int
	open          bool
	connType      string
	fields        []field
	focus         int
	errText       string
}

func New(t *i18n.Translator) Model {
	m := Model{t: t, connType: config.TypePostgres}
	m.fields = []field{
		{kind: fieldType},
		{kind: fieldName, input: newInput("local")},
		{kind: fieldHost, input: newInput("127.0.0.1")},
		{kind: fieldPort, input: newInput("5432")},
		{kind: fieldUser, input: newInput("postgres")},
		{kind: fieldPassword, input: newSecret()},
		{kind: fieldDatabase, input: newInput("postgres")},
		{kind: fieldSSLMode, input: newInput("disable")},
		{kind: fieldPath, input: newInput("./data.db")},
		{kind: fieldAddr, input: newInput("127.0.0.1:6379")},
		{kind: fieldDB, input: newInput("0")},
		{kind: fieldURI, input: newInput("mongodb://127.0.0.1:27017")},
	}
	return m
}

func newInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = ""
	ti.CharLimit = 256
	ti.SetWidth(42)
	return ti
}

func newSecret() textinput.Model {
	ti := newInput("")
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	return ti
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Open() {
	m.open = true
	m.connType = config.TypePostgres
	m.focus = 0
	m.errText = ""
	for i := range m.fields {
		m.fields[i].input.SetValue("")
		m.fields[i].input.Blur()
	}
	m.setValue(fieldPort, "5432")
	m.setValue(fieldSSLMode, "disable")
}

func (m *Model) Close() { m.open = false }

func (m Model) IsOpen() bool { return m.open }

func (m *Model) SetError(text string) {
	m.errText = text
	m.open = true
}

func (m *Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !m.open {
		return *m, nil
	}
	switch msg.String() {
	case "esc":
		m.open = false
		return *m, nil
	case "up":
		m.moveFocus(-1)
		return *m, nil
	case "down":
		m.moveFocus(1)
		return *m, nil
	case "tab":
		m.moveFocus(1)
		return *m, nil
	case "shift+tab":
		m.moveFocus(-1)
		return *m, nil
	case "enter":
		if m.focus >= len(m.visibleFields())-1 {
			return m.submit()
		}
		m.moveFocus(1)
		return *m, nil
	}

	visible := m.visibleFields()
	if len(visible) == 0 {
		return *m, nil
	}
	current := visible[min(max(m.focus, 0), len(visible)-1)]
	if current.kind == fieldType {
		switch msg.String() {
		case "left", "h":
			m.cycleType(-1)
		case "right", "l":
			m.cycleType(1)
		}
		return *m, nil
	}
	var cmd tea.Cmd
	current.input, cmd = current.input.Update(msg)
	return *m, cmd
}

func (m *Model) View() string {
	if !m.open || m.width <= 0 || m.height <= 0 {
		return ""
	}

	boxW := min(80, m.width-4)
	if boxW < 56 {
		boxW = max(40, m.width-2)
	}
	contentW := max(30, boxW-6)

	const labelW = 14
	inputW := contentW - labelW - 5
	if inputW < 16 {
		inputW = 16
	}
	m.setInputWidths(inputW)

	blockW := 1 + 1 + labelW + 1 + inputW
	leftPad := max(0, (contentW-blockW)/2)

	title := m.t.T(i18n.KeyFormTitle)
	titlePad := max(0, (contentW-textutil.Width(title))/2)
	lines := []string{
		strings.Repeat(" ", titlePad) + theme.Title.Render(title),
		"",
	}

	visible := m.visibleFields()
	spacious := m.height >= 2*len(visible)+9
	for i, f := range visible {
		selected := i == m.focus
		marker := " "
		labelStyle := lipgloss.NewStyle()
		if selected {
			marker = "▸"
			labelStyle = theme.Header
		}

		var value string
		if f.kind == fieldType {
			value = "◀ " + m.connType + " ▶"
			if selected {
				value = theme.Header.Render(value)
			}
		} else {
			value = f.input.View()
		}

		line := strings.Repeat(" ", leftPad) + marker + " " +
			labelStyle.Render(pad(m.t.T(f.labelKey()), labelW)) + " " + value
		lines = append(lines, textutil.FitLine(line, contentW))
		if spacious && i < len(visible)-1 {
			lines = append(lines, "")
		}
	}

	if m.errText != "" {
		lines = append(lines, "", theme.Error.Render(textutil.FitLine(" "+m.errText, contentW)))
	}
	lines = append(lines, "",
		theme.Muted.Render(textutil.FitLine(centerText(m.t.T(i18n.KeyFormHint), contentW), contentW)),
	)

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.AccentColor())).
		Padding(1, 2).
		Width(contentW).
		Render(content)
}

func (m *Model) setInputWidths(width int) {
	for i := range m.fields {
		if m.fields[i].kind == fieldType {
			continue
		}
		m.fields[i].input.SetWidth(width)
	}
}

func centerText(s string, width int) string {
	if w := textutil.Width(s); w < width {
		return strings.Repeat(" ", (width-w)/2) + s
	}
	return s
}

func (m *Model) submit() (Model, tea.Cmd) {
	conn, err := m.build()
	if err != nil {
		m.errText = err.Error()
		return *m, nil
	}
	m.errText = ""
	m.open = false
	return *m, func() tea.Msg { return SubmittedMsg{Conn: conn} }
}

func (m *Model) build() (config.Connection, error) {
	name := strings.TrimSpace(m.input(fieldName).Value())
	if name == "" {
		return config.Connection{}, m.required(i18n.KeyFormName)
	}
	conn := config.Connection{Name: name, Type: m.connType}

	switch m.connType {
	case config.TypeSQLite:
		path := strings.TrimSpace(m.input(fieldPath).Value())
		if path == "" {
			return config.Connection{}, m.required(i18n.KeyFormPath)
		}
		conn.Path = path
	case config.TypePostgres, config.TypeMySQL:
		host := strings.TrimSpace(m.input(fieldHost).Value())
		user := strings.TrimSpace(m.input(fieldUser).Value())
		if host == "" {
			return config.Connection{}, m.required(i18n.KeyFormHost)
		}
		if user == "" {
			return config.Connection{}, m.required(i18n.KeyFormUser)
		}
		port := 0
		if portText := strings.TrimSpace(m.input(fieldPort).Value()); portText != "" {
			p, err := strconv.Atoi(portText)
			if err != nil || p < 1 || p > 65535 {
				return config.Connection{}, errors.New(m.t.T(i18n.KeyFormInvalidPort, portText))
			}
			port = p
		}
		conn.Host = host
		conn.User = user
		conn.Port = port
		conn.Password = m.input(fieldPassword).Value()
		if m.connType == config.TypePostgres {
			database := strings.TrimSpace(m.input(fieldDatabase).Value())
			if database == "" {
				return config.Connection{}, m.required(i18n.KeyFormDatabase)
			}
			conn.Database = database
			conn.SSLMode = strings.TrimSpace(m.input(fieldSSLMode).Value())
		}
	case config.TypeRedis:
		addr := strings.TrimSpace(m.input(fieldAddr).Value())
		if addr == "" {
			return config.Connection{}, m.required(i18n.KeyFormAddr)
		}
		database := 0
		if text := strings.TrimSpace(m.input(fieldDB).Value()); text != "" {
			value, err := strconv.Atoi(text)
			if err != nil || value < 0 {
				return config.Connection{}, errors.New(m.t.T(i18n.KeyFormInvalidDB, text))
			}
			database = value
		}
		conn.Addr = addr
		conn.Password = m.input(fieldPassword).Value()
		conn.DB = database
	case config.TypeMongo:
		uri := strings.TrimSpace(m.input(fieldURI).Value())
		if uri == "" {
			return config.Connection{}, m.required(i18n.KeyFormURI)
		}
		conn.URI = uri
	default:
		return config.Connection{}, errors.New(m.t.T(i18n.KeyErrorNotSupported, m.connType))
	}
	return conn, nil
}

func (m *Model) required(key string) error {
	return errors.New(m.t.T(i18n.KeyFormRequired, m.t.T(key)))
}

func (m *Model) input(kind fieldKind) *textinput.Model {
	for i := range m.fields {
		if m.fields[i].kind == kind {
			return &m.fields[i].input
		}
	}
	return nil
}

func (m *Model) setValue(kind fieldKind, value string) {
	if input := m.input(kind); input != nil {
		input.SetValue(value)
	}
}

func (m *Model) visibleFields() []*field {
	order := m.fieldOrder()
	out := make([]*field, 0, len(order))
	for _, kind := range order {
		for i := range m.fields {
			if m.fields[i].kind == kind {
				out = append(out, &m.fields[i])
				break
			}
		}
	}
	return out
}

func (m *Model) fieldOrder() []fieldKind {
	switch m.connType {
	case config.TypeSQLite:
		return []fieldKind{fieldType, fieldName, fieldPath}
	case config.TypeMySQL:
		return []fieldKind{fieldType, fieldName, fieldHost, fieldPort, fieldUser, fieldPassword}
	case config.TypeRedis:
		return []fieldKind{fieldType, fieldName, fieldAddr, fieldPassword, fieldDB}
	case config.TypeMongo:
		return []fieldKind{fieldType, fieldName, fieldURI}
	default:
		return []fieldKind{fieldType, fieldName, fieldHost, fieldPort, fieldUser, fieldPassword, fieldDatabase, fieldSSLMode}
	}
}

func (m *Model) moveFocus(delta int) {
	visible := m.visibleFields()
	if len(visible) == 0 {
		return
	}
	if m.focus >= 0 && m.focus < len(visible) {
		visible[m.focus].input.Blur()
	}
	m.focus = (m.focus + delta + len(visible)) % len(visible)
	if visible[m.focus].kind != fieldType {
		_ = visible[m.focus].input.Focus()
	}
}

func (m *Model) cycleType(delta int) {
	types := []string{config.TypePostgres, config.TypeMySQL, config.TypeSQLite, config.TypeRedis, config.TypeMongo}
	index := 0
	for i, typ := range types {
		if typ == m.connType {
			index = i
			break
		}
	}
	index = (index + delta + len(types)) % len(types)
	m.connType = types[index]
	m.focus = 0
	m.errText = ""
	for i := range m.fields {
		m.fields[i].input.Blur()
	}

	port := m.input(fieldPort)
	switch m.connType {
	case config.TypePostgres:
		if port.Value() == "" || port.Value() == "3306" {
			port.SetValue("5432")
		}
		if m.input(fieldSSLMode).Value() == "" {
			m.setValue(fieldSSLMode, "disable")
		}
	case config.TypeMySQL:
		if port.Value() == "" || port.Value() == "5432" {
			port.SetValue("3306")
		}
	case config.TypeRedis:
		if m.input(fieldAddr).Value() == "" {
			m.setValue(fieldAddr, "127.0.0.1:6379")
		}
		if m.input(fieldDB).Value() == "" {
			m.setValue(fieldDB, "0")
		}
	case config.TypeMongo:
		if m.input(fieldURI).Value() == "" {
			m.setValue(fieldURI, "mongodb://127.0.0.1:27017")
		}
	}
}

func (f *field) labelKey() string {
	switch f.kind {
	case fieldName:
		return i18n.KeyFormName
	case fieldHost:
		return i18n.KeyFormHost
	case fieldPort:
		return i18n.KeyFormPort
	case fieldUser:
		return i18n.KeyFormUser
	case fieldPassword:
		return i18n.KeyFormPassword
	case fieldDatabase:
		return i18n.KeyFormDatabase
	case fieldSSLMode:
		return i18n.KeyFormSSLMode
	case fieldPath:
		return i18n.KeyFormPath
	case fieldAddr:
		return i18n.KeyFormAddr
	case fieldDB:
		return i18n.KeyFormDB
	case fieldURI:
		return i18n.KeyFormURI
	default:
		return i18n.KeyFormType
	}
}

func pad(s string, width int) string {
	if d := width - textutil.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

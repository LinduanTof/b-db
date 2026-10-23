package detail

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/i18n"
)

func TestRenderAndScroll(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(80, 12)
	m.Open("public", "users")
	if view := m.View(); view == "" {
		t.Fatal("expected loading view")
	}
	m.SetData(
		[]db.Column{
			{Name: "id", DataType: "bigint", PrimaryKey: true},
			{Name: "name", DataType: "text", Nullable: true},
		},
		[]db.Index{{Name: "users_pkey", Columns: []string{"id"}, Unique: true}},
		[]db.ForeignKey{{Name: "fk_owner", Column: "owner_id", RefTable: "users", RefColumn: "id"}},
		nil,
	)
	view := m.View()
	for _, want := range []string{"Table structure: public.users", "id", "PK", "users_pkey", "fk_owner"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() {
		t.Fatal("esc should close detail")
	}
}

func TestError(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(60, 8)
	m.Open("public", "users")
	m.SetData(nil, nil, nil, errTest{})
	if view := m.View(); !strings.Contains(view, "boom") {
		t.Fatalf("error not rendered:\n%s", view)
	}
}

type errTest struct{}

func (errTest) Error() string { return "boom" }

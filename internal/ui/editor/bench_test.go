package editor

import (
	"strconv"
	"strings"
	"testing"

	"github.com/linduan-guan/b-db/internal/i18n"
)

func BenchmarkView100Lines(b *testing.B) {
	var sql strings.Builder
	for i := 0; i < 100; i++ {
		sql.WriteString("SELECT id, name FROM users WHERE id = ")
		sql.WriteString(strconv.Itoa(i))
		sql.WriteString(";\n")
	}
	m := New(i18n.New("en"))
	m.SetSize(100, 20)
	_ = m.Focus()
	m.SetValue(sql.String())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

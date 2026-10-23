package results

import (
	"testing"

	"github.com/LinduanTof/b-db/internal/i18n"
)

func BenchmarkView1000Rows(b *testing.B) {
	m := New(i18n.New("en"))
	m.SetSize(120, 30)
	m.SetResult(sample(1000))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkScroll1000Rows(b *testing.B) {
	m := New(i18n.New("en"))
	m.SetSize(120, 30)
	m.SetResult(sample(1000))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Scroll(1)
	}
}

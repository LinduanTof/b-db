package i18n

import "testing"

func TestDictionariesHaveSameKeys(t *testing.T) {
	for key := range en {
		if _, ok := zh[key]; !ok {
			t.Errorf("zh is missing key %q", key)
		}
	}
	for key := range zh {
		if _, ok := en[key]; !ok {
			t.Errorf("en is missing key %q", key)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "zh_CN.UTF-8")
	if got := New("auto").Lang(); got != Zh {
		t.Fatalf("auto with zh locale = %q, want zh", got)
	}
	t.Setenv("LANG", "en_US.UTF-8")
	if got := New("auto").Lang(); got != En {
		t.Fatalf("auto with en locale = %q, want en", got)
	}
	if got := New("zh").Lang(); got != Zh {
		t.Fatalf("explicit zh = %q", got)
	}
}

func TestFallbackToEnglish(t *testing.T) {
	tr := New("zh")
	if got := tr.T(KeyStatusReady); got != "就绪" {
		t.Fatalf("zh ready = %q", got)
	}
	if got := tr.T("unknown.key"); got != "unknown.key" {
		t.Fatalf("unknown key = %q", got)
	}
}

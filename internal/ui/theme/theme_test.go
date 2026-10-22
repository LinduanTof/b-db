package theme

import "testing"

func TestSetAndFallback(t *testing.T) {
	defer Set("default")
	if got := Set("dracula"); got != "dracula" {
		t.Fatalf("Set(dracula) = %q", got)
	}
	if Current() != "dracula" {
		t.Fatalf("current = %q", Current())
	}
	if got := Set("nope"); got != "default" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestNamesAndExists(t *testing.T) {
	names := Names()
	if len(names) < 5 || names[0] != "default" {
		t.Fatalf("names = %v", names)
	}
	for _, name := range names {
		if !Exists(name) {
			t.Fatalf("Exists(%q) = false", name)
		}
	}
	if Exists("nope") {
		t.Fatal("Exists(nope) should be false")
	}
}

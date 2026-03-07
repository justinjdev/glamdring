package tui

import "testing"

func TestThemeRegistry_AllBuiltins(t *testing.T) {
	expected := []string{"glamdring", "rivendell", "mithril", "lothlorien", "shire"}
	for _, name := range expected {
		p, ok := LookupTheme(name)
		if !ok {
			t.Errorf("theme %q not found in registry", name)
			continue
		}
		if p.Name != name {
			t.Errorf("theme %q has Name=%q", name, p.Name)
		}
		if p.Bg == "" || p.Fg == "" || p.Primary == "" {
			t.Errorf("theme %q has empty required fields", name)
		}
	}
}

func TestThemeRegistry_UnknownFallsBack(t *testing.T) {
	p, ok := LookupTheme("nonexistent")
	if ok {
		t.Error("expected ok=false for unknown theme")
	}
	if p.Name != "glamdring" {
		t.Errorf("fallback Name=%q, want glamdring", p.Name)
	}
}

func TestThemeNames(t *testing.T) {
	names := ThemeNames()
	if len(names) != 5 {
		t.Fatalf("ThemeNames() returned %d names, want 5", len(names))
	}
	// Should be sorted.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("ThemeNames not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

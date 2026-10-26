package testkit

import "testing"

func TestFormatNames(t *testing.T) {
	for _, c := range []struct {
		name   string
		errors int
	}{
		{"step", 0}, {"remaining_seconds", 0}, {"gpu.memory", 0}, {"state.json", 0}, {"step_", 0}, {"tr-profile", 0},
		{"adım", 1}, {"süre_sn", 1}, {"Step", 1}, {"", 1}, {"gpu.çöp", 1},
		// Sınır: Türkçe harfsiz bir Türkçe sözcük biçim olarak geçer; anahtarlar ayrıca her biçimin
		// testinde beklenen İngilizce listeyle karşılaştırılır.
		{"gpu.bellek", 0},
	} {
		r := &recorder{TB: t}
		FormatNames(r, c.name)
		if len(r.errors) != c.errors {
			t.Errorf("FormatNames(%q): %d errors, want %d", c.name, len(r.errors), c.errors)
		}
	}
}

func TestFormatJSONKeys(t *testing.T) {
	type inner struct {
		Ok  int `json:"ok_key"`
		Bad int `json:"tür"`
	}
	type outer struct {
		Items   []inner          `json:"items,omitempty"`
		ByName  map[string]inner `json:"by_name"`
		NoTag   int
		Skipped int `json:"-"`
		hidden  int
	}
	r := &recorder{TB: t}
	FormatJSONKeys(r, outer{})
	if len(r.errors) != 2 { // "tür" (bir kez: tip bir kez gezilir) ve NoTag
		t.Errorf("FormatJSONKeys: %q, want 2 errors", r.errors)
	}
}

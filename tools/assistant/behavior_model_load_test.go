package assistant_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Dost modelsiz açılır (tek tık B): asistan.tr'nin ekranlı kipi --ayar'ın klasöründe (Dost klasörü) kurulu takım
// yoksa `setup`'ta bekler; metin turu `not_ready`, katalogda olmayan girdi `unknown_entry`, dosyaları olmayan girdi
// `not_installed` alır. Önceki açılışın yarıda kalan yüklemesi (yukleme.json) görüntüde `model_load_failed` olur ve
// işaret silinir; bozuk işaret girdisizdir. Model yüklenmez: katalog depodakidir, Dost klasörü geçicidir.
const (
	dostCatalog = "tools/assistant/katalog.json"
	slotModule  = "tools/assistant/yuva.tr"
	slotTest    = "tools/assistant/yuva_test.tr"
)

// Yuvanın modülü ve testi README'de anılır; test modülü aktarır, program da.
func TestSlotFilesAreListed(t *testing.T) {
	readme := readFile(t, readmePath)
	for _, f := range []string{slotModule, slotTest} {
		if name := f[strings.LastIndex(f, "/")+1:]; !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("%s does not name %s", readmePath, name)
		}
	}
	for _, f := range []string{slotTest, chainProgram} {
		if !strings.Contains(readFile(t, f), `TR_import "yuva"`) {
			t.Errorf("%s does not import the slot module", f)
		}
	}
}

func startDost(t *testing.T, dost string) screenServer {
	t.Helper()
	return startProgram(t, chainProgram, "DİNLİYORUM", "Dost", "--kip", "canlı", "--ekran", "0",
		"--ayar", filepath.Join(dost, "ayar.json"), "--katalog", dostCatalog)
}

// snapshotOf ilk olayın (session) görüntüsü.
func snapshotOf(t *testing.T, s screenServer) []any {
	t.Helper()
	first := next(t, s.stream(t, ""))
	if first.Type != "session" {
		t.Fatalf("ilk olay %s", first.Type)
	}
	snapshot, _ := first.Data["snapshot"].([]any)
	return snapshot
}

func snapshotItems(snapshot []any, typ string) []map[string]any {
	var items []map[string]any
	for _, x := range snapshot {
		m, _ := x.(map[string]any)
		if m["type"] == typ {
			d, _ := m["data"].(map[string]any)
			items = append(items, d)
		}
	}
	return items
}

func TestDostStartsWithoutModels(t *testing.T) {
	s := startDost(t, filepath.Join(t.TempDir(), "Dost"))
	snapshot := snapshotOf(t, s)
	states := snapshotItems(snapshot, "state")
	if len(states) != 1 || states[0]["state"] != "setup" {
		t.Fatalf("modelsiz açılışta durum setup olmalı: %v", snapshot)
	}
	if n := len(snapshotItems(snapshot, "settings")); n != 0 {
		t.Errorf("ayar dosyası yokken settings gelmemeli, %d geldi", n)
	}
	for _, c := range []struct{ id, typ, data, want string }{
		{"c-1", "send_text", `{"text":"merhaba"}`, "not_ready"},
		{"c-2", "model_load", `{"entry_id":"yok"}`, "unknown_entry"},
		{"c-3", "model_load", `{"entry_id":"whisper-small"}`, "unknown_entry"},
		{"c-4", "model_load", `{"entry_id":"qwen3-4b-q4km"}`, "not_installed"},
	} {
		if got := s.command(t, c.id, c.typ, c.data); got != c.want {
			t.Errorf("%s %s: %s, beklenen %s", c.typ, c.data, got, c.want)
		}
	}
}

func TestDostReportsAnInterruptedLoad(t *testing.T) {
	for _, d := range []struct{ name, marker, model string }{
		{"geçerli", `{"entry": "qwen3-4b-q4km", "started_at": "2026-11-12T00:00:00Z"}`, "qwen3-4b-q4km"},
		{"bozuk", `{bozuk`, ""},
	} {
		t.Run(d.name, func(t *testing.T) {
			dost := filepath.Join(t.TempDir(), "Dost")
			if err := os.MkdirAll(dost, 0o700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(dost, "yukleme.json")
			if err := os.WriteFile(marker, []byte(d.marker), 0o600); err != nil {
				t.Fatal(err)
			}
			s := startDost(t, dost)
			errs := snapshotItems(snapshotOf(t, s), "error")
			if len(errs) != 1 || errs[0]["code"] != "model_load_failed" {
				t.Fatalf("görüntüde tek model_load_failed bekleniyordu: %v", errs)
			}
			values, _ := errs[0]["values"].(map[string]any)
			if got, _ := values["model"].(string); got != d.model {
				t.Errorf("model %q, beklenen %q", got, d.model)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Errorf("işaret silinmedi: %v", err)
			}
		})
	}
}

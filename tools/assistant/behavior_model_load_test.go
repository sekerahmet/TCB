package assistant_test

import (
	"net/http"
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
	// Ekranlı kipin oturumu (tek tık G, DENETİM 489 N4): ayrı TR testi yok, bu dosyanın testleri onu koşturur.
	sessionModule = "tools/assistant/oturum.tr"
)

// Oturumun modülü README'de anılır, program onu aktarır.
func TestSessionFileIsListed(t *testing.T) {
	if name := sessionModule[strings.LastIndex(sessionModule, "/")+1:]; !strings.Contains(readFile(t, readmePath), "`"+name+"`") {
		t.Errorf("%s does not name %s", readmePath, name)
	}
	if !strings.Contains(readFile(t, chainProgram), `TR_import "oturum"`) {
		t.Errorf("%s does not import the session module", chainProgram)
	}
	if !strings.Contains(readFile(t, sessionModule), "TR_struct Oturum {") {
		t.Errorf("%s does not define Oturum", sessionModule)
	}
}

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

// Seçeneksiz Dost'ta eşleme yolları yoktur (tek tık G; dil tanımı 28.2b): kod üretilmez, tarayıcı açılmaz;
// belirteçsiz istek 401 alır ve işleyici çağrılmaz (28.2), belirteçli istek ekranın bilinmeyen yolu gibi 404 alır.
// Sayfa kutuya düşer (index.html, `renew`'un 202 dışı cevabı).
func TestDostWithoutPairingHasNoPairingPaths(t *testing.T) {
	s := startDost(t, filepath.Join(t.TempDir(), "Dost"))
	pairing := map[string]string{"Dost-Pair-Code": strings.Repeat("a", 43), "Dost-Pair-Renew": "1"}
	for _, path := range []string{"/pair", "/pair/renew"} {
		res := s.request(t, "GET", path, pairing, "")
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s without a token: %d, want 401", path, res.StatusCode)
		}
		header := s.auth()
		for k, v := range pairing {
			header[k] = v
		}
		res = s.request(t, "GET", path, header, "")
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s with the token: %d, want 404", path, res.StatusCode)
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
		{"c-5", "settings_set", `{"key":"yok_böyle_bir_anahtar","value":4096}`, "setting_invalid"},
		{"c-6", "settings_set", `{"key":"speech_language","value":"en"}`, "not_installed"},
		{"c-7", "settings_set", `{"key":"speech_language","value":"xx"}`, "unknown_language"},
		// Bağlamın üst sınırı katalogun en büyük context_max'ıdır: modelsiz açılışta da tanımlı.
		{"c-8", "settings_set", `{"key":"context","value":4096}`, "accepted"},
		{"c-9", "settings_set", `{"key":"context","value":40961}`, "setting_invalid"},
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

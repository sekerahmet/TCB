package debugger

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// dapDisplayOut yardımcı testin çıktı dosyasını veren ortam değişkenidir (yalnız alt süreçte).
const dapDisplayOut = "TR_DAP_DISPLAY_OUT"

// TestDAPDisplayHelper bir DAP oturumunda değişkenlerin değerini ve tipini, İzleme Masası'nın tip
// sütununu ve değerlendirmenin tipini dosyaya yazar; yalnızca TestDAPDisplayFollowsPersonsLanguage
// tarafından TR_DIL=tr/en alt süreçlerinde çalışır.
func TestDAPDisplayHelper(t *testing.T) {
	out := os.Getenv(dapDisplayOut)
	if out == "" {
		t.Skip("yalnızca TestDAPDisplayFollowsPersonsLanguage'ın alt süreci")
	}
	c := startDAP(t)
	path := writeProgram(t, "değişken b = doğru\ndeğişken l = [doğru, yok]\ndeğişken t = tensör([1.0, 2.0])\nyazdır(b)\n")
	c.launch(path, false, map[string]interface{}{"line": 4})
	c.event("stopped")
	var lines []string
	for _, v := range c.request("variables", map[string]interface{}{"variablesReference": c.scopeRef("Genel")})["variables"].([]interface{}) {
		m := v.(map[string]interface{})
		if name := m["name"].(string); name == "b" || name == "l" || name == "t" {
			lines = append(lines, fmt.Sprintf("var %s | %s | %s", name, m["value"], m["type"]))
		}
	}
	for _, d := range c.request("trVariables", nil)["variables"].([]interface{}) {
		m := d.(map[string]interface{})
		if name := m["name"].(string); name == "b" || name == "l" || name == "t" {
			lines = append(lines, fmt.Sprintf("panel %s | %s | %s", name, m["value"], m["type"]))
		}
	}
	ev := c.request("evaluate", map[string]interface{}{"expression": "b", "context": "watch"})
	lines = append(lines, fmt.Sprintf("eval b | %s | %s", ev["result"], ev["type"]))
	sort.Strings(lines)
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
	if err := os.WriteFile(out, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Düzenleyicinin hata ayıklayıcısında değer ve tipi kişinin dilindedir (TR_DIL; DENETİM 3. adım,
// bulgu 2): TR_DIL=en ile Türkçe programın değişkeni "true | Bool", İzleme Masası'nda tensörün
// tipi "Tensor [2]". Önceden değer TR_DIL'de, tip kanonik Türkçeydi ("true | Mantık").
// TR_DIL süreç başında okunduğu için her dil ayrı bir alt süreçte koşar.
func TestDAPDisplayFollowsPersonsLanguage(t *testing.T) {
	en := string(testkit.RunWithLanguageFile(t, "TestDAPDisplayHelper", dapDisplayOut, "en"))
	tr := string(testkit.RunWithLanguageFile(t, "TestDAPDisplayHelper", dapDisplayOut, "tr"))
	for _, want := range []string{"var b | true | Bool", "var l | (2) [true, none] | List", "| Tensor", "panel t | ", "Tensor [2]", "eval b | true | Bool"} {
		if !strings.Contains(en, want) {
			t.Errorf("TR_DIL=en: %q yok:\n%s", want, en)
		}
	}
	for _, unwanted := range []string{"Mantık", "Liste", "Tensör", "doğru", "yok"} {
		if strings.Contains(en, unwanted) {
			t.Errorf("TR_DIL=en çıktısında Türkçe %q:\n%s", unwanted, en)
		}
	}
	for _, want := range []string{"var b | doğru | Mantık", "var l | (2) [doğru, yok] | Liste", "Tensör [2]", "eval b | doğru | Mantık"} {
		if !strings.Contains(tr, want) {
			t.Errorf("TR_DIL=tr: %q yok:\n%s", want, tr)
		}
	}
}

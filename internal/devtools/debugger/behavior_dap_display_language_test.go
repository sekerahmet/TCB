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
// tarafından TR_LANG=tr/en alt süreçlerinde çalışır.
func TestDAPDisplayHelper(t *testing.T) {
	out := os.Getenv(dapDisplayOut)
	if out == "" {
		t.Skip("yalnızca TestDAPDisplayFollowsPersonsLanguage'ın alt süreci")
	}
	c := startDAP(t)
	path := writeProgram(t, "TR_var b = TR_true\nTR_var l = [TR_true, TR_none]\nTR_var t = TR_Tensor([1.0, 2.0])\nTR_print(b)\n")
	c.launch(path, false, map[string]interface{}{"line": 5})
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

// dapExpressionOut ifade yardımcı testinin çıktı dosyasını veren ortam değişkenidir (yalnız alt süreçte).
const dapExpressionOut = "TR_DAP_EXPRESSION_OUT"

// TestDAPExpressionHelper koşullu kesme noktasının koşulunu ve izleme ifadesini kimlik yazımıyla
// yazar; durduğu yerdeki i'yi, ifadenin değerini ve Türkçe harfsiz bir komutun (adim) önerilip
// önerilmediğini dosyaya yazar. Yalnızca TestDAPExpressionsAreWrittenWithIdentities'in alt
// süreçlerinde çalışır.
func TestDAPExpressionHelper(t *testing.T) {
	out := os.Getenv(dapExpressionOut)
	if out == "" {
		t.Skip("yalnızca TestDAPExpressionsAreWrittenWithIdentities'in alt süreci")
	}
	c := startDAP(t)
	path := writeProgram(t, "TR_var l = [1, 2]\nTR_for i TR_in 0..3 {\n    TR_print(i)\n}\n")
	c.launch(path, false, map[string]interface{}{"line": 4, "condition": "TR_sum(l) == i + 2"})
	c.event("stopped")
	i := c.request("evaluate", map[string]interface{}{"expression": "i", "context": "watch"})["result"]
	w := c.request("evaluate", map[string]interface{}{"expression": "TR_sum(l) * 2", "context": "watch"})["result"]
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
	_, hinted := oldCommand("adim")
	if err := os.WriteFile(out, []byte(fmt.Sprintf("i=%v watch=%v adim=%v", i, w, hinted)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Kural (17.7): kişinin hata ayıklayıcıya yazdığı ifade (koşul, izleme) koddur, kimlik yazımıyla
// yazılır, TR_LANG ne olursa olsun: "TR_sum(l) == i + 2" koşulu ve "TR_sum(l) * 2" izlemesi her dilde
// doğru değeri verir. Türkçe harfsiz komut (adim) yalnızca Türkçe okuyana önerilir.
func TestDAPExpressionsAreWrittenWithIdentities(t *testing.T) {
	for lang, want := range map[string]string{"en": "i=1 watch=6 adim=false", "tr": "i=1 watch=6 adim=true"} {
		if got := string(testkit.RunWithLanguageFile(t, "TestDAPExpressionHelper", dapExpressionOut, lang)); got != want {
			t.Errorf("TR_LANG=%s: %q, beklenen %q", lang, got, want)
		}
	}
}

// Düzenleyicinin hata ayıklayıcısında değer ve tipi kişinin dilindedir (TR_LANG; DENETİM 3. adım,
// bulgu 2): TR_LANG=en ile Türkçe programın değişkeni "true | Bool", İzleme Masası'nda tensörün
// tipi "Tensor [2]". Önceden değer TR_LANG'de, tip kanonik Türkçeydi ("true | Mantık").
// TR_LANG süreç başında okunduğu için her dil ayrı bir alt süreçte koşar.
func TestDAPDisplayFollowsPersonsLanguage(t *testing.T) {
	en := string(testkit.RunWithLanguageFile(t, "TestDAPDisplayHelper", dapDisplayOut, "en"))
	tr := string(testkit.RunWithLanguageFile(t, "TestDAPDisplayHelper", dapDisplayOut, "tr"))
	for _, want := range []string{"var b | true | Bool", "var l | (2) [true, none] | List", "| Tensor", "panel t | ", "Tensor [2]", "eval b | true | Bool"} {
		if !strings.Contains(en, want) {
			t.Errorf("TR_LANG=en: %q yok:\n%s", want, en)
		}
	}
	for _, unwanted := range []string{"Mantık", "Liste", "Tensör", "doğru", "yok"} {
		if strings.Contains(en, unwanted) {
			t.Errorf("TR_LANG=en çıktısında Türkçe %q:\n%s", unwanted, en)
		}
	}
	for _, want := range []string{"var b | doğru | Mantık", "var l | (2) [doğru, yok] | Liste", "Tensör [2]", "eval b | doğru | Mantık"} {
		if !strings.Contains(tr, want) {
			t.Errorf("TR_LANG=tr: %q yok:\n%s", want, tr)
		}
	}
}

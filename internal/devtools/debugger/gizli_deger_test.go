package debugger

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/process"
)

// Süreç yerleşikleri (komut_satırı, ortam_değişkeni) bu paketin testlerinde de kayıtlı olsun.
func init() { interp.MotorEkle(func(v *interp.VM) { process.Kaydet(v) }) }

// Hata ayıklama oturumunda gizli değerler düzenleyiciye giden hiçbir iletide görünmez:
// değişkenler, izleme ifadesi, İzleme Masası tablosu ve konsol çıktısı.
func TestDAPGizliDegerMaskelenir(t *testing.T) {
	const gizli = "hf_dapgizli77"
	path := writeProgram(t, `değişken ayar = komut_satırı({hf_token: ""})
değişken kayıt = {belirteç: ayar.hf_token, adım: 3}
yazdır("belirteç", ayar.hf_token)
yazdır("bitti")
`)
	c := startDAP(t)
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	c.request("launch", map[string]interface{}{"program": path, "args": []string{"--hf_token", gizli}})
	c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": path},
		"breakpoints": []map[string]interface{}{{"line": 4}}})
	c.request("configurationDone", nil)
	c.event("stopped")

	genel := c.varsOf(c.scopeRef("Genel"))
	if v := genel["kayıt"]; !strings.Contains(v, interp.GizliMaske) || strings.Contains(v, gizli) {
		t.Errorf("değişkenler panelinde gizli değer maskelenmeli: %q", v)
	}
	if v := c.evaluate("ayar.hf_token"); strings.Contains(v, gizli) {
		t.Errorf("izleme ifadesinde gizli değer görünmemeli: %q", v)
	}
	panel, _ := json.Marshal(c.request("trDegiskenler", nil))
	tablo, _ := json.Marshal(c.request("trTablo", map[string]interface{}{"ifade": "kayıt"}))
	for _, ileti := range []string{string(panel), string(tablo)} {
		if strings.Contains(ileti, gizli) || !strings.Contains(ileti, interp.GizliMaske) {
			t.Errorf("İzleme Masası iletisinde gizli değer maskelenmeli: %s", ileti)
		}
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
	for _, m := range c.backlog {
		if b, _ := json.Marshal(m); strings.Contains(string(b), gizli) {
			t.Errorf("iletide gizli değer: %s", b)
		}
	}
}

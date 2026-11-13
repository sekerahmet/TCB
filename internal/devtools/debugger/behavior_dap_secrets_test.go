package debugger

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/process"
)

// Süreç yerleşikleri (komut_satırı, ortam_değişkeni) bu paketin testlerinde de kayıtlı olsun.
func init() { interp.AddEngine(func(v *interp.VM) { process.Register(v) }) }

// Hata ayıklama oturumunda gizli değerler düzenleyiciye giden hiçbir iletide görünmez:
// değişkenler, izleme ifadesi, İzleme Masası tablosu ve konsol çıktısı.
func TestDAPSecretMasked(t *testing.T) {
	const secret = "hf_dapgizli77"
	path := writeProgram(t, `TR_var ayar = TR_command_line({"hf_token": ""})
TR_var kayıt = {"belirteç": ayar["hf_token"], "step": 3}
TR_print("belirteç", ayar["hf_token"])
TR_print("bitti")
`)
	c := startDAP(t)
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	c.request("launch", map[string]interface{}{"program": path, "args": []string{"--hf_token", secret}})
	c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": path},
		"breakpoints": []map[string]interface{}{{"line": 4}}})
	c.request("configurationDone", nil)
	c.event("stopped")

	globalScope := c.varsOf(c.scopeRef("Genel"))
	if v := globalScope["kayıt"]; !strings.Contains(v, interp.SecretMask) || strings.Contains(v, secret) {
		t.Errorf("değişkenler panelinde gizli değer maskelenmeli: %q", v)
	}
	if v := c.evaluate(`ayar["hf_token"]`); strings.Contains(v, secret) {
		t.Errorf("izleme ifadesinde gizli değer görünmemeli: %q", v)
	}
	panel, _ := json.Marshal(c.request("trVariables", nil))
	table, _ := json.Marshal(c.request("trTable", map[string]interface{}{"expression": "kayıt"}))
	for _, message := range []string{string(panel), string(table)} {
		if strings.Contains(message, secret) || !strings.Contains(message, interp.SecretMask) {
			t.Errorf("İzleme Masası iletisinde gizli değer maskelenmeli: %s", message)
		}
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
	for _, m := range c.backlog {
		if b, _ := json.Marshal(m); strings.Contains(string(b), secret) {
			t.Errorf("iletide gizli değer: %s", b)
		}
	}
}

// TestDAPMasksAnAddressWithoutSecrets: gizli değer kayıtlı olmasa da değişken görünümündeki adresin
// parolası örtülür (19.4'ün bağlamlı maskesi): iletinin koruması adresli iletiyi atlamaz.
func TestDAPMasksAnAddressWithoutSecrets(t *testing.T) {
	path := writeProgram(t, `TR_var adres = "postgres://u:ab@db/ana"
TR_print("bekle")
TR_print("bitti")
`)
	c := startDAP(t)
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	c.request("launch", map[string]interface{}{"program": path})
	c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": path},
		"breakpoints": []map[string]interface{}{{"line": 3}}})
	c.request("configurationDone", nil)
	c.event("stopped")
	if v := c.varsOf(c.scopeRef("Genel"))["adres"]; !strings.Contains(v, "u:***@db") {
		t.Errorf("adresin parolası örtülmeli: %q", v)
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
}

// TestDAPDisplayIsCutAfterMask: değişkenin değer metni 200 karakterde kesilmeden önce maskelenir
// (19.4): 200. karakter kayıtsız bir adresin parolasının ortasına düşer; görünümde başı kalmaz.
func TestDAPDisplayIsCutAfterMask(t *testing.T) {
	text := strings.Repeat("a", 186) + " http://u:Vekil2026@h"
	if got := display(&interp.StringVal{Val: text}); strings.Contains(got, "Vek") {
		t.Errorf("parolanın başı kalmamalı: %q", got[180:])
	}
}

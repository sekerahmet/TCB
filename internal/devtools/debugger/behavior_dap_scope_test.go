package debugger

import (
	"reflect"
	"testing"
)

const dapScopeProgram = `değişken sayi = 1
değişken liste = [{"a": 1}]
fonksiyon f() {
    değişken sayi = 2
    değişken liste = [{"b": 2}]
    yazdır(sayi)
}
f()
`

// Aynı adlı genel ve yerel değişken: değer değiştirme isteği hangi kapsamın başvurusunu
// (variablesReference) verdiyse yalnızca onu değiştirir; İzleme Masası ✏️ bu yolu kullanır.
func TestDAPSetVariableTargetsScope(t *testing.T) {
	c := startDAP(t)
	c.launch(writeProgram(t, dapScopeProgram), false, map[string]interface{}{"line": 6})
	c.event("stopped")

	set := func(scope, value string) {
		t.Helper()
		c.request("setVariable", map[string]interface{}{"variablesReference": c.scopeRef(scope), "name": "sayi", "value": value})
	}
	set("Genel", "10")
	if got := c.varsOf(c.scopeRef("Genel"))["sayi"]; got != "10" {
		t.Fatalf("genel sayi = %q, beklenen 10", got)
	}
	if got := c.varsOf(c.scopeRef("Yerel"))["sayi"]; got != "2" {
		t.Fatalf("yerel sayi genel değişince değişti: %q", got)
	}
	set("Yerel", "5")
	if got := c.varsOf(c.scopeRef("Yerel"))["sayi"]; got != "5" {
		t.Fatalf("yerel sayi = %q, beklenen 5", got)
	}
	if got := c.varsOf(c.scopeRef("Genel"))["sayi"]; got != "10" {
		t.Fatalf("genel sayi yerel değişince değişti: %q", got)
	}
}

// Tablo isteği kapsam alır: aynı adlı genel ve yerel tablo ayrı ayrı açılabilir. Kapsamsız
// istek (Variables panelinden "Tablo Görünümü") yerel kapsamdan başlar.
func TestDAPTableTargetsScope(t *testing.T) {
	c := startDAP(t)
	c.launch(writeProgram(t, dapScopeProgram), false, map[string]interface{}{"line": 6})
	c.event("stopped")

	columns := func(args map[string]interface{}) []string {
		t.Helper()
		var out []string
		for _, s := range c.request("trTable", args)["columns"].([]interface{}) {
			out = append(out, s.(string))
		}
		return out
	}
	if got := columns(map[string]interface{}{"expression": "liste", "scope": "Genel"}); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("genel tablo sütunları: %v", got)
	}
	if got := columns(map[string]interface{}{"expression": "liste", "scope": "Yerel"}); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("yerel tablo sütunları: %v", got)
	}
	if got := columns(map[string]interface{}{"expression": "liste"}); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("kapsamsız tablo yerelden başlamalı: %v", got)
	}
}

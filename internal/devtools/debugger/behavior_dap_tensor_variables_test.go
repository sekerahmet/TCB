package debugger

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Bu paketin testlerinde gerçek tensör değerleri de kullanılabilsin diye tensör motoru
// kaydedilir (motor paketi bu paketi içe aktardığından tamamı içe aktarılamaz).
func init() { interp.AddEngine(ml.Register) }

// Değişkenler panelinde tensör ilk eksen boyunca açılır: 2 boyutluda satırlar (yine
// açılabilir 1 boyutlu tensörler), 1 boyutluda sayılar; çok büyük tensörde ilk öğeler ve
// "…" gösterilir. Satır elemanı panelden değiştirilebilir; İzleme Masası tablosu bozulmaz.
func TestDAPTensorVariables(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "TR_var t = TR_Tensor([[1, 2, 3], [4, 5, 6]])\nTR_var büyük = TR_zeros([2500])\nTR_var s = TR_Tensor(7.0)\nTR_print(t)\n")
	c.launch(path, false, map[string]interface{}{"line": 4})
	c.event("stopped")

	globalScope := c.request("variables", map[string]interface{}{"variablesReference": c.scopeRef("Genel")})["variables"].([]interface{})
	refs := map[string]int{}
	for _, v := range globalScope {
		m := v.(map[string]interface{})
		refs[m["name"].(string)] = int(m["variablesReference"].(float64))
		// 1-8 elemanlı tensörün değerleri satır içinde görünür (bkz. izleme_ozeti_test.go)
		if m["name"] == "t" && m["value"] != "[[1.0000, 2.0000, 3.0000], [4.0000, 5.0000, 6.0000]] (Tensör, 2×3)" {
			t.Fatalf("t değeri satır içinde görünmeli: %v", m)
		}
	}
	if refs["t"] == 0 || refs["büyük"] == 0 {
		t.Fatalf("tensörler açılabilir olmalı: %v", refs)
	}
	if refs["s"] != 0 {
		t.Fatal("0 boyutlu tensör açılabilir olmamalı")
	}

	lines := c.request("variables", map[string]interface{}{"variablesReference": refs["t"]})["variables"].([]interface{})
	if len(lines) != 2 {
		t.Fatalf("2 satır bekleniyordu: %v", lines)
	}
	first := lines[1].(map[string]interface{})
	if first["name"] != "[1]" || first["value"] != "[4.0000, 5.0000, 6.0000] (Tensör, 3)" || first["evaluateName"] != "t[1]" {
		t.Fatalf("t[1] satırı değerleriyle görünmeli: %v", first)
	}
	elems := c.varsOf(int(first["variablesReference"].(float64)))
	if elems["[0]"] != "4.0" || elems["[2]"] != "6.0" {
		t.Fatalf("t[1] elemanları 4.0 ... 6.0 olmalı: %v", elems)
	}

	// ✏️ satır elemanını değiştir: t[1][2] = 60
	set := c.request("setVariable", map[string]interface{}{"variablesReference": int(first["variablesReference"].(float64)), "name": "[2]", "value": "60"})
	if set["value"] != "60" {
		t.Fatalf("setVariable 60 döndürmeliydi: %v", set)
	}
	if got := c.evaluate("t.değer"); got != "(2) [[1.0, 2.0, 3.0], [4.0, 5.0, 60.0]]" {
		t.Fatalf("atama tensörü değiştirmeli: %s", got)
	}

	large := c.varsOf(refs["büyük"])
	if len(large) != repeatLimit+1 || large["[999]"] != "0.0" || !strings.Contains(large["…"], "toplam 2500") {
		t.Fatalf("büyük tensörde ilk %d öğe ve '…' bekleniyordu (%d öğe): %q", repeatLimit, len(large), large["…"])
	}

	// İzleme Masası ve tablo görünümü
	panel := c.request("trVariables", nil)
	for _, d := range panel["variables"].([]interface{}) {
		dm := d.(map[string]interface{})
		if dm["name"] == "t" && (dm["kind"] != "tensor" || dm["type"] != "Tensör [2×3]" || dm["table"] != true) {
			t.Fatalf("t İzleme Masası'nda tablo olarak açılabilir tensör olmalı: %v", dm)
		}
	}
	table := c.request("trTable", map[string]interface{}{"expression": "t"})
	if len(table["columns"].([]interface{})) != 3 || len(table["rows"].([]interface{})) != 2 {
		t.Fatalf("2×3 tablo bekleniyordu: %v", table)
	}

	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
}

// Okunamayan program dosyası Türkçe bir hatayla reddedilir (işletim sisteminin metni sızmaz).
func TestDAPProgramUnreadable(t *testing.T) {
	c := startDAP(t)
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	seq := c.send("launch", map[string]interface{}{"program": writeProgram(t, "") + ".yok"})
	resp := c.next(func(m map[string]interface{}) bool {
		return m["type"] == "response" && int(m["request_seq"].(float64)) == seq
	}, "launch yanıtı")
	msgText, _ := resp["message"].(string)
	if resp["success"] != false || !strings.Contains(msgText, "Hata ayıklanacak program okunamadı") || !strings.Contains(msgText, "böyle bir dosya ya da klasör yok") {
		t.Fatalf("Türkçe okuma hatası bekleniyordu: %v", resp)
	}
}

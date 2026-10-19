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
func TestDAPTensorDegiskenleri(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "değişken t = tensör([[1, 2, 3], [4, 5, 6]])\ndeğişken büyük = sıfırlar([2500])\ndeğişken s = tensör(7.0)\nyazdır(t)\n")
	c.launch(path, false, map[string]interface{}{"line": 4})
	c.event("stopped")

	genel := c.request("variables", map[string]interface{}{"variablesReference": c.scopeRef("Genel")})["variables"].([]interface{})
	refler := map[string]int{}
	for _, v := range genel {
		m := v.(map[string]interface{})
		refler[m["name"].(string)] = int(m["variablesReference"].(float64))
		// 1-8 elemanlı tensörün değerleri satır içinde görünür (bkz. izleme_ozeti_test.go)
		if m["name"] == "t" && m["value"] != "[[1.0000, 2.0000, 3.0000], [4.0000, 5.0000, 6.0000]] (Tensör, 2×3)" {
			t.Fatalf("t değeri satır içinde görünmeli: %v", m)
		}
	}
	if refler["t"] == 0 || refler["büyük"] == 0 {
		t.Fatalf("tensörler açılabilir olmalı: %v", refler)
	}
	if refler["s"] != 0 {
		t.Fatal("0 boyutlu tensör açılabilir olmamalı")
	}

	satirlar := c.request("variables", map[string]interface{}{"variablesReference": refler["t"]})["variables"].([]interface{})
	if len(satirlar) != 2 {
		t.Fatalf("2 satır bekleniyordu: %v", satirlar)
	}
	ilk := satirlar[1].(map[string]interface{})
	if ilk["name"] != "[1]" || ilk["value"] != "[4.0000, 5.0000, 6.0000] (Tensör, 3)" || ilk["evaluateName"] != "t[1]" {
		t.Fatalf("t[1] satırı değerleriyle görünmeli: %v", ilk)
	}
	elemanlar := c.varsOf(int(ilk["variablesReference"].(float64)))
	if elemanlar["[0]"] != "4.0" || elemanlar["[2]"] != "6.0" {
		t.Fatalf("t[1] elemanları 4.0 ... 6.0 olmalı: %v", elemanlar)
	}

	// ✏️ satır elemanını değiştir: t[1][2] = 60
	set := c.request("setVariable", map[string]interface{}{"variablesReference": int(ilk["variablesReference"].(float64)), "name": "[2]", "value": "60"})
	if set["value"] != "60" {
		t.Fatalf("setVariable 60 döndürmeliydi: %v", set)
	}
	if got := c.evaluate("t.değer"); got != "(2) [[1.0, 2.0, 3.0], [4.0, 5.0, 60.0]]" {
		t.Fatalf("atama tensörü değiştirmeli: %s", got)
	}

	buyuk := c.varsOf(refler["büyük"])
	if len(buyuk) != yinelemeSiniri+1 || buyuk["[999]"] != "0.0" || !strings.Contains(buyuk["…"], "toplam 2500") {
		t.Fatalf("büyük tensörde ilk %d öğe ve '…' bekleniyordu (%d öğe): %q", yinelemeSiniri, len(buyuk), buyuk["…"])
	}

	// İzleme Masası ve tablo görünümü
	panel := c.request("trDegiskenler", nil)
	for _, d := range panel["degiskenler"].([]interface{}) {
		dm := d.(map[string]interface{})
		if dm["ad"] == "t" && (dm["tur"] != "tensor" || dm["tip"] != "Tensör [2×3]" || dm["tablo"] != true) {
			t.Fatalf("t İzleme Masası'nda tablo olarak açılabilir tensör olmalı: %v", dm)
		}
	}
	tablo := c.request("trTablo", map[string]interface{}{"ifade": "t"})
	if len(tablo["sutunlar"].([]interface{})) != 3 || len(tablo["satirlar"].([]interface{})) != 2 {
		t.Fatalf("2×3 tablo bekleniyordu: %v", tablo)
	}

	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
}

// Okunamayan program dosyası Türkçe bir hatayla reddedilir (işletim sisteminin metni sızmaz).
func TestDAPProgramOkunamadi(t *testing.T) {
	c := startDAP(t)
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	seq := c.send("launch", map[string]interface{}{"program": writeProgram(t, "") + ".yok"})
	resp := c.next(func(m map[string]interface{}) bool {
		return m["type"] == "response" && int(m["request_seq"].(float64)) == seq
	}, "launch yanıtı")
	mesaj, _ := resp["message"].(string)
	if resp["success"] != false || !strings.Contains(mesaj, "Hata ayıklanacak program okunamadı") || !strings.Contains(mesaj, "böyle bir dosya ya da klasör yok") {
		t.Fatalf("Türkçe okuma hatası bekleniyordu: %v", resp)
	}
}

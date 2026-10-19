package debugger

import "testing"

// Skaler tensörün (ör. kayıp) değeri Değişkenler panelinde ve İzleme Masası'nda görünmeli;
// yalnızca "Tensör (skaler)" yazıyordu. 1-8 elemanlı tensörler satır içinde, büyükler etiket
// ve "Tabloyu aç" ile görünür; bf16/fp16 tensörde tür de yazılır.
func TestDAPScalarAndSmallTensorValue(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "değişken kayıp = tensör(0.60738)\n"+
		"değişken w = tensör([[1.42981]])\n"+
		"değişken yarım = tensör(2.5).türe(\"bf16\")\n"+
		"değişken büyük = sıfırlar([3, 4])\n"+
		"yazdır(kayıp)\n")
	c.launch(path, false, map[string]interface{}{"line": 5})
	c.event("stopped")

	values := c.varsOf(c.scopeRef("Genel"))
	await := map[string]string{
		"kayıp": "0.6074 (Tensör, skaler)",
		"w":     "[[1.4298]] (Tensör, 1×1)",
		"yarım": "2.5000 (Tensör, skaler, bf16)",
		"büyük": "Tensör [3×4]",
	}
	for ad, b := range await {
		if values[ad] != b {
			t.Errorf("Değişkenler paneli %s = %q, beklenen %q", ad, values[ad], b)
		}
	}
	if got := c.evaluate("kayıp"); got != await["kayıp"] {
		t.Errorf("hata ayıklama konsolu / üzerine gelme kayıp = %q", got)
	}

	panel := c.request("trDegiskenler", nil)
	summaries := map[string]string{}
	tables := map[string]bool{}
	for _, d := range panel["degiskenler"].([]interface{}) {
		dm := d.(map[string]interface{})
		ad := dm["ad"].(string)
		summaries[ad], _ = dm["ozet"].(string)
		tables[ad], _ = dm["tablo"].(bool)
		if dm["deger"] != await[ad] {
			t.Errorf("İzleme Masası %s değeri %q, beklenen %q", ad, dm["deger"], await[ad])
		}
	}
	// Webview satır içi değeri "ozet" alanından alır; büyük tensörde boştur (yalnızca tablo).
	if summaries["w"] != await["w"] || summaries["kayıp"] != await["kayıp"] || summaries["büyük"] != "" {
		t.Errorf("ozet alanları: %v", summaries)
	}
	if !tables["w"] || !tables["büyük"] || tables["kayıp"] {
		t.Errorf("1-2 boyutlu tensörler tablo olarak açılabilmeli, skaler açılamamalı: %v", tables)
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
}

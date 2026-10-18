package ayikla

import (
	"os"
	"regexp"
	"testing"

	"tr_dili/internal/depo"
)

// İzleme Masası düğmelerinin (media/izleme_masasi.html, data-komut) gönderdiği DAP istekleri:
// her biri program duraklatılmışken sunucuda işlenmeli. Devam (continue) döngüdeki kesme
// noktasında yeniden durmalı ve değerler ilerlemeli; son turdan sonra program bitmeli.
// Durdur, düzenleyicide terminate isteğine dönüşür.
func TestDAPIzlemeMasasiDugmeleri(t *testing.T) {
	html, err := os.ReadFile(depo.Yol("duzenleyici/vscode/media/izleme_masasi.html"))
	if err != nil {
		t.Fatal(err)
	}
	dugmeler := map[string]bool{}
	for _, m := range regexp.MustCompile(`data-komut="([^"]+)"`).FindAllStringSubmatch(string(html), -1) {
		dugmeler[m[1]] = true
	}
	for _, k := range []string{"next", "stepIn", "stepOut", "continue", "stop"} {
		if !dugmeler[k] {
			t.Fatalf("İzleme Masası'nda %q düğmesi yok: %v", k, dugmeler)
		}
	}

	c := startDAP(t)
	path := writeProgram(t, `değişken toplam = 0
fonksiyon kare(x) {
    sonuç x * x
}
döngü s içinde [1, 2, 3] {
    değişken k = kare(s)
    toplam += k
}
yazdır("toplam: {toplam}")
`)
	c.launch(path, false, map[string]interface{}{"line": 6})
	durdu := func(beklenenSatir int, neden string) map[string]interface{} {
		t.Helper()
		ev := c.event("stopped")
		if satir := c.topFrameLine(); satir != beklenenSatir {
			t.Fatalf("%s sonrası satır %d, beklenen %d", neden, satir, beklenenSatir)
		}
		return ev
	}
	panelDegeri := func(ad string) string {
		t.Helper()
		for _, d := range c.request("trDegiskenler", nil)["degiskenler"].([]interface{}) {
			dm := d.(map[string]interface{})
			if dm["ad"] == ad {
				return dm["deger"].(string)
			}
		}
		return ""
	}

	durdu(6, "başlangıç")
	if v := panelDegeri("s"); v != "1" {
		t.Fatalf("ilk turda s = %q", v)
	}
	// Devam: aynı kesme noktasında, sonraki turda yeniden durur.
	c.request("continue", map[string]interface{}{"threadId": 1})
	if ev := durdu(6, "devam"); ev["reason"] != "breakpoint" {
		t.Fatalf("devam kesme noktasında durmalı: %v", ev)
	}
	if v := panelDegeri("s"); v != "2" {
		t.Fatalf("devam programı ilerletmeli: ikinci turda s = %q", v)
	}
	if v := panelDegeri("toplam"); v != "1" {
		t.Fatalf("devam programı ilerletmeli: toplam = %q", v)
	}
	// İçine gir → kare() içinde; Dışarı çık → çağrının döndüğü satır; Sonraki satır.
	c.request("stepIn", map[string]interface{}{"threadId": 1})
	durdu(3, "içine gir")
	c.request("stepOut", map[string]interface{}{"threadId": 1})
	durdu(7, "dışarı çık")
	c.request("next", map[string]interface{}{"threadId": 1})
	durdu(6, "sonraki satır")
	if v := panelDegeri("s"); v != "3" {
		t.Fatalf("üçüncü turda s = %q", v)
	}
	// Son turdan sonra Devam programı bitirir.
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("exited")
	c.event("terminated")
	if out := c.output.String(); !regexp.MustCompile(`toplam: 14`).MatchString(out) {
		t.Fatalf("program sonuna kadar çalışmalı; çıktı: %q", out)
	}
}

// Durdur düğmesi (düzenleyicide terminate) duraklatılmış programı sonlandırır.
func TestDAPIzlemeMasasiDurdur(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "değişken a = 1\nyazdır(a)\n")
	c.launch(path, true)
	c.event("stopped")
	c.request("terminate", map[string]interface{}{"restart": false})
	c.event("terminated")
}

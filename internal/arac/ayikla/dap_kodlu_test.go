package ayikla

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tr_dili/internal/dil/kaynak"
)

// gorunumAdresiYap bir dosya yolunun düzenleyicideki Türkçe görünümünün adresidir; VS Code'un
// yazdığı biçimle aynı: "tr-kodlu:/c%3A/klasor/program.tr" (sürücü harfi küçük, ':' kaçışlı).
func gorunumAdresiYap(yol string) string {
	p := filepath.ToSlash(yol)
	if runtime.GOOS == "windows" && len(p) >= 2 && p[1] == ':' {
		p = "/" + strings.ToLower(p[:1]) + "%3A" + p[2:]
	}
	parcalar := strings.Split(p, "/")
	for i, s := range parcalar {
		if !strings.Contains(s, "%3A") {
			parcalar[i] = url.PathEscape(s)
		}
	}
	return "tr-kodlu:" + strings.Join(parcalar, "/")
}

// Kodlu dosya düzenleyicide Türkçe görünümüyle (başlık gizli) açılır: kesme noktaları
// görünümün adresiyle ve görünümün satırlarıyla gelir (launch'tan önce de), çağrı yığını
// kaynağı görünümün adresiyle ve aynı satırla bildirir (17.5, 17.7).
func TestDAPKodluDosyaGorunumu(t *testing.T) {
	turkce := "değişken a = 1\na = 2\neğer a > 1 {\n    yazdır(\"büyük\")\n}\n"
	kodlu, err := kaynak.Kodla(turkce)
	if err != nil || !strings.HasPrefix(kodlu, kaynak.Baslik+"\n") {
		t.Fatalf("kodlanamadı: %q %v", kodlu, err)
	}
	path := writeProgram(t, kodlu)
	adres := gorunumAdresiYap(path)
	if got := kaynakYolu(adres); !samePath(got, path) {
		t.Fatalf("adres dosya yoluna çevrilmedi: %q → %q (beklenen %q)", adres, got, path)
	}

	c := startDAP(t)
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	// Görünümün 4. satırı: yazdır("büyük") (dosyada başlıkla 5. satır).
	yanit := c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": adres}, "breakpoints": []interface{}{map[string]interface{}{"line": 4}}})
	if bp := yanit["breakpoints"].([]interface{})[0].(map[string]interface{}); bp["verified"] != true {
		t.Fatalf("görünümdeki kesme noktası doğrulanmalı: %v", bp)
	}
	c.request("launch", map[string]interface{}{"program": path, "gorunumAdresi": adres})
	c.request("configurationDone", nil)
	c.event("stopped")
	st := c.request("stackTrace", map[string]interface{}{"threadId": 1})
	cerceve := st["stackFrames"].([]interface{})[0].(map[string]interface{})
	if satir := int(cerceve["line"].(float64)); satir != 4 {
		t.Fatalf("görünümün 4. satırında durmalıydı, %d", satir)
	}
	if kaynakBilgisi := cerceve["source"].(map[string]interface{}); kaynakBilgisi["path"] != adres || kaynakBilgisi["name"] != "program.tr" {
		t.Fatalf("çağrı yığınının kaynağı görünümün adresi olmalı: %v", kaynakBilgisi)
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
	if !strings.Contains(c.output.String(), "büyük") {
		t.Fatalf("program sonuna kadar çalışmalıydı: %q", c.output.String())
	}
}

// kaynakYolu yalnızca adresleri çevirir: dosya yolları (sürücü harfli olanlar da) olduğu gibi
// kalır; ağ paylaşımı adresi paylaşım yoluna çevrilir.
func TestDAPKaynakYolu(t *testing.T) {
	for _, yol := range []string{`C:\proje\a.tr`, "c:/proje/a.tr", "/ev/kullanici/a.tr", "a.tr", ""} {
		if got := kaynakYolu(yol); got != yol {
			t.Errorf("kaynakYolu(%q) = %q; değişmemeli", yol, got)
		}
	}
	if runtime.GOOS == "windows" {
		if got := kaynakYolu("tr-kodlu:/c%3A/proje%20klas%C3%B6r/a.tr"); got != `c:\proje klasör\a.tr` {
			t.Errorf("Windows adresi: %q", got)
		}
		if got := kaynakYolu("tr-kodlu://sunucu/pay/a.tr"); got != `\\sunucu\pay\a.tr` {
			t.Errorf("paylaşım adresi: %q", got)
		}
	} else if got := kaynakYolu("tr-kodlu:/ev/k%C3%BC/a.tr"); got != "/ev/kü/a.tr" {
		t.Errorf("adres: %q", got)
	}
	if got := kaynakYolu("tr-kodlu:"); got != "tr-kodlu:" {
		t.Errorf("yolsuz adres olduğu gibi kalmalı: %q", got)
	}
}

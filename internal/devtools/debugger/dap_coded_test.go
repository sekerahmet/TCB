package debugger

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
)

// makeViewURI bir dosya yolunun düzenleyicideki Türkçe görünümünün adresidir; VS Code'un
// yazdığı biçimle aynı: "tr-kodlu:/c%3A/klasor/program.tr" (sürücü harfi küçük, ':' kaçışlı).
func makeViewURI(fpath string) string {
	p := filepath.ToSlash(fpath)
	if runtime.GOOS == "windows" && len(p) >= 2 && p[1] == ':' {
		p = "/" + strings.ToLower(p[:1]) + "%3A" + p[2:]
	}
	parts := strings.Split(p, "/")
	for i, s := range parts {
		if !strings.Contains(s, "%3A") {
			parts[i] = url.PathEscape(s)
		}
	}
	return "tr-kodlu:" + strings.Join(parts, "/")
}

// Kodlu dosya düzenleyicide Türkçe görünümüyle (başlık gizli) açılır: kesme noktaları
// görünümün adresiyle ve görünümün satırlarıyla gelir (launch'tan önce de), çağrı yığını
// kaynağı görünümün adresiyle ve aynı satırla bildirir (17.5; 17_kodlar.md, "Düzenleyicide").
func TestDAPCodedFileView(t *testing.T) {
	turkishName := "değişken a = 1\na = 2\neğer a > 1 {\n    yazdır(\"büyük\")\n}\n"
	coded, err := source.Encode(turkishName)
	if err != nil || !strings.HasPrefix(coded, source.Header+"\n") {
		t.Fatalf("kodlanamadı: %q %v", coded, err)
	}
	path := writeProgram(t, coded)
	addr := makeViewURI(path)
	if got := sourcePath(addr); !samePath(got, path) {
		t.Fatalf("adres dosya yoluna çevrilmedi: %q → %q (beklenen %q)", addr, got, path)
	}

	c := startDAP(t)
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	// Görünümün 4. satırı: yazdır("büyük") (dosyada başlıkla 5. satır).
	reply := c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": addr}, "breakpoints": []interface{}{map[string]interface{}{"line": 4}}})
	if bp := reply["breakpoints"].([]interface{})[0].(map[string]interface{}); bp["verified"] != true {
		t.Fatalf("görünümdeki kesme noktası doğrulanmalı: %v", bp)
	}
	c.request("launch", map[string]interface{}{"program": path, "viewURI": addr})
	c.request("configurationDone", nil)
	c.event("stopped")
	st := c.request("stackTrace", map[string]interface{}{"threadId": 1})
	frame := st["stackFrames"].([]interface{})[0].(map[string]interface{})
	if line := int(frame["line"].(float64)); line != 4 {
		t.Fatalf("görünümün 4. satırında durmalıydı, %d", line)
	}
	if sourceInfo := frame["source"].(map[string]interface{}); sourceInfo["path"] != addr || sourceInfo["name"] != "program.tr" {
		t.Fatalf("çağrı yığınının kaynağı görünümün adresi olmalı: %v", sourceInfo)
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
	if !strings.Contains(c.output.String(), "büyük") {
		t.Fatalf("program sonuna kadar çalışmalıydı: %q", c.output.String())
	}
}

// sourcePath yalnızca adresleri çevirir: dosya yolları (sürücü harfli olanlar da) olduğu gibi
// kalır; ağ paylaşımı adresi paylaşım yoluna çevrilir.
func TestDAPSourcePath(t *testing.T) {
	for _, fpath := range []string{`C:\proje\a.tr`, "c:/proje/a.tr", "/ev/kullanici/a.tr", "a.tr", ""} {
		if got := sourcePath(fpath); got != fpath {
			t.Errorf("sourcePath(%q) = %q; değişmemeli", fpath, got)
		}
	}
	if runtime.GOOS == "windows" {
		if got := sourcePath("tr-kodlu:/c%3A/proje%20klas%C3%B6r/a.tr"); got != `c:\proje klasör\a.tr` {
			t.Errorf("Windows adresi: %q", got)
		}
		if got := sourcePath("tr-kodlu://sunucu/pay/a.tr"); got != `\\sunucu\pay\a.tr` {
			t.Errorf("paylaşım adresi: %q", got)
		}
	} else if got := sourcePath("tr-kodlu:/ev/k%C3%BC/a.tr"); got != "/ev/kü/a.tr" {
		t.Errorf("adres: %q", got)
	}
	if got := sourcePath("tr-kodlu:"); got != "tr-kodlu:" {
		t.Errorf("yolsuz adres olduğu gibi kalmalı: %q", got)
	}
}

package vm

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// NEREDE / WHERE koşulları artık TR kodu olarak çalıştırılmamalı
// ---------------------------------------------------------------------------

func testRows() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": int64(1), "ad": "Ayşe", "departman": "Yapay Zeka", "maas": int64(92000)},
		{"id": int64(2), "ad": "Ali", "departman": "Satış", "maas": int64(60000)},
		{"id": int64(3), "ad": "İrem", "departman": "Yapay Zeka", "maas": 75000.5},
	}
}

func knownCols() map[string]bool {
	return map[string]bool{"id": true, "ad": true, "departman": true, "maas": true}
}

func TestRowConditionBasics(t *testing.T) {
	cases := []struct {
		cond string
		want int
	}{
		{"maas >= 80000", 1},
		{"departman = 'Yapay Zeka' AND maas > 70000", 2},
		{"departman == \"Satış\" veya id == 1", 2},
		{"değil (id <> 2)", 1},
		{"NOT id = 2 && maas < 100000", 2},
		{"maas * 2 > 150000", 2},
		{"ad == 'O''Brien'", 0},
		{"(id == 1 || id == 3) and maas >= 75000.5", 2},
	}
	for _, c := range cases {
		matched, _, err := filterRows(testRows(), c.cond, knownCols(), nil)
		if err != nil {
			t.Fatalf("%q: beklenmeyen hata: %v", c.cond, err)
		}
		if len(matched) != c.want {
			t.Errorf("%q: %d satır bekleniyordu, %d alındı", c.cond, c.want, len(matched))
		}
	}
}

func TestRowConditionRejectsCodeExecution(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pwned.txt")
	// Eski enterpolasyon saldırısı: metin içindeki süslü parantezler artık düz metindir
	interp := fmt.Sprintf(`ad == '{dosya_yaz(%q, "RCE")}'`, marker)
	if matched, _, err := filterRows(testRows(), interp, knownCols(), nil); err != nil || len(matched) != 0 {
		t.Errorf("%q: 0 eşleşme ve hatasız sonuç bekleniyordu (err=%v, n=%d)", interp, err, len(matched))
	}
	// Fonksiyon çağrısı ve üye erişimi tamamen reddedilmeli
	for _, cond := range []string{
		fmt.Sprintf(`dosya_yaz(%q, "RCE")`, marker),
		`python_çalıştır("print(1)")`,
		`ad.uzunluk > 0`,
	} {
		if _, _, err := filterRows(testRows(), cond, knownCols(), nil); err == nil {
			t.Errorf("%q: güvenli olmayan koşul reddedilmeliydi", cond)
		}
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("koşul içinden dosya_yaz çalıştı: kod yürütme açığı hâlâ var")
	}
}

func TestRowConditionParamsPreventInjection(t *testing.T) {
	evil := &StringVal{Val: "x' veya doğru veya 'y"}
	matched, _, err := filterRows(testRows(), "departman == ?", knownCols(), []Value{evil})
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 0 {
		t.Fatalf("parametre enjeksiyonu %d satır döndürdü, 0 bekleniyordu", len(matched))
	}
	matched, _, err = filterRows(testRows(), "departman == ? ve maas > ?", knownCols(),
		[]Value{&StringVal{Val: "Yapay Zeka"}, &IntVal{Val: 80000}})
	if err != nil || len(matched) != 1 {
		t.Fatalf("parametreli sorgu 1 satır döndürmeliydi (err=%v, n=%d)", err, len(matched))
	}
	if _, _, err := filterRows(testRows(), "id == ? ve id == ?", knownCols(), []Value{&IntVal{Val: 1}}); err == nil {
		t.Fatal("eksik parametre hata vermeliydi")
	}
}

func TestRowConditionErrorsAreReported(t *testing.T) {
	for _, cond := range []string{"maaas > 10", "maas >", "ad == 'kapanmamış", "ad > 5"} {
		if _, _, err := filterRows(testRows(), cond, knownCols(), nil); err == nil {
			t.Errorf("%q: hata bekleniyordu", cond)
		}
	}
}

func TestSQLParamsFromTR(t *testing.T) {
	val, err := runTR(t, `
		değişken vt = veritabanı_aç(":bellek:")
		vt.ekle("personel", [
			{"id": 1, "ad": "Ayşe", "departman": "Yapay Zeka", "maas": 92000},
			{"id": 2, "ad": "Ali", "departman": "Satış", "maas": 60000}
		])
		değişken kotu = "x' veya doğru veya 'y"
		değişken a = vt.sorgula("SEÇ ad TABLO personel NEREDE departman == ?", [kotu])
		değişken b = vt.sorgula("SEÇ ad TABLO personel NEREDE departman == ? SIRALA maas AZALAN", ["Satış"])
		değişken c = vt.say("personel", "maas > ?", [70000])
		değişken d = vt.güncelle("personel", {"maas": 61000}, "id == ?", [2])
		değişken e = tablo_sorgula(vt.tablo_al("personel"), "maas < ?", [70000])
		// Koşul içinde sayma: eskiden kilitlenme (deadlock) yapıyordu
		değişken f = vt.sil("personel", "id == ?", [vt.say("personel")])
		sonuç [a.uzunluk, b[0]["ad"], c, d, e[0]["maas"], f, vt.say("personel")]
	`)
	if err != nil {
		t.Fatalf("SQL hatası: %v", err)
	}
	if got, want := val.Inspect(), `[0, "Ali", 1, 1, 61000, 1, 1]`; got != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, got)
	}
}

// ---------------------------------------------------------------------------
// Görsel debugger: yalnızca 127.0.0.1, erişim anahtarı ve Host denetimi
// ---------------------------------------------------------------------------

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestWebDebuggerRequiresTokenAndLocalhost(t *testing.T) {
	machine := New()
	machine.Out = io.Discard
	machine.GlobalEnv.Define("bakiye", &IntVal{Val: 10}, false)

	d := NewDebugger("test.tr", "değişken bakiye = 10", true)
	port := freePort(t)
	if err := d.StartWebServer(machine, port); err != nil {
		t.Fatal(err)
	}
	defer d.StopWebServer()
	// VM'nin bir satırda duraklatılıp web komutu beklemesini taklit et
	resumed := make(chan struct{})
	go func() {
		d.waitForWebAction(machine, 1, machine.GlobalEnv)
		close(resumed)
	}()
	waitUntil(t, func() bool { d.mu.Lock(); defer d.mu.Unlock(); return d.paused })

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	marker := filepath.Join(t.TempDir(), "dbg_pwned.txt")
	evilBody := fmt.Sprintf(`{"name":"bakiye","value":"dosya_yaz(%q, \"x\")"}`, strings.ReplaceAll(marker, `\`, `\\`))

	do := func(method, path, body, token, host string) int {
		req, _ := http.NewRequest(method, base+path, strings.NewReader(body))
		if token != "" {
			req.Header.Set(debugTokenHeader, token)
		}
		if host != "" {
			req.Host = host
		}
		req.Header.Set("Content-Type", "text/plain") // tarayıcının ön kontrolsüz gönderebileceği tür
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}

	if c := do("GET", "/", "", "", ""); c != http.StatusUnauthorized {
		t.Errorf("anahtarsız ana sayfa 401 vermeliydi, %d", c)
	}
	if c := do("GET", "/?anahtar="+d.webToken, "", "", ""); c != http.StatusOK {
		t.Errorf("anahtarlı ana sayfa 200 vermeliydi, %d", c)
	}
	if c := do("GET", "/api/state", "", "", ""); c != http.StatusUnauthorized {
		t.Errorf("anahtarsız /api/state 401 vermeliydi, %d", c)
	}
	if c := do("POST", "/api/modify", evilBody, "", ""); c != http.StatusUnauthorized {
		t.Errorf("anahtarsız /api/modify 401 vermeliydi, %d", c)
	}
	if c := do("POST", "/api/modify", evilBody, "yanlis-anahtar", ""); c != http.StatusUnauthorized {
		t.Errorf("hatalı anahtarlı /api/modify 401 vermeliydi, %d", c)
	}
	if c := do("GET", "/api/state", "", d.webToken, "kotu-site.example:"+fmt.Sprint(port)); c != http.StatusForbidden {
		t.Errorf("yabancı Host (DNS rebinding) 403 vermeliydi, %d", c)
	}
	if c := do("GET", "/api/modify", "", d.webToken, ""); c != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/modify 405 vermeliydi, %d", c)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("yetkisiz istek kod çalıştırdı")
	}
	if v, _ := machine.GlobalEnv.Get("bakiye"); v.Inspect() != "10" {
		t.Fatalf("yetkisiz istek değişkeni değiştirdi: %s", v.Inspect())
	}

	// Yetkili istek hâlâ çalışmalı (ABAP canlı değişken değiştirme)
	if c := do("POST", "/api/modify", `{"name":"bakiye","value":"42"}`, d.webToken, ""); c != http.StatusOK {
		t.Fatalf("yetkili /api/modify 200 vermeliydi, %d", c)
	}
	if v, _ := machine.GlobalEnv.Get("bakiye"); v.Inspect() != "42" {
		t.Fatalf("yetkili değişiklik uygulanmadı: %s", v.Inspect())
	}

	// F8 ile devam ettikten sonra (program çalışırken) değişiklik ve adım komutları reddedilmeli
	if c := do("POST", "/api/step", `{"action":"f8"}`, d.webToken, ""); c != http.StatusOK {
		t.Fatalf("f8 200 vermeliydi, %d", c)
	}
	<-resumed
	if c := do("POST", "/api/modify", `{"name":"bakiye","value":"7"}`, d.webToken, ""); c != http.StatusConflict {
		t.Errorf("çalışırken /api/modify 409 vermeliydi, %d", c)
	}
	if c := do("POST", "/api/step", `{"action":"f5"}`, d.webToken, ""); c != http.StatusConflict {
		t.Errorf("çalışırken /api/step 409 vermeliydi (hayalet adım), %d", c)
	}
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("koşul 2 saniye içinde sağlanmadı")
}

// ---------------------------------------------------------------------------
// GUI HTML çıktısı: renk ve form alanı adları üzerinden enjeksiyon olmamalı
// ---------------------------------------------------------------------------

func TestGUIRenderEscapesColorAndFieldNames(t *testing.T) {
	g := &GUIAppVal{
		Title: "<b>Başlık</b>",
		KPIs: []guiWidget{{
			Kind: "kpi", Title: "Ciro", Value: "1", Color: `red;"><script>alert(1)</script>`,
		}},
		Widgets: []guiWidget{{
			Kind: "form", ID: "form_1", Title: "Form",
			Fields: []string{`a');alert(1);//`, `"><img src=x onerror=alert(2)>`},
		}},
	}
	out := g.renderHTML()
	for _, bad := range []string{"<script>alert(1)", "<img src=x", "'a');alert(1);//'", "<b>Başlık</b>"} {
		if strings.Contains(out, bad) {
			t.Errorf("HTML çıktısı kaçışlanmamış içerik barındırıyor: %q", bad)
		}
	}
	if !strings.Contains(out, "border-left-color: #38bdf8") {
		t.Error("güvenli olmayan renk varsayılan renkle değiştirilmeliydi")
	}
	if resolveColor("yeşil") != "#22c55e" || resolveColor("#abc") != "#abc" || resolveColor("rgb(1, 2, 3)") != "rgb(1, 2, 3)" {
		t.Error("geçerli renkler korunmalı")
	}
}

package web

// Hakem denetiminin (veri_web.md 11, 12, 16, 17, 18, 25; mimari.md 21) bulduğu kusurların
// regresyon testleri. Sunucular yalnızca 127.0.0.1'de açılır; dış ağa bağlanılmaz.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/vm"
)

// sunucuKur TR kodunu çalıştırır; kod son değer olarak arka planda başlatılmış bir
// WebServerVal döndürmelidir.
func sunucuKur(t *testing.T, kod string) (*vm.VM, *WebServerVal) {
	t.Helper()
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	machine := vm.New()
	machine.Out = io.Discard
	val, err := machine.Run(prog)
	if err != nil {
		t.Fatal(err)
	}
	srv := val.(*WebServerVal)
	t.Cleanup(func() { srv.server.Close() })
	return machine, srv
}

func istekAt(t *testing.T, yontem, adres, govde string, basliklar map[string]string) (int, string, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(yontem, adres, strings.NewReader(govde))
	for k, v := range basliklar {
		if k == "Host" {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b), res.Header
}

// 11: statik klasör nokta dosyalarını sunuyor, dizin listeliyor, bağlantı ile klasör dışına
// çıkıyordu; "/" önekiyle de yakalanamayan bir panik veriyordu.
func TestStatikKlasorGuvenli(t *testing.T) {
	kok := t.TempDir()
	statik := filepath.Join(kok, "statik")
	disari := filepath.Join(kok, "gizli")
	for _, d := range []string{filepath.Join(statik, "alt"), filepath.Join(statik, ".git"), filepath.Join(statik, "sayfa"), disari} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	yaz := func(yol, icerik string) {
		if err := os.WriteFile(yol, []byte(icerik), 0644); err != nil {
			t.Fatal(err)
		}
	}
	yaz(filepath.Join(statik, "a.txt"), "herkese açık")
	yaz(filepath.Join(statik, ".env"), "GIZLI_ANAHTAR=123")
	yaz(filepath.Join(statik, ".git", "config"), "gizli ayar")
	yaz(filepath.Join(statik, "alt", "b.txt"), "alt dosya")
	yaz(filepath.Join(statik, "sayfa", "index.html"), "<!DOCTYPE html><p>dizin sayfası</p>")
	yaz(filepath.Join(disari, "sir.txt"), "klasör dışı sır")
	bagVar := false
	bag := filepath.Join(statik, "bag")
	if err := os.Symlink(disari, bag); err == nil {
		bagVar = true
	} else if runtime.GOOS == "windows" {
		if exec.Command("cmd", "/c", "mklink", "/J", bag, disari).Run() == nil {
			bagVar = true
		}
	}

	port := freePort(t)
	_, _ = sunucuKur(t, fmt.Sprintf(`
		değişken s = web_sunucu(port: %d)
		s.statik_klasör("/dosyalar/", %q)
		s.statik_klasör("/", %q)
		s.al("/api", fonksiyon(istek) { sonuç {"durum": "rota"} })
		s.arka_planda_başlat()
		sonuç s`, port, filepath.ToSlash(statik), filepath.ToSlash(filepath.Join(statik, "sayfa"))))
	taban := fmt.Sprintf("http://127.0.0.1:%d", port)
	for yol, bekle := range map[string]int{
		"/dosyalar/a.txt":                200,
		"/dosyalar/alt/b.txt":            200,
		"/dosyalar/.env":                 404,
		"/dosyalar/.git/config":          404,
		"/dosyalar/alt/":                 404, // dizin listelenmez
		"/dosyalar/":                     404,
		"/dosyalar/..%2fgizli/sir.txt":   404,
		"/dosyalar/..%5cgizli%5csir.txt": 404,
		"/dosyalar/a.txt::$DATA":         404,
		"/":                              200, // "/" öneki: sayfa/index.html
		"/api":                           200, // rotalar statik klasörden önce gelir
	} {
		durum, govde, _ := istekAt(t, "GET", taban+yol, "", nil)
		if durum != bekle {
			t.Errorf("%s: %d bekleniyordu, %d alındı (%q)", yol, bekle, durum, govde)
		}
		if strings.Contains(govde, "GIZLI") || strings.Contains(govde, "sır") || strings.Contains(govde, "gizli ayar") {
			t.Errorf("%s: gizli içerik sızdı: %q", yol, govde)
		}
	}
	if _, govde, _ := istekAt(t, "GET", taban+"/", "", nil); !strings.Contains(govde, "dizin sayfası") {
		t.Errorf("/ index.html sunmalı: %q", govde)
	}
	if bagVar {
		if durum, govde, _ := istekAt(t, "GET", taban+"/dosyalar/bag/sir.txt", "", nil); durum != 404 {
			t.Errorf("bağlantı ile klasör dışına çıkılmamalı: %d %q", durum, govde)
		}
	} else {
		t.Log("bağlantı oluşturulamadı; bağlantı denetimi atlandı")
	}

	// Olmayan klasör açık hata verir
	_, _, err := runTRWithin(t, 5*time.Second, `web_sunucu(port: 18999).statik_klasör("/x/", "olmayan_statik_klasor")`)
	if err == nil || !strings.Contains(err.Error(), "statik_klasör") {
		t.Fatalf("olmayan klasör hatası bekleniyordu: %v", err)
	}
}

// 16: başka siteden gelen GET istekleri işleyiciyi çalıştırıyordu (kılavuz §6 "korunur" diyordu).
func TestBaskaSitedenGetReddedilir(t *testing.T) {
	port := freePort(t)
	machine, _ := sunucuKur(t, fmt.Sprintf(`
		değişken silinen = 0
		değişken s = web_sunucu(port: %d)
		s.al("/sil", fonksiyon(istek) {
			silinen += 1
			sonuç "silindi"
		})
		s.arka_planda_başlat()
		sonuç s`, port))
	taban := fmt.Sprintf("http://127.0.0.1:%d", port)
	for _, b := range []map[string]string{
		{"Sec-Fetch-Site": "cross-site"},
		{"Sec-Fetch-Site": "same-site"},
		{"Origin": "https://saldirgan.example"},
		{"Referer": "https://saldirgan.example/sayfa"},
	} {
		if durum, _, _ := istekAt(t, "GET", taban+"/sil", "", b); durum != http.StatusForbidden {
			t.Errorf("%v: 403 bekleniyordu, %d", b, durum)
		}
	}
	for _, b := range []map[string]string{
		nil,
		{"Sec-Fetch-Site": "same-origin"},
		{"Sec-Fetch-Site": "none"},
		{"Referer": taban + "/"},
	} {
		if durum, _, _ := istekAt(t, "GET", taban+"/sil", "", b); durum != 200 {
			t.Errorf("%v: 200 bekleniyordu, %d", b, durum)
		}
	}
	if v, _ := machine.GlobalEnv.Get("silinen"); v.Inspect() != "4" {
		t.Fatalf("işleyici yalnızca 4 yerel istekte çalışmalı: %s", v.Inspect())
	}
}

// 17: sunucu geçersiz ya da boş JSON üretiyordu (%q JSON kaçışı değil, MarshalIndent hatası
// yok sayılıyordu) ve iç hata metni (satır, kod çerçevesi) istemciye sızıyordu.
func TestSunucuYanitlariGecerliJSON(t *testing.T) {
	port := freePort(t)
	eskiGunluk := hataGunlugu
	var gunluk bytes.Buffer
	var gunlukMu sync.Mutex
	hataGunlugu = kilitliYazici{&gunluk, &gunlukMu}
	defer func() { hataGunlugu = eskiGunluk }()
	_, _ = sunucuKur(t, fmt.Sprintf(`
		değişken s = web_sunucu(port: %d)
		s.al("/nan", fonksiyon(istek) { sonuç {"durum": "tamam", "deger": karekök(-1.0)} })
		s.al("/hata", fonksiyon(istek) { fırlat("gizli iç ayrıntı") })
		s.al("/sira", fonksiyon(istek) { sonuç {"z": 1, "a": 2} })
		s.al("/dongu", fonksiyon(istek) {
			değişken l = [1]
			l.ekle(l)
			sonuç l
		})
		s.arka_planda_başlat()
		sonuç s`, port))
	taban := fmt.Sprintf("http://127.0.0.1:%d", port)
	for yol, bekle := range map[string]int{"/x%01y": 404, "/nan": 500, "/hata": 500, "/dongu": 500, "/sira": 200} {
		durum, govde, h := istekAt(t, "GET", taban+yol, "", nil)
		if durum != bekle || !json.Valid([]byte(govde)) || !strings.HasPrefix(h.Get("Content-Type"), "application/json") {
			t.Errorf("%s: %d ve geçerli JSON bekleniyordu: %d %q", yol, bekle, durum, govde)
		}
		if strings.Contains(govde, "Satır") || strings.Contains(govde, "gizli iç ayrıntı") {
			t.Errorf("%s: iç hata metni istemciye sızmamalı: %q", yol, govde)
		}
		if yol == "/sira" && !strings.Contains(govde, `"z": 1,`) {
			t.Errorf("anahtar sırası korunmalı: %q", govde)
		}
	}
	gunlukMu.Lock()
	defer gunlukMu.Unlock()
	if !strings.Contains(gunluk.String(), "gizli iç ayrıntı") {
		t.Errorf("iç hata sunucu tarafında yazılmalı: %q", gunluk.String())
	}
}

type kilitliYazici struct {
	b  *bytes.Buffer
	mu *sync.Mutex
}

func (k kilitliYazici) Write(p []byte) (int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.b.Write(p)
}

// 18: port: 0 "localhost:0" diye dinlemeye çalışıp her isteğe 403 veriyordu; geçersiz port
// açık hata vermeli.
func TestPortDogrulanir(t *testing.T) {
	for _, kod := range []string{
		`web_sunucu(port: 0)`,
		`web_sunucu(port: 70000)`,
		`web_sunucu(port: "8080")`,
		`web_sunucu(-1)`,
		`arayüz_oluştur("a").başlat(port: 0, pencere_aç: yanlış)`,
		`arayüz_oluştur("a").arka_planda_başlat(port: 65536)`,
	} {
		_, _, err := runTRWithin(t, 5*time.Second, "dene { "+kod+" } yakala h { fırlat(h) }")
		if err == nil || !strings.Contains(err.Error(), "1 ile 65535") {
			t.Errorf("%s: port hatası bekleniyordu: %v", kod, err)
		}
	}
}

// 12: 50 MB'lık yanıt sınırı belleği sınırlamıyordu (49 MB "[0,0,…]" → 26 M eleman, ~1 GB
// yığın) ve JSON dönüşümü 30 sn'lik zaman aşımının dışında kalıyordu.
func TestIstemciJSONDonusumSinirli(t *testing.T) {
	eskiSinir := EnFazlaJSONDegeri
	EnFazlaJSONDegeri = 1000
	defer func() { EnFazlaJSONDegeri = eskiSinir }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := 999
		if r.URL.Path == "/buyuk" {
			n = 1001
		}
		fmt.Fprint(w, "["+strings.TrimSuffix(strings.Repeat("0,", n), ",")+"]")
	}))
	defer srv.Close()
	_, val, err := runTRWithin(t, 10*time.Second, fmt.Sprintf(`sonuç uzunluk(web_al("%s/kucuk"))`, srv.URL))
	if err != nil || val.Inspect() != "999" {
		t.Fatalf("sınırın altındaki JSON çözülmeli: %v %v", val, err)
	}
	_, _, err = runTRWithin(t, 10*time.Second, fmt.Sprintf(`web_al("%s/buyuk")`, srv.URL))
	if err == nil || !strings.Contains(err.Error(), "çok fazla değer") {
		t.Fatalf("değer sınırı hatası bekleniyordu: %v", err)
	}
}

func TestIstemciJSONDonusumuZamanAsimiIcinde(t *testing.T) {
	if testing.Short() {
		t.Skip("kısa kipte atlanır")
	}
	eski := webZamanAsimi
	webZamanAsimi = 100 * time.Millisecond
	defer func() { webZamanAsimi = eski }()
	govde := "[" + strings.TrimSuffix(strings.Repeat("{\"a\":[0,0]},", 400_000), ",") + "]"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, govde)
	}))
	defer srv.Close()
	bas := time.Now()
	_, _, err := runTRWithin(t, 30*time.Second, fmt.Sprintf(`web_al("%s/")`, srv.URL))
	if err == nil || !strings.Contains(err.Error(), "zaman aşımı") {
		t.Fatalf("dönüşüm de zaman aşımına girmeli: %v (%v)", err, time.Since(bas))
	}
}

// web_gönder gövdesi de sıralı ve döngü denetimli JSON yazıcısından geçer.
func TestWebGonderSiraVeDongu(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.Write(b)
	}))
	defer srv.Close()
	_, val, err := runTRWithin(t, 10*time.Second, fmt.Sprintf(`
		değişken l = [1]
		l.ekle(l)
		değişken m = ""
		dene { web_gönder("%[1]s/", l) } yakala h { m = "{h}" }
		sonuç [web_gönder("%[1]s/", {"z": 1, "a": 2}).anahtarlar(), m]`, srv.URL))
	if err != nil || !strings.HasPrefix(val.Inspect(), `[["z", "a"], "`) || !strings.Contains(val.Inspect(), "kendini içeriyor") {
		t.Fatalf("%v %v", val, err)
	}
}

// 25 ve mimari 21: arayüz bileşen listesi /api/etkilesim tarafından yorumlayıcı kilidi
// olmadan geziliyordu (kart_ekle/form_ekle g.mu almıyordu); Host denetimi iki yerde farklı
// yazılmıştı ([::1] birinde kabul, ötekinde ret). Bileşen eklerken eşzamanlı etkileşim
// istekleri kilitlenmeden ve çökmeden yanıtlanmalı; geri çağrı bileşen ekleyebilmeli.
func TestArayuzEtkilesimEsZamanliVeTekHostDenetimi(t *testing.T) {
	port := freePort(t)
	p := parser.New(lexer.New(fmt.Sprintf(`
		değişken a = arayüz_oluştur("Deneme")
		a.buton_ekle("Ekle", fonksiyon(f) {
			a.metin_ekle("geri çağrıdan")
			sonuç "tamam"
		})
		a.arka_planda_başlat(port: %d, pencere_aç: yanlış)
		döngü i içinde 0..300 {
			a.kart_ekle("k{i}", i)
			a.form_ekle("f{i}", ["x"], fonksiyon(f) { sonuç f["x"] })
			bekle(0.001)
		}
		sonuç a`, port)))
	prog := p.ParseProgram()
	machine := vm.New()
	machine.Out = io.Discard
	bitti := make(chan error, 1)
	var g *GUIAppVal
	go func() {
		val, err := machine.Run(prog)
		if err == nil {
			g = val.(*GUIAppVal)
		}
		bitti <- err
	}()
	adres := fmt.Sprintf("http://127.0.0.1:%d/api/etkilesim", port)
	// Sunucunun açılmasını bekle
	for i := 0; i < 200; i++ {
		if res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port)); err == nil {
			res.Body.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var wg sync.WaitGroup
	hatalar := make(chan string, 100)
	for k := 0; k < 4; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				host := "127.0.0.1"
				if i%2 == 1 {
					host = "[::1]"
				}
				req, _ := http.NewRequest("POST", adres, strings.NewReader(`{"widget_id":"btn_1","values":{}}`))
				req.Host = fmt.Sprintf("%s:%d", host, port)
				req.Header.Set("Content-Type", "application/json")
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					hatalar <- err.Error()
					return
				}
				b, _ := io.ReadAll(res.Body)
				res.Body.Close()
				if res.StatusCode != 200 || !json.Valid(b) || !strings.Contains(string(b), "tamam") {
					hatalar <- fmt.Sprintf("%s: %d %s", host, res.StatusCode, b)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(hatalar)
	for h := range hatalar {
		t.Error(h)
	}
	select {
	case err := <-bitti:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("program bitmedi (kilitlenme?)")
	}
	defer g.server.Close()
	g.mu.Lock()
	n := len(g.Widgets)
	g.mu.Unlock()
	if n < 300 {
		t.Fatalf("bileşenler eklenmeli: %d", n)
	}
}

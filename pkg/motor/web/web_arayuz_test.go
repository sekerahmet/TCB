package web

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Web istek işleyicileri ana programla aynı anda paylaşılan değerlere yazar; ayrıca program
// kendi sunucusuna web_al ile istek atar (kilit ağ beklemesinde bırakılmazsa kilitlenirdi).
func TestWebServerHandlersShareStateSafely(t *testing.T) {
	port := freePort(t)
	_, val, err := runTRWithin(t, 30*time.Second, fmt.Sprintf(`
		değişken sayac = 0
		değişken kayit = {}
		değişken api = web_sunucu(port: %d)
		api.al("/artir", fonksiyon(istek) {
			sayac += 1
			kayit[metin(sayac)] = istek["parametreler"]["n"]
			sonuç {"sayac": sayac}
		})
		api.arka_planda_başlat()
		döngü i içinde 0..40 {
			eşzamanlı { web_al("http://127.0.0.1:%d/artir?n={i}") }
		}
		bekle_hepsi()
		değişken son = web_al("http://127.0.0.1:%d/artir?n=son")
		api.durdur()
		sonuç [sayac, kayit.anahtarlar().uzunluk, son["sayac"]]`, port, port, port))
	if err != nil {
		t.Fatal(err)
	}
	if val.Inspect() != "[41, 41, 41]" {
		t.Fatalf("[41, 41, 41] bekleniyordu, %s alındı", val.Inspect())
	}
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
	if resolveColor("yeşil") != "#22c55e" || resolveColor("YEŞİL") != "#22c55e" || resolveColor("#abc") != "#abc" || resolveColor("rgb(1, 2, 3)") != "rgb(1, 2, 3)" {
		t.Error("geçerli renkler korunmalı")
	}
}

// Renk adları yalnızca Türkçedir: İngilizce ve ASCII yazımlar Türkçe adı önerir.
func TestRenkAdlariTurkce(t *testing.T) {
	for renk, want := range map[string]string{
		"green":   "'green' TR'de yok; 'yeşil' yazmak mı istediniz?",
		"Red":     "'Red' TR'de yok; 'kırmızı' yazmak mı istediniz?",
		"yesil":   "'yesil' TR'de yok; 'yeşil' yazmak mı istediniz?",
		"kirmizi": "'kirmizi' TR'de yok; 'kırmızı' yazmak mı istediniz?",
		"teal":    "bilinmeyen renk \"teal\"",
	} {
		if _, err := renkCoz(renk); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %q içeren hata bekleniyordu: %v", renk, want, err)
		}
	}
	_, _, err := runTRWithin(t, 5*time.Second, `değişken a = arayüz_oluştur("x")
a.kart_ekle("b", "1", "c", "green")`)
	if err == nil || !strings.Contains(err.Error(), "kart_ekle: 'green' TR'de yok; 'yeşil' yazmak mı istediniz?") {
		t.Errorf("kart_ekle İngilizce rengi reddetmeli: %v", err)
	}
}

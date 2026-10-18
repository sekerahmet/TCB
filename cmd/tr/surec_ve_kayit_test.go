package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Komut satırı seçenekleri (docs/dil/19_surec_ve_kayit.md): argümanlar komut_satırı'na
// gelir; --yardım standart çıktıya yazılır ve 0 ile, hatalı argüman standart hataya yazılır
// ve 2 ile biter. İkisinde de ertele blokları çalışır; argüman uyarısı verilmez.
func TestKomutSatiriSecenekleri(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "egit.tr", `ertele yazdır("ertelenen")
değişken ayar = komut_satırı({adım: 10, oran: 0.5, hızlı: yanlış}, açıklama: "Deneme eğitimi")
yazdır("adım={ayar.adım} oran={ayar.oran} hızlı={ayar.hızlı}")
`)
	for _, komut := range [][]string{
		{"egit.tr", "--adım", "20", "--oran=0.25", "--hızlı"},
		{"çalıştır", "egit.tr", "--adım", "20", "--oran=0.25", "--hızlı"},
		{"hata_ayıkla", "egit.tr", "--adım", "20", "--oran=0.25", "--hızlı"},
	} {
		s := tr(t, klasor, komut...)
		bekle(t, s, 0, []string{"adım=20 oran=0.25 hızlı=doğru", "ertelenen"}, nil)
		if strings.Contains(s.hata, "Uyarı") {
			t.Errorf("%v: okunan argümanlar için uyarı olmamalı: %q", komut, s.hata)
		}
	}
	s := tr(t, klasor, "egit.tr", "--yardım")
	bekle(t, s, 0, []string{"Kullanım: tr egit.tr [seçenekler]", "Deneme eğitimi", "--adım <Tam>", "(varsayılan: 10)", "ertelenen"}, nil)
	if s.hata != "" || strings.Contains(s.cikti, "adım=") {
		t.Errorf("yardım programı bitirmeli, standart hata boş kalmalı: %+v", s)
	}
	s = tr(t, klasor, "egit.tr", "--adim", "5")
	bekle(t, s, 2, []string{"ertelenen"}, []string{"Hata: bilinmeyen seçenek '--adim'. Bunu mu demek istediniz: '--adım'? Seçenekler için: tr egit.tr --yardım"})
	if strings.Contains(s.hata, "Çalışma Zamanı") || strings.Contains(s.cikti, "Hata:") {
		t.Errorf("kullanım hatası çalışma hatası gibi yazılmamalı: %+v", s)
	}
	// Derlenmiş uygulamada kullanım satırı uygulamanın adını gösterir.
	cikis := filepath.Join(t.TempDir(), "uyg.exe")
	bekle(t, tr(t, klasor, "derle", "egit.tr", cikis), 0, nil, nil)
	bekle(t, calistir(t, cikis, klasor, "", nil, "-h"), 0, []string{"Kullanım: uyg.exe [seçenekler]"}, nil)
	bekle(t, calistir(t, cikis, klasor, "", nil, "--oran", "0,5"), 2, nil, []string{"ondalık ayırıcı nokta olmalı", "Seçenekler için: uyg.exe --yardım"})
}

// Gizli değerler: gizli adlı ortam değişkeninin ve seçeneğin değeri yazdır'da, hata
// mesajında ve günlükte *** olur; dosyaya yazılan veri (dosya_yaz) değişmez.
func TestGizliDegerlerCiktidaMaskelenir(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "g.tr", `değişken ayar = komut_satırı({api_key: ""})
değişken belirteç = ortam_değişkeni("HF_TOKEN")
yazdır("belirteç:", belirteç, "anahtar:", ayar.api_key)
değişken g = günlük("kayit/g.jsonl")
g.ekle({belirteç: belirteç})
dosya_yaz("veri.txt", belirteç)
fırlat "bozuk: " + ayar.api_key
`)
	s := calistir(t, trIkilisi, klasor, "", []string{"HF_TOKEN=hf_cokgizli123"}, "g.tr", "--api_key", "sk-anahtar-456")
	bekle(t, s, 1, []string{"belirteç: *** anahtar: ***"}, []string{"bozuk: ***"})
	for _, gizli := range []string{"hf_cokgizli123", "sk-anahtar-456"} {
		if strings.Contains(s.cikti+s.hata, gizli) {
			t.Errorf("%q çıktıda görünmemeli:\n%s\n%s", gizli, s.cikti, s.hata)
		}
	}
	gunlukIcerik, _ := os.ReadFile(filepath.Join(klasor, "kayit", "g.jsonl"))
	if strings.Contains(string(gunlukIcerik), "hf_cokgizli123") || !strings.Contains(string(gunlukIcerik), `"belirteç":"***"`) {
		t.Errorf("günlükte gizli değer maskelenmeli:\n%s", gunlukIcerik)
	}
	if veri, _ := os.ReadFile(filepath.Join(klasor, "veri.txt")); string(veri) != "hf_cokgizli123" {
		t.Errorf("veri çıktısı değişmemeli: %q", veri)
	}
}

// İlerleme çubuğu boruya (terminal değil) yazarken standart çıktıya karışmaz, sonda özet
// satırı yazılır; program hatayla bitse de çubuk biter, bitiş günlüğe yazılır.
func TestIlerlemeKomutSatirinda(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "i.tr", `değişken g = günlük("i.jsonl")
değişken ç = ilerleme(10, günlük: g)
döngü i içinde 0..4 { ç.güncelle({kayıp: 1.5}) }
yazdır("bitti mi:", ç)
fırlat "patladı"
`)
	s := tr(t, klasor, "i.tr")
	bekle(t, s, 1, []string{"bitti mi: İlerleme(adım 4/10)"}, []string{"adım 4/10 · kayıp 1,5 ·", "sürdü\n❌ Çalışma Zamanı Hatası:", "patladı"})
	if strings.Contains(s.cikti, "adım 4/10 ·") || strings.Contains(s.hata, "\r") {
		t.Errorf("çubuk standart çıktıya ve (terminal değilken) satır başı karakteriyle yazılmamalı: %+v", s)
	}
	icerik, _ := os.ReadFile(filepath.Join(klasor, "i.jsonl"))
	if !strings.Contains(string(icerik), `"tür":"bitiş"`) {
		t.Errorf("bitiş kaydı yazılmalı:\n%s", icerik)
	}
}

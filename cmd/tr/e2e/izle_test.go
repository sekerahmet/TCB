package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// izleEgitimi 60 adımlık kısa bir eğitimdir (tr izle / tr profil alt süreç testleri).
const izleEgitimi = `değişken w = tensör(rastgele_normal([128, 128], tohum: 1), türev_izle: doğru)
değişken x = rastgele_normal([64, 128], tohum: 2)
değişken op = AdamW([w], öğrenme_oranı: 0.01)
değişken ç = ilerleme(60)
döngü i içinde 0..60 {
    op.sıfırla()
    değişken kayıp = toplam(relu(x @ w))
    geri_yayılım(kayıp)
    op.adım()
    ç.güncelle({kayıp: kayıp})
}
yazdır("eğitim bitti")
`

// orneklemeSayisi raporun "Örnekleme: ... aralıkla N örnek" satırındaki örnek sayısı
// (binlik ayırıcı boşluk olabilir).
var orneklemeSayisi = regexp.MustCompile(`aralıkla ([0-9 \x{00a0}]+) örnek`)

// satirOrnekDenetle programın kendi satırının (satir, ör. "egit.tr:") sıcak satırlarda
// görünmesini, rapor yeterince örnek topladıysa ister. Test programı birkaç on milisaniye
// sürer: yüklü bir makinede (CI, Windows) örnekleyici o dosyanın satırlarına hiç denk
// gelmeyebilir; o zaman yalnızca örnek sayısının raporda bildirildiği denetlenir.
func satirOrnekDenetle(t *testing.T, rapor, satir string) {
	t.Helper()
	m := orneklemeSayisi.FindStringSubmatch(rapor)
	if m == nil {
		t.Fatalf("raporda örnekleme satırı yok:\n%s", rapor)
	}
	n, _ := strconv.Atoi(strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, m[1]))
	if n >= 20 && !strings.Contains(rapor, satir) {
		t.Errorf("%d örnekte %q satırı yok:\n%s", n, satir, rapor)
	}
}

// raporOku speed.json/profil.json dosyasını çözer.
func raporOku(t *testing.T, yol string) map[string]any {
	t.Helper()
	veri, err := os.ReadFile(yol)
	if err != nil {
		t.Fatalf("rapor dosyası okunamadı: %v", err)
	}
	var d map[string]any
	if err := json.Unmarshal(veri, &d); err != nil {
		t.Fatalf("rapor geçerli JSON değil: %v\n%s", err, veri)
	}
	return d
}

// islemCagrisi rapordaki bir işlemin (kod, yön) çağrı sayısıdır.
func islemCagrisi(d map[string]any, kod, yon string) float64 {
	for _, x := range d["tensör"].(map[string]any)["işlemler"].([]any) {
		b := x.(map[string]any)
		if b["kod"] == kod && b["yön"] == yon {
			return b["çağrı"].(float64)
		}
	}
	return -1
}

// tr profil: metin rapor standart çıktıya, JSON rapor koşu klasörüne (speed.json).
func TestProfilKomutu(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "egit.tr", izleEgitimi)
	kosu := filepath.Join(klasor, "runs", "deneme1") + string(os.PathSeparator)
	s := tr(t, klasor, "profil", "--json", kosu, "egit.tr")
	bekle(t, s, 0, []string{"eğitim bitti", "⏱ Profil: egit.tr", "Adımlar: 60 adım", "Tensör işlemleri",
		"matris çarpımı (@)", "geri · matris çarpımı (@)", "AdamW.adım", "En çok zaman alan satırlar"},
		[]string{"speed.json"})
	satirOrnekDenetle(t, s.cikti+s.hata, "egit.tr:")
	d := raporOku(t, filepath.Join(kosu, "speed.json"))
	if d["biçim"] != "tr-profil" || d["tür"] != "rapor" || d["çıkış_kodu"] != float64(0) {
		t.Errorf("rapor kimliği: %v %v %v", d["biçim"], d["tür"], d["çıkış_kodu"])
	}
	for _, x := range []struct {
		kod, yon string
		n        float64
	}{{"matris_carpimi", "ileri", 60}, {"matris_carpimi", "geri", 60}, {"adamw", "ileri", 60}} {
		if n := islemCagrisi(d, x.kod, x.yon); n != x.n {
			t.Errorf("%s (%s) %v çağrı, beklenen %v", x.kod, x.yon, n, x.n)
		}
	}
	if d["örnekleme"] == nil {
		t.Error("tr profil satır örneklemesiyle çalışmalı")
	}
}

// tr profil program hata verse de raporu yazar; çıkış kodu programınkidir.
func TestProfilKomutuHatadaRaporlar(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "hata.tr", "değişken a = birler([2, 3])\nyazdır(a @ a)\n")
	s := tr(t, klasor, "profil", "--json", "rapor.json", "hata.tr")
	bekle(t, s, 1, []string{"⏱ Profil: hata.tr", "çıkış kodu 1"}, []string{"Çalışma Zamanı Hatası"})
	if d := raporOku(t, filepath.Join(klasor, "rapor.json")); d["çıkış_kodu"] != float64(1) {
		t.Errorf("rapordaki çıkış kodu %v", d["çıkış_kodu"])
	}
}

// tr izle terminal değilken: program çıktısı olduğu gibi akar, son rapor standart hataya.
func TestIzleKomutuTerminalDegil(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "egit.tr", izleEgitimi)
	s := tr(t, klasor, "izle", "egit.tr")
	bekle(t, s, 0, []string{"eğitim bitti"}, []string{"adım 60/60", "⏱ Profil: egit.tr", "Adımlar: 60 adım",
		"geri · matris çarpımı (@)", "En çok zaman alan satırlar"})
	satirOrnekDenetle(t, s.hata, "egit.tr:")
	if strings.Contains(s.hata, "\x1b[") {
		t.Error("terminal değilken denetim dizisi yazılmamalı")
	}
}

// Program ilerleme(...) kullanmıyorsa rapor nasıl kullanılacağını söyler.
func TestIzleAdimBilgisiYok(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "dongu.tr", "değişken s = 0\ndöngü i içinde 0..1000 {\n    s += i\n}\nyazdır(s)\n")
	s := tr(t, klasor, "izle", "dongu.tr")
	bekle(t, s, 0, []string{"499500"}, []string{"adım bilgisi yok: eğitim döngüsünde ilerleme(...) ve .güncelle(...) kullanın"})
}

// tr izle .tr olmayan bir adla (arka plan işini izleme henüz yok) açık hata verir.
func TestIzleIsAdiHenuzYok(t *testing.T) {
	s := tr(t, t.TempDir(), "izle", "deneme1")
	bekle(t, s, 1, nil, []string{"bir .tr dosyası değil", "tr izle egit.tr"})
	s = tr(t, t.TempDir(), "profil", "--bilinmeyen", "x.tr")
	bekle(t, s, 1, nil, []string{"bilinmeyen seçenek"})
}

// Arka plan işi (TR_İŞ) olarak çalışan program iş klasörüne profil.json yazar.
func TestIsProfilDosyasi(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "egit.tr", izleEgitimi)
	is := filepath.Join(klasor, ".tr", "işler", "deneme1")
	if err := os.MkdirAll(is, 0o755); err != nil {
		t.Fatal(err)
	}
	s := calistir(t, trIkilisi, klasor, "", []string{"TR_İŞ=" + is}, "egit.tr")
	bekle(t, s, 0, []string{"eğitim bitti"}, nil)
	d := raporOku(t, filepath.Join(is, "profil.json"))
	if d["bitti"] != true || d["örnekleme"] != nil {
		t.Errorf("profil.json: bitti %v, örnekleme %v (iş profilinde örnekleme kapalı)", d["bitti"], d["örnekleme"])
	}
	if n := islemCagrisi(d, "matris_carpimi", "ileri"); n != 60 {
		t.Errorf("iş profilinde matris çarpımı %v çağrı, beklenen 60", n)
	}
}

// gpuIzleEgitimi GPU'da (TR_GPU_BENZETIM=1 ile benzetimde) 8 adımlık küçük bir eğitimdir.
const gpuIzleEgitimi = `varsayılan_cihaz("gpu")
değişken w = tensör(rastgele_normal([16, 16], tohum: 1), türev_izle: doğru)
değişken x = rastgele_normal([8, 16], tohum: 2)
değişken op = AdamW([w], öğrenme_oranı: 0.01)
değişken ç = ilerleme(8)
döngü i içinde 0..8 {
    op.sıfırla()
    değişken kayıp = toplam(relu(x @ w))
    geri_yayılım(kayıp)
    op.adım()
    ç.güncelle({kayıp: kayıp})
}
yazdır("gpu eğitimi bitti")
`

// tr profil ve tr izle GPU'da: çekirdek süreleri ve GPU belleği rapora girer; GPU arka
// ucunun kendi raporu bir kez yazılır (ölçümü profil açtıysa raporun içinde, TR_GPU_PROFIL
// ile açıksa arka ucun kendisince).
func TestProfilVeIzleGPU(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "gpu.tr", gpuIzleEgitimi)
	for _, d := range []struct {
		ad, gpuProfil string
		args          []string
	}{
		{"profil", "", []string{"profil", "--json", "speed.json", "gpu.tr"}},
		{"izle", "", []string{"izle", "gpu.tr"}},
		{"profil_TR_GPU_PROFIL", "1", []string{"profil", "gpu.tr"}},
	} {
		t.Run(d.ad, func(t *testing.T) {
			s := calistir(t, trIkilisi, klasor, "", []string{"TR_GPU_BENZETIM=1", "TR_GPU_PROFIL=" + d.gpuProfil}, d.args...)
			if s.kod != 0 {
				t.Fatalf("çıkış kodu %d\nçıktı:\n%s\nhata:\n%s", s.kod, s.cikti, s.hata)
			}
			hepsi := s.cikti + s.hata
			for _, p := range []string{"gpu eğitimi bitti", "GPU çekirdekleri: cihazda", "carpim", "GPU canlı"} {
				if !strings.Contains(hepsi, p) {
					t.Errorf("çıktıda %q yok:\n%s", p, hepsi)
				}
			}
			if n := strings.Count(hepsi, "== GPU profili"); n != 1 {
				t.Errorf("GPU arka ucunun raporu %d kez yazıldı, beklenen 1:\n%s", n, hepsi)
			}
		})
	}
	d := raporOku(t, filepath.Join(klasor, "speed.json"))
	g := d["gpu"].(map[string]any)
	if g["kullanıldı"] != true || g["ölçüm"] != true || len(g["aileler"].([]any)) == 0 || len(g["bellek"].([]any)) == 0 {
		t.Errorf("speed.json GPU bölümü eksik: %v", g)
	}
}

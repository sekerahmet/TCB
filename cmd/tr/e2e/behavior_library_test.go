package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural 11.1 (güvenlik bulgusu O2): başka klasörden çalıştırılan programın göreli 'aktar'ı
// çalışma klasöründeki aynı adlı dosyayı değil, programın yanındakini yükler.
func TestAktarCalismaKlasorundekiDosyayiYuklemez(t *testing.T) {
	proje := t.TempDir()
	dosyaYaz(t, proje, "ana.tr", "aktar \"yardimci\"\nyazdır(kaynak)\n")
	dosyaYaz(t, proje, "yardimci.tr", "değişken kaynak = \"programın yanı\"\n")
	calisma := t.TempDir()
	dosyaYaz(t, calisma, "yardimci.tr", "değişken kaynak = \"SAHTE\"\n")
	s := tr(t, calisma, filepath.Join(proje, "ana.tr"))
	bekle(t, s, 0, []string{"programın yanı"}, nil)
	if strings.Contains(s.cikti, "SAHTE") {
		t.Error("çalışma klasöründeki dosya programın modülünün yerine geçti")
	}
	// tr kontrol ve tr derle aynı kuralla çözer.
	bekle(t, tr(t, calisma, "kontrol", filepath.Join(proje, "ana.tr")), 0, nil, nil)
}

// Kural 11.6: tr/ önekli modül gömülü kitaplıktandır; tr kontrol olmayanı bildirir, tr derle
// kitaplığı pakete gömmez (uygulama yorumlayıcının kopyasıdır) ve uygulama başka klasörde,
// diskte tr/katman.tr varken de kitaplıktakini yükler.
func TestKitaplikModuluKontrolVeDerle(t *testing.T) {
	proje := t.TempDir()
	dosyaYaz(t, proje, "ana.tr", strings.Join([]string{
		`aktar "tr/katman" olarak k`,
		`aktar "yerel"`,
		`değişken d = k.Doğrusal(3, 2, tohum: 1)`,
		`yazdır("şekil {d.ileri(birler([1, 3])).şekil} {YEREL}")`,
	}, "\n")+"\n")
	dosyaYaz(t, proje, "yerel.tr", "değişken YEREL = \"yerel modül\"\n")
	dosyaYaz(t, proje, "tr/katman.tr", "değişken Doğrusal = yok\n")
	beklenen := "şekil [1, 2] yerel modül"
	bekle(t, tr(t, proje, "ana.tr"), 0, []string{beklenen}, nil)

	cikis := filepath.Join(t.TempDir(), "uygulama.exe")
	s := tr(t, proje, "derle", "ana.tr", cikis)
	bekle(t, s, 0, []string{"Başarıyla derlendi", "Gömülen modüller (1)", "yerel.tr"}, nil)
	if strings.Contains(s.cikti, "katman") {
		t.Errorf("kitaplık modülü pakete gömülmemeli:\n%s", s.cikti)
	}
	baska := t.TempDir()
	dosyaYaz(t, baska, "tr/katman.tr", "değişken Doğrusal = yok\n")
	bekle(t, calistir(t, cikis, baska, "", nil), 0, []string{beklenen}, nil)

	dosyaYaz(t, proje, "hatali.tr", "aktar \"tr/olmayan\"\n")
	bekle(t, tr(t, proje, "kontrol", "hatali.tr"), 1, nil, []string{`"tr/olmayan" dilin kitaplığında yok; kitaplıktaki modüller: tr/iyilestirici, tr/katman`})
	bekle(t, tr(t, proje, "derle", "hatali.tr", cikis), 1, nil, []string{`"tr/olmayan" dilin kitaplığında yok`})
}

// Colab eğitim betiğinin (tools/gpu/gpt_egitim.tr) duman testi: depodaki sentetik veri
// SimpleStories klasörü düzenine (tokenizer.json, train.npy, valid.npy) konur, çok küçük ayarla
// CPU'da koşturulur; koşu klasörüne beş çıktı yazılır, aynı komut kaldığı yerden sürer.
func TestGPTEgitimBetigiDumanTesti(t *testing.T) {
	veri := t.TempDir()
	for hedef, kaynak := range map[string]string{"tokenizer.json": "tests/data/parcalayici.json",
		"train.npy": "tests/data/text/metin_000.npy", "valid.npy": "tests/data/text/metin_001.npy"} {
		b, err := os.ReadFile(reporoot.Path(kaynak))
		if err != nil {
			t.Fatal(err)
		}
		dosyaYaz(t, veri, hedef, string(b))
	}
	kosu := filepath.Join(t.TempDir(), "kosu")
	args := []string{reporoot.Path("tools/gpu/gpt_egitim.tr"), "--veri", veri, "--koşu", kosu, "--cihaz", "cpu",
		"--boyut", "16", "--katman", "1", "--baş", "2", "--bağlam", "16", "--toplu", "4", "--birikim", "2",
		"--adım", "12", "--ısınma", "2", "--değerlendir_her", "6", "--değerlendir_toplu", "2", "--örnek_her", "6",
		"--örnek_sayısı", "2", "--örnek_uzunluğu", "6", "--ölçüm_ısınması", "2", "--günlük_her", "3"}
	s := tr(t, t.TempDir(), args...)
	bekle(t, s, 0, []string{"ms/adım", "bitti: en iyi doğrulama kaybı", "Once upon a time"}, nil)
	if strings.Contains(s.hata, "Uyar") {
		t.Errorf("betik uyarı verdi:\n%s", s.hata)
	}
	for _, ad := range []string{"config.json", "metrics.jsonl", "samples.json", "speed.json", "model.trt"} {
		if _, err := os.Stat(filepath.Join(kosu, ad)); err != nil {
			t.Errorf("%s yazılmadı: %v", ad, err)
		}
	}
	var hiz map[string]any
	if b, err := os.ReadFile(filepath.Join(kosu, "speed.json")); err != nil || json.Unmarshal(b, &hiz) != nil || hiz["ms_adım"] == nil {
		t.Errorf("speed.json okunamadı: %v %v", err, hiz)
	}
	metrik, _ := os.ReadFile(filepath.Join(kosu, "metrics.jsonl"))
	if !strings.Contains(string(metrik), `"doğrulama_kaybı"`) || !strings.Contains(string(metrik), `"oran_çarpanı"`) {
		t.Errorf("metrics.jsonl'de doğrulama kaybı ya da oran çarpanı yok:\n%s", metrik)
	}
	// Aynı komut: eğitim bitmiş, kontrol noktasından yüklenir.
	bekle(t, tr(t, t.TempDir(), args...), 0, []string{"bitti"}, []string{"Kaldığı yerden devam: adım 12"})
}

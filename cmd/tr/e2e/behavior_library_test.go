package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural 11.1 (güvenlik bulgusu O2): başka klasörden çalıştırılan programın göreli 'aktar'ı
// çalışma klasöründeki aynı adlı dosyayı değil, programın yanındakini yükler.
func TestImportDoesNotLoadWorkingDirFile(t *testing.T) {
	project := t.TempDir()
	testkit.WriteFile(t, project, "ana.tr", "aktar \"yardimci\"\nyazdır(kaynak)\n")
	testkit.WriteFile(t, project, "yardimci.tr", "değişken kaynak = \"programın yanı\"\n")
	runDir := t.TempDir()
	testkit.WriteFile(t, runDir, "yardimci.tr", "değişken kaynak = \"SAHTE\"\n")
	s := tr(t, runDir, filepath.Join(project, "ana.tr"))
	testkit.Wait(t, s, 0, []string{"programın yanı"}, nil)
	if strings.Contains(s.Stdout, "SAHTE") {
		t.Error("çalışma klasöründeki dosya programın modülünün yerine geçti")
	}
	// tr kontrol ve tr derle aynı kuralla çözer.
	testkit.Wait(t, tr(t, runDir, "kontrol", filepath.Join(project, "ana.tr")), 0, nil, nil)
}

// Kural 11.6: tr/ önekli modül gömülü kitaplıktandır; tr kontrol olmayanı bildirir, tr derle
// kitaplığı pakete gömmez (uygulama yorumlayıcının kopyasıdır) ve uygulama başka klasörde,
// diskte tr/katman.tr varken de kitaplıktakini yükler.
func TestLibraryModuleCheckAndBuild(t *testing.T) {
	project := t.TempDir()
	testkit.WriteFile(t, project, "ana.tr", strings.Join([]string{
		`aktar "tr/katman" olarak k`,
		`aktar "yerel"`,
		`değişken d = k.Doğrusal(3, 2, tohum: 1)`,
		`yazdır("şekil {d.ileri(birler([1, 3])).şekil} {YEREL}")`,
	}, "\n")+"\n")
	testkit.WriteFile(t, project, "yerel.tr", "değişken YEREL = \"yerel modül\"\n")
	testkit.WriteFile(t, project, "tr/katman.tr", "değişken Doğrusal = yok\n")
	expected := "şekil [1, 2] yerel modül"
	testkit.Wait(t, tr(t, project, "ana.tr"), 0, []string{expected}, nil)

	exit := filepath.Join(t.TempDir(), "uygulama.exe")
	s := tr(t, project, "derle", "ana.tr", exit)
	testkit.Wait(t, s, 0, []string{"Başarıyla derlendi", "Gömülen modüller (1)", "yerel.tr"}, nil)
	if strings.Contains(s.Stdout, "katman") {
		t.Errorf("kitaplık modülü pakete gömülmemeli:\n%s", s.Stdout)
	}
	other := t.TempDir()
	testkit.WriteFile(t, other, "tr/katman.tr", "değişken Doğrusal = yok\n")
	testkit.Wait(t, testkit.RunBinary(t, exit, other, "", nil), 0, []string{expected}, nil)

	testkit.WriteFile(t, project, "hatali.tr", "aktar \"tr/olmayan\"\n")
	testkit.Wait(t, tr(t, project, "kontrol", "hatali.tr"), 1, nil, []string{`"tr/olmayan" dilin kitaplığında yok; kitaplıktaki modüller: tr/egitim, tr/hazir, tr/iyilestirici, tr/katman`})
	testkit.Wait(t, tr(t, project, "derle", "hatali.tr", exit), 1, nil, []string{`"tr/olmayan" dilin kitaplığında yok`})
}

// Colab eğitim betiğinin (tools/gpu/gpt_training.tr) duman testi: depodaki sentetik veri
// SimpleStories klasörü düzenine (tokenizer.json, train.npy, valid.npy) konur, çok küçük ayarla
// CPU'da koşturulur; koşu klasörüne beş çıktı yazılır, her istem için örnek_sayısı metin üretilir,
// aynı komut kaldığı yerden sürer.
func TestGPTTrainingScriptSmoke(t *testing.T) {
	data := t.TempDir()
	for target, source := range map[string]string{"tokenizer.json": "tests/data/tokenizer.json",
		"train.npy": "tests/data/text/text_000.npy", "valid.npy": "tests/data/text/text_001.npy"} {
		b, err := os.ReadFile(reporoot.Path(source))
		if err != nil {
			t.Fatal(err)
		}
		testkit.WriteFile(t, data, target, string(b))
	}
	run := filepath.Join(t.TempDir(), "kosu")
	args := []string{reporoot.Path("tools/gpu/gpt_training.tr"), "--veri", data, "--koşu", run, "--cihaz", "cpu",
		"--boyut", "16", "--katman", "1", "--bakış_sayısı", "2", "--bağlam", "16", "--toplu", "4", "--birikim", "2",
		"--adım", "12", "--ısınma", "2", "--değerlendir_her", "6", "--değerlendir_toplu", "2", "--örnek_her", "6",
		"--örnek_sayısı", "2", "--örnek_uzunluğu", "6", "--ölçüm_ısınması", "2", "--günlük_her", "3",
		"--istemler", "Once upon a time|The dog"}
	s := tr(t, t.TempDir(), args...)
	testkit.Wait(t, s, 0, []string{"ms/adım", "bitti: en iyi doğrulama kaybı", "Once upon a time", "The dog"}, nil)
	if strings.Contains(s.Stderr, "Uyar") {
		t.Errorf("betik uyarı verdi:\n%s", s.Stderr)
	}
	for _, ad := range []string{"config.json", "metrics.jsonl", "samples.json", "speed.json", "model.trt"} {
		if _, err := os.Stat(filepath.Join(run, ad)); err != nil {
			t.Errorf("%s yazılmadı: %v", ad, err)
		}
	}
	var speed map[string]any
	if b, err := os.ReadFile(filepath.Join(run, "speed.json")); err != nil || json.Unmarshal(b, &speed) != nil || speed["ms_adım"] == nil {
		t.Errorf("speed.json okunamadı: %v %v", err, speed)
	}
	metric, _ := os.ReadFile(filepath.Join(run, "metrics.jsonl"))
	if !strings.Contains(string(metric), `"doğrulama_kaybı"`) || !strings.Contains(string(metric), `"oran_çarpanı"`) {
		t.Errorf("metrics.jsonl'de doğrulama kaybı ya da oran çarpanı yok:\n%s", metric)
	}
	var samples []struct{ Metinler []string `json:"metinler"` }
	if b, err := os.ReadFile(filepath.Join(run, "samples.json")); err != nil || json.Unmarshal(b, &samples) != nil || len(samples) == 0 {
		t.Fatalf("samples.json okunamadı: %v", err)
	}
	if m := samples[len(samples)-1].Metinler; len(m) != 4 || !strings.HasPrefix(m[0], "Once upon a time") || !strings.HasPrefix(m[2], "The dog") {
		t.Errorf("son örnekler: iki istemin ikişer metni beklenirdi: %q", m)
	}
	// Aynı komut: eğitim bitmiş, kontrol noktasından yüklenir.
	testkit.Wait(t, tr(t, t.TempDir(), args...), 0, []string{"bitti"}, []string{"Kaldığı yerden devam: adım 12"})
}

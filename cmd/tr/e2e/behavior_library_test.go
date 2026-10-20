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
func TestImportDoesNotLoadWorkingDirFile(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, "ana.tr", "aktar \"yardimci\"\nyazdır(kaynak)\n")
	writeFile(t, project, "yardimci.tr", "değişken kaynak = \"programın yanı\"\n")
	runDir := t.TempDir()
	writeFile(t, runDir, "yardimci.tr", "değişken kaynak = \"SAHTE\"\n")
	s := tr(t, runDir, filepath.Join(project, "ana.tr"))
	wait(t, s, 0, []string{"programın yanı"}, nil)
	if strings.Contains(s.stdout, "SAHTE") {
		t.Error("çalışma klasöründeki dosya programın modülünün yerine geçti")
	}
	// tr kontrol ve tr derle aynı kuralla çözer.
	wait(t, tr(t, runDir, "kontrol", filepath.Join(project, "ana.tr")), 0, nil, nil)
}

// Kural 11.6: tr/ önekli modül gömülü kitaplıktandır; tr kontrol olmayanı bildirir, tr derle
// kitaplığı pakete gömmez (uygulama yorumlayıcının kopyasıdır) ve uygulama başka klasörde,
// diskte tr/katman.tr varken de kitaplıktakini yükler.
func TestLibraryModuleCheckAndBuild(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, "ana.tr", strings.Join([]string{
		`aktar "tr/katman" olarak k`,
		`aktar "yerel"`,
		`değişken d = k.Doğrusal(3, 2, tohum: 1)`,
		`yazdır("şekil {d.ileri(birler([1, 3])).şekil} {YEREL}")`,
	}, "\n")+"\n")
	writeFile(t, project, "yerel.tr", "değişken YEREL = \"yerel modül\"\n")
	writeFile(t, project, "tr/katman.tr", "değişken Doğrusal = yok\n")
	expected := "şekil [1, 2] yerel modül"
	wait(t, tr(t, project, "ana.tr"), 0, []string{expected}, nil)

	exit := filepath.Join(t.TempDir(), "uygulama.exe")
	s := tr(t, project, "derle", "ana.tr", exit)
	wait(t, s, 0, []string{"Başarıyla derlendi", "Gömülen modüller (1)", "yerel.tr"}, nil)
	if strings.Contains(s.stdout, "katman") {
		t.Errorf("kitaplık modülü pakete gömülmemeli:\n%s", s.stdout)
	}
	other := t.TempDir()
	writeFile(t, other, "tr/katman.tr", "değişken Doğrusal = yok\n")
	wait(t, run(t, exit, other, "", nil), 0, []string{expected}, nil)

	writeFile(t, project, "hatali.tr", "aktar \"tr/olmayan\"\n")
	wait(t, tr(t, project, "kontrol", "hatali.tr"), 1, nil, []string{`"tr/olmayan" dilin kitaplığında yok; kitaplıktaki modüller: tr/iyilestirici, tr/katman`})
	wait(t, tr(t, project, "derle", "hatali.tr", exit), 1, nil, []string{`"tr/olmayan" dilin kitaplığında yok`})
}

// Colab eğitim betiğinin (tools/gpu/gpt_egitim.tr) duman testi: depodaki sentetik veri
// SimpleStories klasörü düzenine (tokenizer.json, train.npy, valid.npy) konur, çok küçük ayarla
// CPU'da koşturulur; koşu klasörüne beş çıktı yazılır, aynı komut kaldığı yerden sürer.
func TestGPTTrainingScriptSmoke(t *testing.T) {
	data := t.TempDir()
	for target, source := range map[string]string{"tokenizer.json": "tests/data/parcalayici.json",
		"train.npy": "tests/data/text/metin_000.npy", "valid.npy": "tests/data/text/metin_001.npy"} {
		b, err := os.ReadFile(reporoot.Path(source))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, data, target, string(b))
	}
	run := filepath.Join(t.TempDir(), "kosu")
	args := []string{reporoot.Path("tools/gpu/gpt_egitim.tr"), "--veri", data, "--koşu", run, "--cihaz", "cpu",
		"--boyut", "16", "--katman", "1", "--baş", "2", "--bağlam", "16", "--toplu", "4", "--birikim", "2",
		"--adım", "12", "--ısınma", "2", "--değerlendir_her", "6", "--değerlendir_toplu", "2", "--örnek_her", "6",
		"--örnek_sayısı", "2", "--örnek_uzunluğu", "6", "--ölçüm_ısınması", "2", "--günlük_her", "3"}
	s := tr(t, t.TempDir(), args...)
	wait(t, s, 0, []string{"ms/adım", "bitti: en iyi doğrulama kaybı", "Once upon a time"}, nil)
	if strings.Contains(s.stderr, "Uyar") {
		t.Errorf("betik uyarı verdi:\n%s", s.stderr)
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
	// Aynı komut: eğitim bitmiş, kontrol noktasından yüklenir.
	wait(t, tr(t, t.TempDir(), args...), 0, []string{"bitti"}, []string{"Kaldığı yerden devam: adım 12"})
}

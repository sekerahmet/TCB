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
	writeTR(t, project, "ana.tr", "TR_import \"yardimci\"\nTR_print(kaynak)\n")
	writeTR(t, project, "yardimci.tr", "TR_var kaynak = \"programın yanı\"\n")
	runDir := t.TempDir()
	writeTR(t, runDir, "yardimci.tr", "TR_var kaynak = \"SAHTE\"\n")
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
// diskte tr/layers.tr varken de kitaplıktakini yükler.
func TestLibraryModuleCheckAndBuild(t *testing.T) {
	project := t.TempDir()
	writeTR(t, project, "ana.tr", strings.Join([]string{
		`TR_import "tr/layers" TR_as k`,
		`TR_import "yerel"`,
		`TR_var d = k.TR_Linear(3, 2, TR_seed: 1)`,
		`TR_print("şekil \(d.TR_forward(TR_ones([1, 3])).TR_shape) \(YEREL)")`,
	}, "\n")+"\n")
	writeTR(t, project, "yerel.tr", "TR_var YEREL = \"yerel modül\"\n")
	writeTR(t, project, "tr/layers.tr", "TR_var TR_Linear = TR_none\n")
	expected := "şekil [1, 2] yerel modül"
	testkit.Wait(t, tr(t, project, "ana.tr"), 0, []string{expected}, nil)

	exit := filepath.Join(t.TempDir(), "uygulama.exe")
	s := tr(t, project, "derle", "ana.tr", exit)
	testkit.Wait(t, s, 0, []string{"Başarıyla derlendi", "Gömülen modüller (1)", "yerel.tr"}, nil)
	if strings.Contains(s.Stdout, "layers") {
		t.Errorf("kitaplık modülü pakete gömülmemeli:\n%s", s.Stdout)
	}
	other := t.TempDir()
	writeTR(t, other, "tr/layers.tr", "TR_var TR_Linear = TR_none\n")
	testkit.Wait(t, testkit.RunBinary(t, exit, other, "", nil), 0, []string{expected}, nil)

	writeTR(t, project, "hatali.tr", "TR_import \"tr/olmayan\"\n")
	testkit.Wait(t, tr(t, project, "kontrol", "hatali.tr"), 1, nil, []string{`"tr/olmayan" dilin kitaplığında yok; kitaplıktaki modüller: tr/layers, tr/optimizers, tr/pretrained, tr/training`})
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
	args := []string{reporoot.Path("tools/gpu/gpt_training.tr"), "--veri", data, "--koşu", run, "--device", "cpu",
		"--embed_dim", "16", "--katman", "1", "--num_heads", "2", "--bağlam", "16", "--toplu", "4", "--birikim", "2",
		"--step", "12", "--ısınma", "2", "--değerlendir_her", "6", "--değerlendir_toplu", "2", "--örnek_her", "6",
		"--örnek_sayısı", "2", "--örnek_uzunluğu", "6", "--ölçüm_ısınması", "2", "--günlük_her", "3",
		"--istemler", "Once upon a time|The dog",
		// PyTorch tarifini eşleyen seçenekler (tools/gpu/COLAB.md §9): hepsi kurulup bir adım çalışmalı.
		"--dağarcık", "1024", "--qk_normu", "--birleşik_kv", "--birleşik_kapı",
		"--ileri_sapmalı", "--eps", "0.000001", "--ara_boyut", "48", "--muon_ölçeği", "rms",
		"--adamw_çürümesi", "0.1", "--weight_decay", "0.1", "--son_değerlendir_toplu", "1000"}
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
	// Eğitim sağlığı ölçüleri (PyTorch kıyas koşularının metrics.jsonl'i gibi): gerçek oranlar, kırpma öncesi
	// gradyan normu, aralığın ms/adım ve parça/sn'si (ilk günlük kaydında aralık yoktur).
	for _, key := range []string{`"muon_oranı"`, `"adamw_oranı"`, `"gradyan_normu"`, `"ms_adım"`, `"parça_sn"`} {
		if !strings.Contains(string(metric), key) {
			t.Errorf("metrics.jsonl'de %s yok:\n%s", key, metric)
		}
	}
	if speed["dış_sn"] == nil {
		t.Errorf("speed.json'da dış_sn yok: %v", speed)
	}
	var samples []struct {
		Metinler []string `json:"metinler"`
	}
	if b, err := os.ReadFile(filepath.Join(run, "samples.json")); err != nil || json.Unmarshal(b, &samples) != nil || len(samples) == 0 {
		t.Fatalf("samples.json okunamadı: %v", err)
	}
	if m := samples[len(samples)-1].Metinler; len(m) != 4 || !strings.HasPrefix(m[0], "Once upon a time") || !strings.HasPrefix(m[2], "The dog") {
		t.Errorf("son örnekler: iki istemin ikişer metni beklenirdi: %q", m)
	}
	// Aynı komut: eğitim bitmiş, kontrol noktasından yüklenir.
	testkit.Wait(t, tr(t, t.TempDir(), args...), 0, []string{"bitti"}, []string{"Kaldığı yerden devam: adım 12"})

	// Prova (tr/training Koşu): aynı komut iki adım, geçici klasörde; verilen koşu klasörüne dokunmaz.
	unused := filepath.Join(t.TempDir(), "kosu")
	trial := append(append([]string{}, args...), "--dry_run")
	trial[4] = unused
	testkit.Wait(t, tr(t, t.TempDir(), trial...), 0, []string{"Prova geçti: 2 adım"}, nil)
	if _, err := os.Stat(unused); err == nil {
		t.Errorf("prova koşu klasörünü açtı: %s", unused)
	}

	// SimpleStories sınavı (tools/gpu/ss_exam.tr) eğitilen modeli koşu klasöründen kurar ve results.json yazar.
	exam := tr(t, t.TempDir(), reporoot.Path("tools/gpu/ss_exam.tr"), "--koşu", run, "--veri", data, "--device", "cpu",
		"--hassasiyet", "float32", "--toplu", "4", "--sonda_uzunluğu", "6", "--örnek_uzunluğu", "6")
	testkit.Wait(t, exam, 0, []string{"sınav: kayıp", "bant 0-16", "sonda: farklı_4", "örnek: Once upon a time"}, nil)
	var result struct {
		Params int `json:"parametre_sayısı"`
		Steps  int `json:"step"` // kodlu çıplak anahtar ({adım: …}) dilden bağımsız adıyla (17.3)
		Final  struct {
			Loss     float64          `json:"loss"`
			BPB      float64          `json:"bpb"`
			Accuracy float64          `json:"doğruluk"`
			Tokens   int              `json:"parça"`
			Probes   []map[string]any `json:"sondalar"`
		} `json:"last"`
		Speed   map[string]any   `json:"hız"`
		Samples []map[string]any `json:"örnekler"`
	}
	if b, err := os.ReadFile(filepath.Join(run, "results.json")); err != nil || json.Unmarshal(b, &result) != nil {
		t.Fatalf("results.json okunamadı: %v", err)
	}
	f := result.Final
	if result.Params == 0 || result.Steps != 12 || result.Speed == nil || f.Loss <= 0 || f.BPB <= 0 || f.Accuracy < 0 ||
		f.Tokens == 0 || len(f.Probes) != 2 || len(result.Samples) != 4 {
		t.Errorf("results.json alanları: %+v", result)
	}
}

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// watchTraining 60 adımlık kısa bir eğitimdir (tr izle / tr profil alt süreç testleri).
const watchTraining = `TR_var w = TR_Tensor(TR_randn([128, 128], TR_seed: 1), TR_requires_grad: TR_true)
TR_var x = TR_randn([64, 128], TR_seed: 2)
TR_var op = TR_AdamW([w], TR_lr: 0.01)
TR_var ç = TR_Progress(60)
TR_for i TR_in 0..60 {
    op.TR_zero_grad()
    TR_var kayıp = TR_sum(TR_relu(x @ w))
    TR_backward(kayıp)
    op.TR_step()
    ç.TR_update({"loss": kayıp})
}
TR_print("eğitim bitti")
`

// sampleCount raporun "Örnekleme: ... aralıkla N örnek" satırındaki örnek sayısı
// (binlik ayırıcı boşluk olabilir).
var sampleCount = regexp.MustCompile(`aralıkla ([0-9 \x{00a0}]+) örnek`)

// checkLineExample programın kendi satırının (satir, ör. "egit.tr:") sıcak satırlarda
// görünmesini, rapor yeterince örnek topladıysa ister. Test programı birkaç on milisaniye
// sürer: yüklü bir makinede (CI, Windows) örnekleyici o dosyanın satırlarına hiç denk
// gelmeyebilir; o zaman yalnızca örnek sayısının raporda bildirildiği denetlenir.
func checkLineExample(t *testing.T, report, line string) {
	t.Helper()
	m := sampleCount.FindStringSubmatch(report)
	if m == nil {
		t.Fatalf("raporda örnekleme satırı yok:\n%s", report)
	}
	n, _ := strconv.Atoi(strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, m[1]))
	if n >= 20 && !strings.Contains(report, line) {
		t.Errorf("%d örnekte %q satırı yok:\n%s", n, line, report)
	}
}

// readReport speed.json/profile.json dosyasını çözer.
func readReport(t *testing.T, filePath string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("rapor dosyası okunamadı: %v", err)
	}
	var d map[string]any
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("rapor geçerli JSON değil: %v\n%s", err, data)
	}
	return d
}

// processCall rapordaki bir işlemin (kod, yön) çağrı sayısıdır.
func processCall(d map[string]any, code, direction string) float64 {
	for _, x := range d["tensor"].(map[string]any)["ops"].([]any) {
		b := x.(map[string]any)
		if b["code"] == code && b["direction"] == direction {
			return b["calls"].(float64)
		}
	}
	return -1
}

// tr profil: metin rapor standart çıktıya, JSON rapor koşu klasörüne (speed.json).
func TestProfileCommand(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "egit.tr", watchTraining)
	run := filepath.Join(dir, "runs", "deneme1") + string(os.PathSeparator)
	s := tr(t, dir, "profil", "--json", run, "egit.tr")
	testkit.Wait(t, s, 0, []string{"eğitim bitti", "⏱ Profil: egit.tr", "Adımlar: 60 adım", "Tensör işlemleri",
		"matris çarpımı (@)", "geri · matris çarpımı (@)", "AdamW.adım", "En çok zaman alan satırlar"},
		[]string{"speed.json"})
	checkLineExample(t, s.Stdout+s.Stderr, "egit.tr:")
	d := readReport(t, filepath.Join(run, "speed.json"))
	if d["format"] != "tr-profile" || d["kind"] != "report" || d["exit_code"] != float64(0) {
		t.Errorf("rapor kimliği: %v %v %v", d["format"], d["kind"], d["exit_code"])
	}
	for _, x := range []struct {
		code, direction string
		n               float64
	}{{"matmul", "forward", 60}, {"matmul", "backward", 60}, {"adamw", "forward", 60}} {
		if n := processCall(d, x.code, x.direction); n != x.n {
			t.Errorf("%s (%s) %v çağrı, beklenen %v", x.code, x.direction, n, x.n)
		}
	}
	if d["sampling"] == nil {
		t.Error("tr profil satır örneklemesiyle çalışmalı")
	}
}

// tr profil program hata verse de raporu yazar; çıkış kodu programınkidir.
func TestProfileCommandReportsOnError(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "hata.tr", "TR_var a = TR_ones([2, 3])\nTR_print(a @ a)\n")
	s := tr(t, dir, "profil", "--json", "rapor.json", "hata.tr")
	testkit.Wait(t, s, 1, []string{"⏱ Profil: hata.tr", "çıkış kodu 1"}, []string{"Çalışma Zamanı Hatası"})
	if d := readReport(t, filepath.Join(dir, "rapor.json")); d["exit_code"] != float64(1) {
		t.Errorf("rapordaki çıkış kodu %v", d["exit_code"])
	}
}

// tr izle terminal değilken: program çıktısı olduğu gibi akar, son rapor standart hataya.
func TestWatchCommandNotTerminal(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "egit.tr", watchTraining)
	s := tr(t, dir, "izle", "egit.tr")
	testkit.Wait(t, s, 0, []string{"eğitim bitti"}, []string{"adım 60/60", "⏱ Profil: egit.tr", "Adımlar: 60 adım",
		"geri · matris çarpımı (@)", "En çok zaman alan satırlar"})
	checkLineExample(t, s.Stderr, "egit.tr:")
	if strings.Contains(s.Stderr, "\x1b[") {
		t.Error("terminal değilken denetim dizisi yazılmamalı")
	}
}

// Program İlerleme(...) kullanmıyorsa rapor nasıl kullanılacağını söyler.
func TestWatchNoStepInfo(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "dongu.tr", "TR_var s = 0\nTR_for i TR_in 0..1000 {\n    s += i\n}\nTR_print(s)\n")
	s := tr(t, dir, "izle", "dongu.tr")
	testkit.Wait(t, s, 0, []string{"499500"}, []string{"adım bilgisi yok: eğitim döngüsünde İlerleme(...) ve .güncelle(...) kullanın"})
}

// tr izle .tr olmayan bir adla (arka plan işini izleme henüz yok) açık hata verir.
func TestWatchJobNameNotYet(t *testing.T) {
	s := tr(t, t.TempDir(), "izle", "deneme1")
	testkit.Wait(t, s, 2, nil, []string{"bir .tr dosyası değil", "tr watch <dosya.tr>"})
	s = tr(t, t.TempDir(), "profil", "--bilinmeyen", "x.tr")
	testkit.Wait(t, s, 2, nil, []string{"bilinmeyen seçenek"})
}

// Arka plan işi (TR_IS_KLASORU) olarak çalışan program iş klasörüne profile.json yazar.
func TestJobProfileFile(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "egit.tr", watchTraining)
	is := filepath.Join(dir, ".tr", "işler", "deneme1")
	if err := os.MkdirAll(is, 0o755); err != nil {
		t.Fatal(err)
	}
	s := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_IS_KLASORU=" + is}, "egit.tr")
	testkit.Wait(t, s, 0, []string{"eğitim bitti"}, nil)
	d := readReport(t, filepath.Join(is, "profile.json"))
	if d["done"] != true || d["sampling"] != nil {
		t.Errorf("profile.json: bitti %v, örnekleme %v (iş profilinde örnekleme kapalı)", d["done"], d["sampling"])
	}
	if n := processCall(d, "matmul", "forward"); n != 60 {
		t.Errorf("iş profilinde matris çarpımı %v çağrı, beklenen 60", n)
	}
}

// gpuWatchTraining GPU'da (TR_GPU_SIM=1 ile benzetimde) 8 adımlık küçük bir eğitimdir.
const gpuWatchTraining = `TR_default_device("gpu")
TR_var w = TR_Tensor(TR_randn([16, 16], TR_seed: 1), TR_requires_grad: TR_true)
TR_var x = TR_randn([8, 16], TR_seed: 2)
TR_var op = TR_AdamW([w], TR_lr: 0.01)
TR_var ç = TR_Progress(8)
TR_for i TR_in 0..8 {
    op.TR_zero_grad()
    TR_var kayıp = TR_sum(TR_relu(x @ w))
    TR_backward(kayıp)
    op.TR_step()
    ç.TR_update({"loss": kayıp})
}
TR_print("gpu eğitimi bitti")
`

// tr profil ve tr izle GPU'da: çekirdek süreleri ve GPU belleği rapora girer; GPU arka
// ucunun kendi raporu bir kez yazılır (ölçümü profil açtıysa raporun içinde, TR_GPU_PROFIL
// ile açıksa arka ucun kendisince).
func TestProfileAndWatchGPU(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "gpu.tr", gpuWatchTraining)
	for _, d := range []struct {
		ad, gpuProfile string
		args           []string
	}{
		{"profil", "", []string{"profil", "--json", "speed.json", "gpu.tr"}},
		{"izle", "", []string{"izle", "gpu.tr"}},
		{"profil_TR_GPU_PROFIL", "1", []string{"profil", "gpu.tr"}},
	} {
		t.Run(d.ad, func(t *testing.T) {
			s := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_GPU_SIM=1", "TR_GPU_PROFIL=" + d.gpuProfile}, d.args...)
			if s.Code != 0 {
				t.Fatalf("çıkış kodu %d\nçıktı:\n%s\nhata:\n%s", s.Code, s.Stdout, s.Stderr)
			}
			all := s.Stdout + s.Stderr
			for _, p := range []string{"gpu eğitimi bitti", "GPU kernelleri: cihazda", "carpim", "GPU canlı"} {
				if !strings.Contains(all, p) {
					t.Errorf("çıktıda %q yok:\n%s", p, all)
				}
			}
			if n := strings.Count(all, "== GPU profili"); n != 1 {
				t.Errorf("GPU arka ucunun raporu %d kez yazıldı, beklenen 1:\n%s", n, all)
			}
		})
	}
	d := readReport(t, filepath.Join(dir, "speed.json"))
	g := d["gpu"].(map[string]any)
	if g["used"] != true || g["measured"] != true || len(g["families"].([]any)) == 0 || len(g["memory"].([]any)) == 0 {
		t.Errorf("speed.json GPU bölümü eksik: %v", g)
	}
}

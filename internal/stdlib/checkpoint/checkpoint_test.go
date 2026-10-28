package checkpoint_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
	"github.com/sekerahmet/TCB/internal/testkit"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// run kodu yeni bir VM'de (yeni bir program gibi) çalıştırır; standart çıktıyı ve
// hatayı birlikte verir.
func run(t *testing.T, code string) (string, error) {
	t.Helper()
	m := interp.New()
	w := &testkit.TeeWriter{}
	m.Out, m.Err = w, w
	_, err := runVM(t, m, code)
	return w.String(), err
}

// runVM kodu verilen VM'de bir program olarak çalıştırır (program dosyası depo
// kökündedir) ve motorların kaynaklarını kapatır.
func runVM(t *testing.T, m *interp.VM, code string) (interp.Value, error) {
	t.Helper()
	m.SourceFile = filepath.Join(reporoot.Root(), "egit.tr")
	prog := trrun.Parse(t, code)
	v, err := m.Run(prog)
	m.WaitAsync()
	stdlib.Close()
	return v, err
}

func trPath(s string) string { return filepath.ToSlash(s) }

// trainingProgram küçük bir modeli kontrol noktasıyla dur adıma kadar eğitir (toplam adımlık
// bir koşunun parçası). İki model (AdamW ve momentumlu SGD), örnekleyici, sıralı okuyucu,
// günlük ve düz değerler noktaya girer; sonunda parametreler ve iyileştirici durumları
// karşılaştırma için kaydedilir.
func trainingProgram(runDir string, stop, total int, extraOption string) string {
	return fmt.Sprintf(`
değişken KOŞU = %q
değişken V = 4096
değişken D = 16
değişken sayaç = 0
fonksiyon parametre(şekil, ölçek) {
    sayaç += 1
    sonuç tensör(türevsiz(rastgele_normal(şekil, tohum: sayaç) * ölçek), türev_izle: doğru)
}
değişken model = {E: parametre([V, D], 0.1), W: parametre([D, V], 0.1)}
değişken ikinci = [parametre([D], 1.0)]
değişken op = AdamW(parametreler(model), öğrenme_oranı: 0.01)
değişken op2 = SGD(ikinci, öğrenme_oranı: 0.1, momentum: 0.9)
değişken küme = veri_kümesi(%q)
değişken ö = küme.örnekleyici(toplu: 4, bağlam: 8, tohum: 5)
değişken s = küme.sıralı(toplu: 2, bağlam: 8)
değişken g = günlük(yol(KOŞU, "metrics.jsonl"))
değişken izlenen = {en_iyi: 99.0, geçmiş: [], son_sıralı: yok}
değişken kn = kontrol_noktası(KOŞU, {model: model, op: op, ikinci: ikinci, op2: op2, ö: ö, s: s, g: g, izlenen: izlenen}%s)
değişken ç = ilerleme(%d, günlük: g)
döngü adım içinde kn.kaldığı_adım..%d {
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    op2.sıfırla()
    değişken kayıp = çapraz_entropi(gömme(model.E, giriş) @ model.W, hedef) + toplam(ikinci[0] * ikinci[0]) * 0.01
    geri_yayılım(kayıp)
    op.adım()
    op2.adım()
    değişken d = s.sonraki()
    eğer d != yok { izlenen.son_sıralı = d[0].değer[0][0] }
    eğer kayıp.değer < izlenen.en_iyi { izlenen.en_iyi = kayıp.değer }
    izlenen.geçmiş.ekle(adım)
    ç.güncelle({kayıp: kayıp}, adım: adım + 1)
    kn.güncelle(adım + 1)
}
tensör_kaydet(yol(KOŞU, "son.trt"), parametreler([model, ikinci], adlarıyla: doğru))
tensör_kaydet(yol(KOŞU, "op.trt"), op.durum)
tensör_kaydet(yol(KOŞU, "op2.trt"), op2.durum)
yazdır("izlenen:", izlenen)
yazdır("ö:", ö.durum)
`, trPath(runDir), trPath(filepath.Join(reporoot.Root(), "tests", "data", "text")), extraOption, total, stop)
}

// sameResults iki koşunun parametre, iyileştirici dosyalarının, geçerli kayıp dizisinin ve
// programın son çıktısının (düz değerler, okuyucu durumu) bit düzeyinde aynı olduğunu denetler.
func sameResults(t *testing.T, a, b, outputA, outputB string) {
	t.Helper()
	testkit.AssertSameRunFiles(t, a, b, []string{"son.trt", "op.trt", "op2.trt"}, false)
	last := func(c string) string { return c[strings.LastIndex(c, "izlenen:"):] }
	if last(outputA) != last(outputB) {
		t.Errorf("son durum farklı:\n%s\n%s", last(outputA), last(outputB))
	}
}

// Kesintisiz eğitim ile kesilip (aralıklı kayıttan, kaydedilmemiş adımlar yeniden
// hesaplanarak) sürdürülen eğitim aynı son adımda bit düzeyinde aynı parametreleri,
// iyileştirici durumlarını, kayıp dizisini, okuyucu konumlarını ve düz değerleri verir.
func TestResumedSameAsUninterrupted(t *testing.T) {
	var now time.Time
	t.Cleanup(checkpoint.FakeClock(func() time.Time { now = now.Add(time.Minute); return now }))
	// Her güncelle'de saat 1 dk ilerler: her_dakika: 3 ile yaklaşık üç adımda bir kayıt.
	a := t.TempDir()
	outputA, err := run(t, trainingProgram(a, 24, 24, ", her_dakika: 3"))
	if err != nil {
		t.Fatalf("%v\n%s", err, outputA)
	}
	b := t.TempDir()
	if c, err := run(t, trainingProgram(b, 13, 24, ", her_dakika: 3")); err != nil {
		t.Fatalf("%v\n%s", err, c)
	}
	outputB, err := run(t, trainingProgram(b, 24, 24, ", her_dakika: 3"))
	if err != nil {
		t.Fatalf("%v\n%s", err, outputB)
	}
	if !strings.Contains(outputB, "Kaldığı yerden devam: adım 1") || strings.Contains(outputB, "adım 13 (") {
		t.Fatalf("13. adımdan önceki son aralıklı noktadan sürmeliydi:\n%s", outputB)
	}
	sameResults(t, a, b, outputA, outputB)
	// tut: 2 → en yeni iki nokta kalır.
	inputs, _ := reporoot.ReadDir(filepath.Join(b, "checkpoints"))
	var ckpts []string
	for _, g := range inputs {
		ckpts = append(ckpts, g.Name())
	}
	if len(ckpts) != 3 || ckpts[0] != "latest.json" {
		t.Errorf("iki nokta ve latest.json bekleniyordu: %v", ckpts)
	}
}

// Ağ sürücüsü (FUSE) benzetimi: yeniden adlandırma başarısız olunca aralıklı kayıt eğitimi
// durdurmaz (uyarı, sonraki aralıkta yeniden deneme), önceki geçerli nokta bozulmaz, geçici
// klasör kalmaz; art arda üçüncü başarısızlıkta açık hata verilir. Yeniden adlandırma
// düzelince kayıt sürer ve sayaç sıfırlanır.
func TestRenameErrorDoesNotStopTraining(t *testing.T) {
	var broken atomicBool
	t.Cleanup(checkpoint.FakeRename(func(old, fresh string) error {
		if broken.load() && filepath.Base(filepath.Dir(fresh)) == "checkpoints" && strings.HasPrefix(filepath.Base(fresh), "step_") {
			return &os.LinkError{Op: "rename", Old: old, New: fresh, Err: errors.New("Input/output error")}
		}
		return os.Rename(old, fresh)
	}))
	runDir := t.TempDir()
	program := func(n int, brokenSteps string) string {
		return fmt.Sprintf(`
TR_var w = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var op = TR_SGD([w], TR_lr: 0.1)
TR_var kn = TR_checkpoint(%q, {w: w, op: op}, TR_every_minutes: 0)
TR_var sonuçlar = []
TR_for adım TR_in kn.TR_resume_step..%d {
    TR_backward(TR_sum(w * w))
    op.TR_step()
    op.TR_zero_grad()
    TR_if [%s].TR_contains(adım + 1) { boz() }
    sonuçlar.TR_append(kn.TR_update(adım + 1))
    düzelt()
}
TR_print("sonuçlar:", sonuçlar)
`, trPath(runDir), n, brokenSteps)
	}
	hooks := func(code string) string {
		return strings.ReplaceAll(strings.ReplaceAll(code, "boz()", "bozuk_yap(TR_true)"), "düzelt()", "bozuk_yap(TR_false)")
	}
	output, err := runHooked(t, hooks(program(6, "3, 4")), &broken)
	if err != nil {
		t.Fatalf("iki başarısızlık eğitimi durdurmamalıydı: %v\n%s", err, output)
	}
	if !strings.Contains(output, "sonuçlar: [doğru, doğru, yanlış, yanlış, doğru, doğru]") ||
		strings.Count(output, "Uyarı: kontrol_noktası: ") != 2 ||
		!strings.Contains(output, "(1/3; eğitim sürüyor") || !strings.Contains(output, "Son geçerli nokta: adım 2") ||
		!strings.Contains(output, "yerel diske kaydedip") {
		t.Fatalf("beklenen uyarılar ve sonuçlar yok:\n%s", output)
	}
	checkLeftovers(t, runDir)
	output, err = runHooked(t, hooks(program(12, "8, 9, 10")), &broken)
	if !messages.Has(err, messages.H7412) || !strings.Contains(err.Error(), "Son geçerli nokta: adım 7") {
		t.Fatalf("üçüncü başarısızlık açık hata vermeliydi: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Kaldığı yerden devam: adım 6") {
		t.Fatalf("önceki koşunun son noktasından sürmeliydi:\n%s", output)
	}
	checkLeftovers(t, runDir)
	// Önceki geçerli nokta sağlam: sürdürme 7'den başlar.
	output, err = runHooked(t, hooks(program(8, "")), &broken)
	if err != nil || !strings.Contains(output, "Kaldığı yerden devam: adım 7") {
		t.Fatalf("son geçerli noktadan sürmeliydi: %v\n%s", err, output)
	}
}

// checkLeftovers noktalar klasöründe geçici ya da kenara alınmış klasör kalmadığını denetler.
func checkLeftovers(t *testing.T, runDir string) {
	t.Helper()
	inputs, _ := reporoot.ReadDir(filepath.Join(runDir, "checkpoints"))
	for _, g := range inputs {
		if strings.Contains(g.Name(), ".tmp-") || strings.Contains(g.Name(), ".old-") {
			t.Errorf("artık klasör kaldı: %s", g.Name())
		}
	}
}

type atomicBool struct {
	mu sync.Mutex
	v  bool
}

func (a *atomicBool) load() bool { a.mu.Lock(); defer a.mu.Unlock(); return a.v }
func (a *atomicBool) store(v bool) {
	a.mu.Lock()
	a.v = v
	a.mu.Unlock()
}

// runHooked calistir gibidir; programa bozuk_yap(Mantık) yerleşiği eklenir (testin
// yeniden adlandırma benzetimini açıp kapatır).
func runHooked(t *testing.T, code string, broken *atomicBool) (string, error) {
	t.Helper()
	m := interp.New()
	w := &testkit.TeeWriter{}
	m.Out, m.Err = w, w
	m.GlobalEnv.Define("bozuk_yap", &interp.BuiltinVal{Name: "bozuk_yap", Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		broken.store(args[0] == interp.True)
		return interp.Null, nil
	}}, true)
	_, err := runVM(t, m, code)
	return w.String(), err
}

// resignCheckpoint noktanın digest.json'unu (sürüm 3) state.json ve tensors.trt dosyalarının şimdiki
// boyları ve SHA-256 özetleriyle yeniden yazar: testler dosyayı bozduktan sonra özet doğru kalır ve
// yükleme özet denetiminden geçip içerik kurallarına ulaşır. Özet elle yazılır (üretimdeki yazıcı
// çağrılmaz: yoksa test kendi kendini doğrulardı).
func resignCheckpoint(t *testing.T, stepDir string, step int) {
	t.Helper()
	trtBytes, err := os.ReadFile(filepath.Join(stepDir, "tensors.trt"))
	if err != nil {
		t.Fatal(err)
	}
	durumBytes, err := os.ReadFile(filepath.Join(stepDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	trtSum := sha256.Sum256(trtBytes)
	durumSum := sha256.Sum256(durumBytes)
	ozet := fmt.Sprintf(`{
  "version": 3,
  "step": %d,
  "files": {
    "state.json": {"bytes": %d, "sha256": %q},
    "tensors.trt": {"bytes": %d, "sha256": %q}
  }
}
`, step, len(durumBytes), hex.EncodeToString(durumSum[:]), len(trtBytes), hex.EncodeToString(trtSum[:]))
	if err := os.WriteFile(filepath.Join(stepDir, "digest.json"), []byte(ozet), 0o644); err != nil {
		t.Fatal(err)
	}
}

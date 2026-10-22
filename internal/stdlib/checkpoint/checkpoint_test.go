package checkpoint_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// teeWriter bir tamponu birden çok çıkışa paylaştırır.
type teeWriter struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (e *teeWriter) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.Write(p)
}

func (e *teeWriter) String() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.String()
}

// run kodu yeni bir VM'de (yeni bir program gibi) çalıştırır; standart çıktıyı ve
// hatayı birlikte verir.
func run(t *testing.T, code string) (string, error) {
	t.Helper()
	m := interp.New()
	w := &teeWriter{}
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

// validMetrics günlüğün geçerli ölçü kayıtlarını (adım → kayıp) okur: bir devam kaydından
// önce gelen ve adımı ondan büyük olan kayıtlar geçersizdir (19, günlük okuyucu kuralı).
func validMetrics(t *testing.T, fpath string) []string {
	t.Helper()
	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	type metric struct {
		step int64
		lost string
	}
	var metrics []metric
	for _, s := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var k map[string]json.RawMessage
		if err := json.Unmarshal([]byte(s), &k); err != nil {
			t.Fatalf("geçersiz satır %q", s)
		}
		var step int64
		_ = json.Unmarshal(k["adım"], &step)
		var kind string
		_ = json.Unmarshal(k["tür"], &kind)
		switch kind {
		case "devam":
			metrics = slices.DeleteFunc(metrics, func(o metric) bool { return o.step > step })
		case "":
			metrics = append(metrics, metric{step, string(k["kayıp"])})
		}
	}
	var out []string
	for _, o := range metrics {
		out = append(out, fmt.Sprintf("%d:%s", o.step, o.lost))
	}
	return out
}

// sameResults iki koşunun parametre, iyileştirici dosyalarının, geçerli kayıp dizisinin ve
// programın son çıktısının (düz değerler, okuyucu durumu) bit düzeyinde aynı olduğunu denetler.
func sameResults(t *testing.T, a, b, outputA, outputB string) {
	t.Helper()
	for _, name := range []string{"son.trt", "op.trt", "op2.trt"} {
		x, err1 := os.ReadFile(filepath.Join(a, name))
		y, err2 := os.ReadFile(filepath.Join(b, name))
		if err1 != nil || err2 != nil {
			t.Fatalf("%s okunamadı: %v %v", name, err1, err2)
		}
		if !bytes.Equal(x, y) {
			t.Errorf("%s bit düzeyinde aynı değil", name)
		}
	}
	ka, kb := validMetrics(t, filepath.Join(a, "metrics.jsonl")), validMetrics(t, filepath.Join(b, "metrics.jsonl"))
	if !slices.Equal(ka, kb) {
		t.Errorf("geçerli kayıp dizileri farklı:\n%v\n%v", ka, kb)
	}
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
	inputs, _ := os.ReadDir(filepath.Join(b, "kontrol_noktalari"))
	var ckpts []string
	for _, g := range inputs {
		ckpts = append(ckpts, g.Name())
	}
	if len(ckpts) != 3 || ckpts[2] != "son.json" {
		t.Errorf("iki nokta ve son.json bekleniyordu: %v", ckpts)
	}
}

// Ağ sürücüsü (FUSE) benzetimi: yeniden adlandırma başarısız olunca aralıklı kayıt eğitimi
// durdurmaz (uyarı, sonraki aralıkta yeniden deneme), önceki geçerli nokta bozulmaz, geçici
// klasör kalmaz; art arda üçüncü başarısızlıkta açık hata verilir. Yeniden adlandırma
// düzelince kayıt sürer ve sayaç sıfırlanır.
func TestRenameErrorDoesNotStopTraining(t *testing.T) {
	var broken atomicBool
	t.Cleanup(checkpoint.FakeRename(func(old, fresh string) error {
		if broken.load() && filepath.Base(filepath.Dir(fresh)) == "kontrol_noktalari" && strings.HasPrefix(filepath.Base(fresh), "adim_") {
			return &os.LinkError{Op: "rename", Old: old, New: fresh, Err: errors.New("Input/output error")}
		}
		return os.Rename(old, fresh)
	}))
	runDir := t.TempDir()
	program := func(n int, brokenSteps string) string {
		return fmt.Sprintf(`
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken op = SGD([w], öğrenme_oranı: 0.1)
değişken kn = kontrol_noktası(%q, {w: w, op: op}, her_dakika: 0)
değişken sonuçlar = []
döngü adım içinde kn.kaldığı_adım..%d {
    geri_yayılım(toplam(w * w))
    op.adım()
    op.sıfırla()
    eğer [%s].içerir(adım + 1) { boz() }
    sonuçlar.ekle(kn.güncelle(adım + 1))
    düzelt()
}
yazdır("sonuçlar:", sonuçlar)
`, trPath(runDir), n, brokenSteps)
	}
	hooks := func(code string) string {
		return strings.ReplaceAll(strings.ReplaceAll(code, "boz()", "bozuk_yap(doğru)"), "düzelt()", "bozuk_yap(yanlış)")
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
	inputs, _ := os.ReadDir(filepath.Join(runDir, "kontrol_noktalari"))
	for _, g := range inputs {
		if strings.Contains(g.Name(), ".tmp-") || strings.Contains(g.Name(), ".eski-") {
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
	w := &teeWriter{}
	m.Out, m.Err = w, w
	m.GlobalEnv.Define("bozuk_yap", &interp.BuiltinVal{Name: "bozuk_yap", Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		broken.store(args[0] == interp.True)
		return interp.Null, nil
	}}, true)
	_, err := runVM(t, m, code)
	return w.String(), err
}

// resignCheckpoint noktanın ozet.json'unu (sürüm 2) durum.json ve tensorler.trt dosyalarının şimdiki
// boyları ve SHA-256 özetleriyle yeniden yazar: testler dosyayı bozduktan sonra özet doğru kalır ve
// yükleme özet denetiminden geçip içerik kurallarına ulaşır. Özet elle yazılır (üretimdeki yazıcı
// çağrılmaz: yoksa test kendi kendini doğrulardı).
func resignCheckpoint(t *testing.T, stepDir string, step int) {
	t.Helper()
	trtBytes, err := os.ReadFile(filepath.Join(stepDir, "tensorler.trt"))
	if err != nil {
		t.Fatal(err)
	}
	durumBytes, err := os.ReadFile(filepath.Join(stepDir, "durum.json"))
	if err != nil {
		t.Fatal(err)
	}
	trtSum := sha256.Sum256(trtBytes)
	durumSum := sha256.Sum256(durumBytes)
	ozet := fmt.Sprintf(`{
  "sürüm": 2,
  "adım": %d,
  "dosyalar": {
    "durum.json": {"bayt": %d, "sha256": %q},
    "tensorler.trt": {"bayt": %d, "sha256": %q}
  }
}
`, step, len(durumBytes), hex.EncodeToString(durumSum[:]), len(trtBytes), hex.EncodeToString(trtSum[:]))
	if err := os.WriteFile(filepath.Join(stepDir, "ozet.json"), []byte(ozet), 0o644); err != nil {
		t.Fatal(err)
	}
}

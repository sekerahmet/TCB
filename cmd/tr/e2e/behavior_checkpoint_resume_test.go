package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kontrol noktasının kabul testi (docs/spec/20_kontrol_noktasi.md): N adım kesintisiz eğitim
// ile K adımda kesilip (ayrı süreçte; Ctrl+C ile ve süreç öldürülerek) sürdürülen eğitim aynı
// N. adımda bit düzeyinde aynı parametreleri, iyileştirici durumunu, geçerli kayıp dizisini
// ve düz değerleri verir. Model (gömme, rms normu, nedensel dikkat, GELU katmanı, çapraz
// entropi, gradyan kırpma, AdamW) CPU çekirdeklerinin paralel yoluna girecek kadar büyüktür
// (çarpım ve [256, 2048] eleman bazlı işler); kesintisiz koşu GOMAXPROCS=1 ve 8 ile, öldürülen
// koşu 8 işçiyle başlayıp 1 işçiyle sürdürülür (indirgemeler işçi sayısından bağımsız olmalı).
//
// Test belirlenimcidir, zamana bağlı değildir: kayıtlar adım sayısıyla yapılır (--kayıt_her N:
// her N adımda kn.kaydet()) ve süreç --dur K ile K. adımın sınırında durup bekler (bekliyor.txt
// yazılır); kesme ve öldürme ancak o zaman gönderilir. Böylece eğitim kesilmeden bitemez ve
// kesmenin hangi adımda, son noktanın hangi adımda olduğu bellidir.

const checkTrainingProgram = `değişken ayar = komut_satırı({adım: 40, koşu: "", veri: "", kayıt_her: 0, dur: 0})
değişken V = 2048
değişken D = 64
değişken H = 128
değişken sayaç = 0
fonksiyon param(şekil, ölçek) {
    sayaç += 1
    sonuç tensör(türevsiz(rastgele_normal(şekil, tohum: sayaç) * ölçek), türev_izle: doğru)
}
değişken model = {E: param([V, D], 0.05), g: tensör(birler([D]), türev_izle: doğru), Wq: param([D, D], 0.1), Wk: param([D, D], 0.1),
    Wv: param([D, D], 0.1), W1: param([D, H], 0.1), W2: param([H, V], 0.1)}
değişken op = AdamW(parametreler(model), öğrenme_oranı: 0.003)
fonksiyon bakışlara_ayır(x) { sonuç x.şekillendir([8, 32, 4, 16]).eksenleri_sırala([0, 2, 1, 3]) }
fonksiyon ileri(giriş) {
    değişken x = gömme(model.E, giriş)
    değişken h = rms_normu(x, model.g)
    değişken y = dikkat(bakışlara_ayır(h @ model.Wq), bakışlara_ayır(h @ model.Wk), bakışlara_ayır(h @ model.Wv), nedensel: doğru)
    x = x + y.eksenleri_sırala([0, 2, 1, 3]).şekillendir([8, 32, D])
    sonuç gelu(x @ model.W1) @ model.W2
}
değişken ö = veri_kümesi(ayar.veri).örnekleyici(toplu: 8, bağlam: 32, tohum: 11)
değişken g = günlük(yol(ayar.koşu, "metrics.jsonl"))
değişken izlenen = {en_iyi: 99.0, adım_sayısı: 0}
değişken kn = kontrol_noktası(ayar.koşu, {model: model, op: op, ö: ö, g: g, izlenen: izlenen}, her_dakika: 60.0)
değişken ç = ilerleme(ayar.adım, günlük: g)
döngü adım içinde kn.kaldığı_adım..ayar.adım {
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    değişken kayıp = çapraz_entropi(ileri(giriş), hedef)
    geri_yayılım(kayıp)
    gradyan_kırp(parametreler(model), en_fazla: 1.0)
    op.adım()
    eğer kayıp.değer < izlenen.en_iyi { izlenen.en_iyi = kayıp.değer }
    izlenen.adım_sayısı += 1
    ç.güncelle({kayıp: kayıp}, adım: adım + 1)
    kn.güncelle(adım + 1)
    eğer ayar.kayıt_her > 0 ve (adım + 1) % ayar.kayıt_her == 0 { kn.kaydet() }
    eğer adım + 1 == ayar.dur {
        // Testin kesme ya da öldürme noktası: süreç bu adımın sınırında bekler.
        dosya_yaz(yol(ayar.koşu, "bekliyor.txt"), adım + 1)
        döngü doğru { kn.güncelle(adım + 1) }
    }
}
tensör_kaydet(yol(ayar.koşu, "son.trt"), parametreler(model, adlarıyla: doğru))
tensör_kaydet(yol(ayar.koşu, "op.trt"), op.durum)
yazdır("izlenen:", izlenen, ö.durum)
`

// checkData deterministik parça kimliklerinden dört .npy veri dosyası yazar.
func checkData(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 4 {
		v := make([]uint16, 20000)
		for j := range v {
			v[j] = uint16(r.IntN(2048))
		}
		var b bytes.Buffer
		if err := dataload.WriteNpyU16(&b, v); err != nil {
			t.Fatal(err)
		}
		testkit.WriteFile(t, d, fmt.Sprintf("parca_%d.npy", i), b.String())
	}
	return d
}

// trainingRun eğitim programını ayrı bir süreçte başlatır.
type trainingRun struct {
	cmd    *exec.Cmd
	run    string // koşu klasörü
	stderr *testkit.TeeWriter
	stdout *testkit.TeeWriter
	done   chan error
}

// startTraining eğitimi isci işçiyle başlatır; ek komut satırı ayarlarıdır (--dur, --kayıt_her).
func startTraining(t *testing.T, dir, run, data string, worker int, ek ...string) *trainingRun {
	t.Helper()
	cmd := exec.Command(trBinary, append([]string{"egit.tr", "--koşu", run, "--veri", data}, ek...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOMAXPROCS=%d", worker))
	k := &trainingRun{cmd: cmd, run: filepath.FromSlash(run), stderr: &testkit.TeeWriter{}, stdout: &testkit.TeeWriter{}, done: make(chan error, 1)}
	cmd.Stdout, cmd.Stderr = k.stdout, k.stderr
	prepareProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("eğitim başlatılamadı: %v", err)
	}
	go func() { k.done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		err := <-k.done
		k.done <- err
	})
	return k
}

// report başarısızlık mesajı için alt sürecin standart hatasını, standart çıktısını, günlüğün
// last satırlarını ve kontrol noktası klasörünün içeriğini verir.
func (k *trainingRun) report() string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- standart hata:\n%s\n--- standart çıktı:\n%s\n", k.stderr.String(), k.stdout.String())
	if g, err := os.ReadFile(filepath.Join(k.run, "metrics.jsonl")); err == nil {
		lines := strings.Split(strings.TrimRight(string(g), "\n"), "\n")
		fmt.Fprintf(&b, "--- günlük (%d satır, son 12):\n%s\n", len(lines), strings.Join(lines[max(0, len(lines)-12):], "\n"))
	}
	if g, err := os.ReadDir(filepath.Join(k.run, "kontrol_noktalari")); err == nil {
		var names []string
		for _, e := range g {
			names = append(names, e.Name())
		}
		fmt.Fprintf(&b, "--- kontrol_noktalari: %v\n", names)
	}
	return b.String()
}

// waitForExit sürecin bitmesini (en çok 2 dk) bekler; çıkış kodunu ve bitmediyse yanlış verir.
func (k *trainingRun) waitForExit() (int, bool, error) {
	select {
	case err := <-k.done:
		k.done <- err
		var ch *exec.ExitError
		if errors.As(err, &ch) {
			return ch.ExitCode(), true, nil
		}
		return 0, true, err
	case <-time.After(2 * time.Minute):
		return -1, false, nil
	}
}

// wait sürecin bitmesini (en çok 2 dk) bekler ve çıkış kodunu verir.
func (k *trainingRun) wait(t *testing.T) int {
	t.Helper()
	code, done, err := k.waitForExit()
	if !done {
		t.Fatalf("eğitim 2 dk içinde bitmedi\n%s", k.report())
	}
	if err != nil {
		t.Fatalf("eğitim beklenemedi: %v\n%s", err, k.report())
	}
	return code
}

// train eğitimi baştan sona (ya da kaldığı yerden sona) çalıştırır ve çıktısını verir.
func train(t *testing.T, dir, run, data string, worker int, ek ...string) string {
	t.Helper()
	k := startTraining(t, dir, run, data, worker, ek...)
	if code := k.wait(t); code != 0 {
		t.Fatalf("eğitim %d koduyla bitti\n%s", code, k.report())
	}
	if strings.Contains(k.stderr.String(), "Uyarı") {
		t.Errorf("uyarı beklenmiyordu\n%s", k.report())
	}
	return k.stdout.String()
}

// waitForStop sürecin --dur adımının sınırına gelip beklemeye geçmesini (bekliyor.txt; en çok
// 2 dk) bekler. Süreç bu sırada biterse ya da süre dolarsa test raporla kalır.
func (k *trainingRun) waitForStop(t *testing.T, step int) {
	t.Helper()
	mark := filepath.Join(k.run, "bekliyor.txt")
	last := time.Now().Add(2 * time.Minute)
	for {
		if b, err := os.ReadFile(mark); err == nil && strings.TrimSpace(string(b)) == strconv.Itoa(step) {
			return
		}
		select {
		case err := <-k.done:
			k.done <- err
			t.Fatalf("eğitim %d. adımda beklemeden bitti (%v)\n%s", step, err, k.report())
		case <-time.After(5 * time.Millisecond):
		}
		if time.Now().After(last) {
			t.Fatalf("eğitim 2 dk içinde %d. adıma gelmedi\n%s", step, k.report())
		}
	}
}

// measureCount günlükteki ölçü kaydı sayısıdır (yarım son satır sayılmaz).
func measureCount(filePath string) int {
	b, _ := os.ReadFile(filePath)
	return strings.Count(string(b), `"kayıp":`) - strings.Count(string(b[bytes.LastIndexByte(b, '\n')+1:]), `"kayıp":`)
}

// validLosses günlüğün geçerli ölçü kayıtlarını "adım:kayıp" olarak verir: bir devam
// kaydından önce gelen ve adımı ondan büyük olan kayıtlar geçersizdir (19, günlük okuyucu kuralı).
func validLosses(t *testing.T, filePath string) []string {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	type measure struct {
		step int64
		loss string
	}
	var measures []measure
	for _, s := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var k map[string]json.RawMessage
		if json.Unmarshal([]byte(s), &k) != nil {
			continue // öldürülen sürecin yarım kalmış son satırı
		}
		var step int64
		var tur string
		_ = json.Unmarshal(k["adım"], &step)
		_ = json.Unmarshal(k["tür"], &tur)
		switch {
		case tur == "devam":
			measures = slices.DeleteFunc(measures, func(o measure) bool { return o.step > step })
		case tur == "" && k["kayıp"] != nil:
			measures = append(measures, measure{step, string(k["kayıp"])})
		}
	}
	var out []string
	for _, o := range measures {
		out = append(out, fmt.Sprintf("%d:%s", o.step, o.loss))
	}
	return out
}

// isSame iki koşunun sonucunu bit düzeyinde karşılaştırır.
func isSame(t *testing.T, ne, a, b, stdoutA, stdoutB string) {
	t.Helper()
	for _, ad := range []string{"son.trt", "op.trt"} {
		x, err1 := os.ReadFile(filepath.Join(a, ad))
		y, err2 := os.ReadFile(filepath.Join(b, ad))
		if err1 != nil || err2 != nil || !bytes.Equal(x, y) {
			t.Errorf("%s: %s bit düzeyinde aynı değil (%v %v)", ne, ad, err1, err2)
		}
	}
	ka, kb := validLosses(t, filepath.Join(a, "metrics.jsonl")), validLosses(t, filepath.Join(b, "metrics.jsonl"))
	if len(ka) != 40 || !slices.Equal(ka, kb) {
		t.Errorf("%s: geçerli kayıp dizileri farklı (%d, %d kayıt):\n%v\n%v", ne, len(ka), len(kb), ka, kb)
	}
	if stdoutA != stdoutB {
		t.Errorf("%s: son durum farklı:\n%s\n%s", ne, stdoutA, stdoutB)
	}
}

// resumeSteps günlükteki devam kayıtlarının adımlarıdır.
func resumeSteps(t *testing.T, filePath string) []int64 {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	var out []int64
	for _, s := range strings.Split(string(data), "\n") {
		var k struct {
			Kind string `json:"tür"`
			Step int64  `json:"adım"`
		}
		if json.Unmarshal([]byte(s), &k) == nil && k.Kind == "devam" {
			out = append(out, k.Step)
		}
	}
	return out
}

// lastPointStep kontrol_noktalari/son.json'un söylediği adımdır.
func lastPointStep(t *testing.T, run string) int64 {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(run, "kontrol_noktalari", "son.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Step int64 `json:"adım"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("son.json çözülemedi: %v (%s)", err, b)
	}
	return s.Step
}

// copyDir bir nokta klasörünü kopyalar; yarim > 0 ise her dosyanın yalnızca ilk yarim
// baytını yazar (öldürülen bir kaydın yarım kalmış dosyaları).
func copyDir(t *testing.T, source, target string, half int64) {
	t.Helper()
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	inputs, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range inputs {
		b, err := os.ReadFile(filepath.Join(source, g.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if half > 0 {
			b = b[:min(int64(len(b)), half)]
		}
		if err := os.WriteFile(filepath.Join(target, g.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCheckpointResumeBitExact(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "egit.tr", checkTrainingProgram)
	data := checkData(t)
	run := func(ad string) string { return filepath.ToSlash(filepath.Join(dir, ad)) }

	stdoutA := train(t, dir, run("kesintisiz8"), data, 8)
	isSame(t, "GOMAXPROCS=1 ve 8", run("kesintisiz8"), run("kesintisiz1"), stdoutA, train(t, dir, run("kesintisiz1"), data, 1))

	t.Run("Ctrl+C", func(t *testing.T) {
		k := startTraining(t, dir, run("kesme"), data, 8, "--dur", "12")
		k.waitForStop(t, 12)
		sendInterrupt(t, k.cmd)
		if code := k.wait(t); code != 3 {
			t.Fatalf("çıkış kodu %d, 3 bekleniyordu\n%s", code, k.report())
		}
		if h := k.stderr.String(); !strings.Contains(h, "adım bitince kontrol noktası kaydedilip çıkılacak") || !strings.Contains(h, "Kontrol noktası kaydedildi: adım 12") {
			t.Fatalf("durdurma bildirimleri yok\n%s", k.report())
		}
		isSame(t, "Ctrl+C ve devam", run("kesintisiz8"), run("kesme"), stdoutA, train(t, dir, run("kesme"), data, 8))
		if d := resumeSteps(t, filepath.Join(dir, "kesme", "metrics.jsonl")); !slices.Equal(d, []int64{12}) {
			t.Errorf("günlükte adım 12'de bir devam kaydı bekleniyordu, %v var\n%s", d, k.report())
		}
	})

	// killAndResume eğitimi 8 işçiyle 25. adımda (son nokta 20. adımda) öldürür, koşu klasörünü
	// breaker ile değiştirir (kayıt ortasında öldürülmenin bıraktığı durum) ve 1 işçiyle sürdürür.
	// 21-25. adımlar yeniden hesaplanır: günlükte devamdan önce geçersiz kalan 5 kayıt vardır.
	killAndResume := func(t *testing.T, ad string, breaker func(points string, pid int)) {
		k := startTraining(t, dir, run(ad), data, 8, "--dur", "25", "--kayıt_her", "10")
		k.waitForStop(t, 25)
		if n := lastPointStep(t, k.run); n != 20 {
			t.Fatalf("öldürmeden önce son nokta adım %d, 20 bekleniyordu\n%s", n, k.report())
		}
		if err := k.cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		k.wait(t)
		if breaker != nil {
			breaker(filepath.Join(k.run, "kontrol_noktalari"), k.cmd.Process.Pid)
		}
		d := startTraining(t, dir, run(ad), data, 1, "--kayıt_her", "10")
		if code := d.wait(t); code != 0 || strings.Contains(d.stderr.String(), "Uyarı") || !strings.Contains(d.stderr.String(), "Kaldığı yerden devam: adım 20 ") {
			t.Fatalf("devam %d koduyla bitti ya da 20. adımdan sürmedi\n%s", code, d.report())
		}
		isSame(t, ad+" ve 1 işçiyle devam", run("kesintisiz8"), run(ad), stdoutA, d.stdout.String())
		if n := measureCount(filepath.Join(k.run, "metrics.jsonl")); n != 45 {
			t.Errorf("günlükte %d ölçü kaydı, 45 (25 + yeniden hesaplanan 20) bekleniyordu\n%s", n, d.report())
		}
		if a := resumeSteps(t, filepath.Join(k.run, "metrics.jsonl")); !slices.Equal(a, []int64{20}) {
			t.Errorf("günlükte adım 20'de bir devam kaydı bekleniyordu, %v var\n%s", a, d.report())
		}
		inputs, _ := os.ReadDir(filepath.Join(k.run, "kontrol_noktalari"))
		for _, g := range inputs {
			if strings.Contains(g.Name(), ".tmp-") || strings.Contains(g.Name(), ".eski-") {
				t.Errorf("yarım kaydın artığı kaldı: %s\n%s", g.Name(), d.report())
			}
		}
	}
	t.Run("öldürme", func(t *testing.T) { killAndResume(t, "oldurme", nil) })
	t.Run("kayıt ortasında öldürme", func(t *testing.T) {
		// 30. adımın kaydı yazılırken öldürülmüş gibi: geçici klasörde yarım dosyalar.
		killAndResume(t, "yarim_kayit", func(points string, pid int) {
			copyDir(t, filepath.Join(points, "adim_0000000020"), filepath.Join(points, "adim_0000000030.tmp-"+strconv.Itoa(pid)), 4096)
		})
	})
	t.Run("aynı adımlı nokta yerine konurken öldürme", func(t *testing.T) {
		// Aynı adımlı noktanın yerine yenisi konurken öldürülmüş gibi: eskisi kenara alınmış,
		// yenisi geçici klasörde (tam), yerine konmamış.
		killAndResume(t, "yerine_koyma", func(points string, pid int) {
			actual := filepath.Join(points, "adim_0000000020")
			copyDir(t, actual, filepath.Join(points, "adim_0000000020.tmp-"+strconv.Itoa(pid)), 0)
			if err := os.Rename(actual, actual+".eski-"+strconv.Itoa(pid)); err != nil {
				t.Fatal(err)
			}
		})
	})
}

package checkpoint_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
)

// Kontrol noktası dosyaları güvenilmeyen girdidir (SECURITY.md): kurcalanmış, yarım
// kopyalanmış ya da daha yeni bir biçimle yazılmış nokta açık bir hatayla reddedilir; biçim
// yalnızca veri taşır, yol çözmez.

// buildCkpt tek parametreli bir modelin 1. ve 2. adım noktalarını yazar; klasörü verir.
func buildCkpt(t *testing.T) string {
	t.Helper()
	runDir := t.TempDir()
	output, err := run(t, fmt.Sprintf(`
değişken w = tensör([1.0, 2.0, 3.0], türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w, op: AdamW([w]), d: {a: 1}})
kn.güncelle(1)
kn.kaydet()
kn.güncelle(2)
kn.kaydet()
`, trPath(runDir)))
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	return runDir
}

func load(t *testing.T, runDir string) (string, error) {
	t.Helper()
	return run(t, fmt.Sprintf(`
değişken w = tensör([0.0, 0.0, 0.0], türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w, op: AdamW([w]), d: {a: 1}})
yazdır("adım", kn.kaldığı_adım, w.değer)
`, trPath(runDir)))
}

func TestTamperedCheckpointRejected(t *testing.T) {
	states := []struct {
		name, file string
		corrupt    func([]byte) []byte
		want       messages.Code
		detail     string // kodu olmayan iç neden (ml, encoding/json)
	}{
		{"tensör baytı", "tensorler.trt", func(b []byte) []byte { b[len(b)/2] ^= 1; return b }, messages.H2000, "tensorler.trt: dosya bozuk: sağlama toplamı tutmuyor"},
		{"yarım tensör", "tensorler.trt", func(b []byte) []byte { return b[:len(b)-7] }, messages.H7474, ""},
		{"yeni sürüm", "ozet.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `"sürüm": 2`, `"sürüm": 3`, 1)) }, messages.H7421, ""},
		{"sürüm 0", "ozet.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `"sürüm": 2`, `"sürüm": 0`, 1)) }, messages.H7422, ""},
		{"özet ile durumun sürümü farklı", "ozet.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `"sürüm": 2`, `"sürüm": 1`, 1)) }, messages.H7473, ""},
		{"bilinmeyen alan", "ozet.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `{`, `{"yol": "../../x", `, 1)) }, messages.H7423, "unknown field"},
		{"başka adım", "ozet.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `"adım": 2`, `"adım": 3`, 1)) }, messages.H7424, ""},
		{"büyük özet", "ozet.json", func(b []byte) []byte { return append(b, make([]byte, 70<<10)...) }, messages.H7477, "ozet.json çok büyük"},
		{"eksik özet", "ozet.json", func([]byte) []byte { return nil }, messages.H7423, ""},
	}
	for _, d := range states {
		t.Run(d.name, func(t *testing.T) {
			runDir := buildCkpt(t)
			fpath := filepath.Join(runDir, "kontrol_noktalari", "adim_0000000002", d.file)
			b, err := os.ReadFile(fpath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fpath, d.corrupt(b), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = load(t, runDir)
			if !messages.Has(err, messages.H7472) || !messages.Has(err, d.want) || !strings.Contains(err.Error(), d.detail) ||
				!strings.Contains(err.Error(), messages.Text(messages.H7471, "adim_0000000001", 1)) {
				t.Fatalf("%s ve bir önceki nokta önerisi bekleniyordu: %v", d.want, err)
			}
		})
	}
}

// son.json yalnızca araçlar içindir: bozuk ya da var olmayan bir noktayı gösterse de yükleme
// klasörlere bakar. Kenara alınmış (aynı adımlı yenisi yerine konamadan kesilmiş) bir nokta
// yerine geri konur.
func TestLatestJSONAndStrayCheckpoint(t *testing.T) {
	runDir := buildCkpt(t)
	kn := filepath.Join(runDir, "kontrol_noktalari")
	if err := os.WriteFile(filepath.Join(kn, "son.json"), []byte(`{"klasör": "../../../etc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := load(t, runDir); err != nil || !strings.Contains(output, "adım 2 [1.") {
		t.Fatalf("2. adımdan sürmeliydi: %v\n%s", err, output)
	}
	if err := os.Rename(filepath.Join(kn, "adim_0000000002"), filepath.Join(kn, "adim_0000000002.eski-77")); err != nil {
		t.Fatal(err)
	}
	if output, err := load(t, runDir); err != nil || !strings.Contains(output, "Kaldığı yerden devam: adım 2") {
		t.Fatalf("kenardaki nokta geri konmalıydı: %v\n%s", err, output)
	}
}

// Kesme isteği kontrol noktası kurulduktan sonra gelirse ertelenir: sıradaki güncelle noktayı
// kaydeder, program nazikçe durur (ertelenen işler çalışır, Run ErrKesildi verir).
func TestDeferredInterruptSavesAtStepBoundary(t *testing.T) {
	runDir := t.TempDir()
	m := interp.New()
	w := &teeWriter{}
	m.Out, m.Err = w, w
	m.GlobalEnv.Define("kesme_iste", &interp.BuiltinVal{Name: "kesme_iste", Fn: func(m *interp.VM, _ []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		return interp.BoolToVal(m.RequestInterrupt()), nil
	}}, true)
	_, err := runVM(t, m, fmt.Sprintf(`
ertele { yazdır("ertelenen çalıştı") }
değişken w = tensör([1.0], türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w})
döngü adım içinde 0..10 {
    eğer adım == 3 { yazdır("ertelendi:", kesme_iste()) }
    kn.güncelle(adım + 1)
    yazdır("adım bitti", adım + 1)
}
yazdır("döngü bitti")
`, trPath(runDir)))
	output := w.String()
	if !errors.Is(err, interp.ErrInterrupted) {
		t.Fatalf("program kesilmeliydi: %v\n%s", err, output)
	}
	for _, s := range []string{"ertelendi: doğru", "adım bitti 3", "Kontrol noktası kaydedildi: adım 4", "ertelenen çalıştı"} {
		if !strings.Contains(output, s) {
			t.Errorf("%q bekleniyordu:\n%s", s, output)
		}
	}
	if strings.Contains(output, "adım bitti 4") || strings.Contains(output, "döngü bitti") {
		t.Errorf("program adım sınırında durmalıydı:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(runDir, "kontrol_noktalari", "adim_0000000004")); err != nil {
		t.Errorf("4. adımın noktası yok: %v", err)
	}
}

// Kontrol noktası kurulduktan sonra tohumsuz rastgelelik kullanılırsa bir kez uyarılır.
func TestUnseededRandomWarning(t *testing.T) {
	output, err := run(t, fmt.Sprintf(`
değişken w = tensör(rastgele_normal([2]), türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w})
rastgele_normal([2], tohum: 1)
rastgele_normal([2])
örnekle(tensör([[0.1, 0.2]]), sıcaklık: 1.0)
`, trPath(t.TempDir())))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(output, "tohumsuz rastgelelik kullanıldı") != 1 || !strings.Contains(output, "rastgele_normal ya da örnekle, tohum: verilmeden") {
		t.Fatalf("bir kez uyarı bekleniyordu:\n%s", output)
	}
}

// Biçim çözücüleri rastgele girdide çökmez; geçerli bir noktanın dosyaları tohumdur.
func FuzzCheckpointFiles(f *testing.F) {
	f.Add([]byte(`{"sürüm":1,"adım":2,"dosyalar":{"durum.json":{"bayt":1,"sha256":"00"},"tensorler.trt":{"bayt":1,"sha256":"00"}}}`))
	f.Add([]byte(`{"sürüm":1,"adım":2,"zaman":"","tr":"","program":"","işçi":1,"nesneler":[{"ad":"d","tür":"değerler","değer":{"sözlük":[["a",1]]}}]}`))
	f.Add([]byte(`{"sözlük":[[1,{"ondalık":"NaN"}],["b",[null,true,"x"]]]}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = checkpoint.DecodeDigest(b, 2)
		_, _ = checkpoint.DecodeState(b, 2)
		if v, err := checkpoint.ReadPlain(b); err == nil {
			if _, err := checkpoint.WritePlain(v); err != nil {
				t.Fatalf("okunan değer yeniden yazılamadı: %v", err)
			}
		}
	})
}

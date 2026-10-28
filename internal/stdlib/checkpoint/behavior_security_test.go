package checkpoint_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kontrol noktası dosyaları güvenilmeyen girdidir (SECURITY.md): kurcalanmış, yarım
// kopyalanmış ya da daha yeni bir biçimle yazılmış nokta açık bir hatayla reddedilir; biçim
// yalnızca veri taşır, yol çözmez.

// buildCkpt tek parametreli bir modelin 1. ve 2. adım noktalarını yazar; klasörü verir.
func buildCkpt(t *testing.T) string {
	t.Helper()
	runDir := t.TempDir()
	output, err := run(t, fmt.Sprintf(`
TR_var w = TR_tensor([1.0, 2.0, 3.0], TR_requires_grad: TR_true)
TR_var kn = TR_checkpoint(%q, {"w": w, "op": TR_AdamW([w]), "d": {"a": 1}})
kn.TR_update(1)
kn.TR_save()
kn.TR_update(2)
kn.TR_save()
`, trPath(runDir)))
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	return runDir
}

func load(t *testing.T, runDir string) (string, error) {
	t.Helper()
	return run(t, fmt.Sprintf(`
TR_var w = TR_tensor([0.0, 0.0, 0.0], TR_requires_grad: TR_true)
TR_var kn = TR_checkpoint(%q, {"w": w, "op": TR_AdamW([w]), "d": {"a": 1}})
TR_print("adım", kn.TR_resume_step, w.TR_tolist)
`, trPath(runDir)))
}

func TestTamperedCheckpointRejected(t *testing.T) {
	states := []struct {
		name, file string
		corrupt    func([]byte) []byte
		want       messages.Code
		detail     string // kodu olmayan iç neden (ml, encoding/json)
	}{
		{"tensör baytı", "tensors.trt", func(b []byte) []byte { b[len(b)/2] ^= 1; return b }, messages.H2000, "tensors.trt: dosya bozuk: sağlama toplamı tutmuyor"},
		{"yarım tensör", "tensors.trt", func(b []byte) []byte { return b[:len(b)-7] }, messages.H7474, ""},
		{"yeni sürüm", "digest.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `"version": 3`, `"version": 4`, 1)) }, messages.H7421, ""},
		{"sürüm 0", "digest.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `"version": 3`, `"version": 0`, 1)) }, messages.H7422, ""},
		{"eski sürüm okunmaz", "digest.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `"version": 3`, `"version": 2`, 1)) }, messages.H7422, ""},
		{"bilinmeyen alan", "digest.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `{`, `{"yol": "../../x", `, 1)) }, messages.H7423, "unknown field"},
		{"başka adım", "digest.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `"step": 2`, `"step": 3`, 1)) }, messages.H7424, ""},
		{"büyük özet", "digest.json", func(b []byte) []byte { return append(b, make([]byte, 70<<10)...) }, messages.H6600, "digest.json çok büyük"},
		{"eksik özet", "digest.json", func([]byte) []byte { return nil }, messages.H7423, ""},
	}
	for _, d := range states {
		t.Run(d.name, func(t *testing.T) {
			runDir := buildCkpt(t)
			fpath := filepath.Join(runDir, "checkpoints", "step_0000000002", d.file)
			b, err := os.ReadFile(fpath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fpath, d.corrupt(b), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = load(t, runDir)
			if !messages.Has(err, messages.H7472) || !messages.Has(err, d.want) || !strings.Contains(err.Error(), d.detail) ||
				!strings.Contains(err.Error(), messages.Text(messages.H7471, "step_0000000001", 1)) {
				t.Fatalf("%s ve bir önceki nokta önerisi bekleniyordu: %v", d.want, err)
			}
		})
	}
}

// latest.json yalnızca araçlar içindir: bozuk ya da var olmayan bir noktayı gösterse de yükleme
// klasörlere bakar. Kenara alınmış (aynı adımlı yenisi yerine konamadan kesilmiş) bir nokta
// yerine geri konur.
func TestLatestJSONAndStrayCheckpoint(t *testing.T) {
	runDir := buildCkpt(t)
	kn := filepath.Join(runDir, "checkpoints")
	if err := os.WriteFile(filepath.Join(kn, "latest.json"), []byte(`{"dir": "../../../etc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := load(t, runDir); err != nil || !strings.Contains(output, "adım 2 [1.") {
		t.Fatalf("2. adımdan sürmeliydi: %v\n%s", err, output)
	}
	if err := os.Rename(filepath.Join(kn, "step_0000000002"), filepath.Join(kn, "step_0000000002.old-77")); err != nil {
		t.Fatal(err)
	}
	if output, err := load(t, runDir); err != nil || !strings.Contains(output, "Kaldığı yerden devam: adım 2") {
		t.Fatalf("kenardaki nokta geri konmalıydı: %v\n%s", err, output)
	}
}

// Yazım atomiktir: süreç yazarken ölürse geride yarım bir geçici klasör (step_N.tmp-<süreç>)
// kalır; eski noktalar bozulmaz, sonraki açılışta geçici klasör silinir.
func TestStrayTempCheckpointCleaned(t *testing.T) {
	runDir := buildCkpt(t)
	stray := filepath.Join(runDir, "checkpoints", "step_0000000003.tmp-4242")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stray, "tensors.trt"), []byte("yarım"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := load(t, runDir); err != nil || !strings.Contains(output, "adım 2 [1.") {
		t.Fatalf("yarım yazım son sağlam noktayı bozmamalı: %v\n%s", err, output)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Fatalf("geçici klasör açılışta silinmeliydi: %v", err)
	}
}

// Durdurma isteği kontrol noktası kurulduktan ve iki güncelleme yapıldıktan sonra gelirse
// ertelenir: sıradaki güncelle noktayı kaydeder, program nazikçe durur (ertelenen işler çalışır,
// Run ErrInterrupted verir). Saat her okunuşta bir saniye ilerler: adım süresi bir saniye,
// erteleme süresi iki saniyedir (1.6).
func TestDeferredInterruptSavesAtStepBoundary(t *testing.T) {
	runDir := t.TempDir()
	m := interp.New()
	w := &testkit.TeeWriter{}
	m.Out, m.Err = w, w
	ticks := 0
	defer checkpoint.FakeClock(func() time.Time { ticks++; return time.Unix(0, 0).Add(time.Duration(ticks) * time.Second) })()
	m.GlobalEnv.Define("kesme_iste", &interp.BuiltinVal{Name: "kesme_iste", Fn: func(m *interp.VM, _ []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		return interp.BoolToVal(m.RequestStop(2) == interp.StopDeferred), nil
	}}, true)
	_, err := runVM(t, m, fmt.Sprintf(`
TR_defer { TR_print("ertelenen çalıştı") }
TR_var w = TR_tensor([1.0], TR_requires_grad: TR_true)
TR_var kn = TR_checkpoint(%q, {"w": w})
TR_for adım TR_in 0..10 {
    TR_if adım == 3 { TR_print("ertelendi:", kesme_iste()) }
    kn.TR_update(adım + 1)
    TR_print("adım bitti", adım + 1)
}
TR_print("döngü bitti")
`, trPath(runDir)))
	output := w.String()
	if !errors.Is(err, interp.ErrInterrupted) {
		t.Fatalf("program kesilmeliydi: %v\n%s", err, output)
	}
	for _, s := range []string{"ertelendi: true", "adım bitti 3", "Kontrol noktası kaydedildi: adım 4", "ertelenen çalıştı"} {
		if !strings.Contains(output, s) {
			t.Errorf("%q bekleniyordu:\n%s", s, output)
		}
	}
	if strings.Contains(output, "adım bitti 4") || strings.Contains(output, "döngü bitti") {
		t.Errorf("program adım sınırında durmalıydı:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(runDir, "checkpoints", "step_0000000004")); err != nil {
		t.Errorf("4. adımın noktası none: %v", err)
	}
}

// Kontrol noktası kurulduktan sonra tohumsuz rastgelelik kullanılırsa bir kez uyarılır.
func TestUnseededRandomWarning(t *testing.T) {
	output, err := run(t, fmt.Sprintf(`
TR_var w = TR_tensor(TR_randn([2]), TR_requires_grad: TR_true)
TR_var kn = TR_checkpoint(%q, {"w": w})
TR_randn([2], TR_seed: 1)
TR_randn([2])
TR_sample(TR_tensor([[0.1, 0.2]]), TR_temperature: 1.0)
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
	f.Add([]byte(`{"version":3,"step":2,"files":{"state.json":{"bytes":1,"sha256":"00"},"tensors.trt":{"bytes":1,"sha256":"00"}}}`))
	f.Add([]byte(`{"version":3,"step":2,"time":"","tr":"","program":"","workers":1,"objects":[{"name":"d","kind":"values","value":{"dict":[["a",1]]}}]}`))
	f.Add([]byte(`{"dict":[[1,{"float":"NaN"}],["b",[null,true,"x"]]]}`))
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

// Kural (20.7, 20.9): noktanın dosyaları sağlam ama içeriği kurallara uymuyorsa (geçersiz
// iyileştirici adım sayacı) yükleme hiçbir nesneyi değiştirmez ve hata bir önceki geçerli noktayı
// önerir. Yükleme iki aşamalıdır: önce her şey denetlenir, sonra yazılır.
// Neden: eskiden parametreler iyileştiriciden önce yerinde yazılıyor, iyileştiricinin hatası
// modeli yarı yüklenmiş bırakıyordu (Gemini güvenlik denetimi, bulgu 3).
func TestCorruptOptimizerStateChangesNothingAndSuggestsPrevious(t *testing.T) {
	runDir := buildCkpt(t)
	step2Dir := filepath.Join(runDir, "checkpoints", "step_0000000002")

	// step_0000000002/tensors.trt içine w=[99, 99, 99] ve geçersiz op/step=[-1, 0]
	// yazıp digest.json'daki boy ve SHA-256 özetini güncelliyoruz.
	trtPath := filepath.Join(step2Dir, "tensors.trt")
	_, err := run(t, fmt.Sprintf(`
TR_save_tensors(%q, {
    "w/0": TR_tensor([99.0, 99.0, 99.0]),
    "op/step": TR_tensor([-1.0, 0.0])
})
`, trPath(trtPath)))
	if err != nil {
		t.Fatalf("tensör_kaydet başarısız: %v", err)
	}

	resignCheckpoint(t, step2Dir, 2)

	// Başlangıçta w = [1.0, 2.0, 3.0]. Yükleme hata verdiğinde w hiç değişmemiş
	// olmalı ve hata mesajı bir önceki sağlam noktayı (step_0000000001) önermelidir.
	output, err := run(t, fmt.Sprintf(`
TR_var w = TR_tensor([1.0, 2.0, 3.0], TR_requires_grad: TR_true)
TR_var hata_metni = ""
TR_try {
    TR_checkpoint(%q, {"w": w, "op": TR_AdamW([w]), "d": {"a": 1}})
} TR_catch h {
    hata_metni = TR_str(h)
}
TR_print("W:", w.TR_tolist)
TR_print("HATA:", hata_metni)
`, trPath(runDir)))
	if err != nil {
		t.Fatalf("beklenmeyen çalışma hatası: %v\n%s", err, output)
	}
	if !strings.Contains(output, "W: [1.0, 2.0, 3.0]") {
		t.Errorf("kontrol_noktası yükleme hatasında model parametresi w yerinde bozuldu (atomiklik ihlali):\n%s", output)
	}
	// Yakalanan hatanın metni dilden bağımsızdır (İngilizce); program onu yazdırır.
	wantHint := messages.TextIn("en", messages.H7471, "step_0000000001", 1)
	if !strings.Contains(output, wantHint) {
		t.Errorf("bozuk son noktada bir önceki sağlam nokta (%s) önerilmedi:\n%s", wantHint, output)
	}
}

// Kural (20, Biçim ve güvenlik): düz değerin JSON'u da katı çözülür; ilk değerden sonra
// fazladan veri hatadır (digest.json ve state.json gibi).
// Neden: kurcalanmış bir state.json'da ikinci bir değer sessizce yok sayılmasın (Gemini
// güvenlik denetimi, bulgu 10).
func TestPlainValueRejectsTrailingData(t *testing.T) {
	v, err := checkpoint.ReadPlain([]byte(`42 {"fazladan": "veri"}`))
	if err == nil {
		t.Fatalf("ReadPlain fazladan JSON verisini reddetmeliydi; alınan değer: %v", v)
	}
}

// Kural (20.7, 20.9): dosyaları sağlam (özetler tutuyor) ama içeriği kurallara uymayan nokta
// bozuk sayılır ve bir önceki geçerli noktayı önerir: geçersiz okuyucu durumu, bozuk düz değer
// JSON'u, şekli bozuk adım sayacı. Kullanıcının programındaki fark (başka tohum, toplu, bağlam,
// veri kümesi, model) bozukluk değil uyumsuzluktur; önceki nokta önerilmez.
// Neden: eskiden yalnızca iyileştiricinin değer denetimi önceki noktayı öneriyordu (Gemini
// kararlılık denetimi, bulgu 3).
func TestCorruptContentSuggestsPreviousButMismatchDoesNot(t *testing.T) {
	dataDir := trPath(filepath.Join(reporoot.Root(), "tests", "data", "text"))

	buildWithReader := func(t *testing.T) string {
		t.Helper()
		runDir := t.TempDir()
		out, err := run(t, fmt.Sprintf(`
TR_var w = TR_tensor([1.0, 2.0, 3.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w])
TR_var ö = TR_dataset(%q).TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1)
TR_var d = {"a": 1}
TR_var kn = TR_checkpoint(%q, {"w": w, "op": op, "ö": ö, "d": d})
ö.TR_next()
kn.TR_update(1)
kn.TR_save()
ö.TR_next()
kn.TR_update(2)
kn.TR_save()
`, dataDir, trPath(runDir)))
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return runDir
	}

	wantHint := messages.Text(messages.H7471, "step_0000000001", 1)

	cases := []struct {
		name   string
		tamper func(t *testing.T, step2Dir string)
	}{
		{
			name: "okuyucu_durumunda_negatif_pencere",
			tamper: func(t *testing.T, step2Dir string) {
				durumPath := filepath.Join(step2Dir, "state.json")
				b, err := os.ReadFile(durumPath)
				if err != nil {
					t.Fatal(err)
				}
				corrupted := strings.Replace(string(b), "\"window\",\n            4", "\"window\",\n            -1", 1)
				if corrupted == string(b) {
					t.Fatalf("pencere alanı state.json içinde bulunamadı:\n%s", string(b))
				}
				if err := os.WriteFile(durumPath, []byte(corrupted), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "duz_degerde_fazladan_json",
			tamper: func(t *testing.T, step2Dir string) {
				durumPath := filepath.Join(step2Dir, "state.json")
				b, err := os.ReadFile(durumPath)
				if err != nil {
					t.Fatal(err)
				}
				corrupted := strings.Replace(string(b), "\"a\",\n            1", "\"a\",\n            {\"float\":\"bozuk\"}", 1)
				if corrupted == string(b) {
					t.Fatalf("d alanı state.json içinde bulunamadı:\n%s", string(b))
				}
				if err := os.WriteFile(durumPath, []byte(corrupted), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "iyilestirici_adim_sayaci_bozuk_sekil",
			tamper: func(t *testing.T, step2Dir string) {
				trtPath := filepath.Join(step2Dir, "tensors.trt")
				if _, err := run(t, fmt.Sprintf(`
TR_save_tensors(%q, {
    "w/0": TR_tensor([99.0, 99.0, 99.0]),
    "op/step": TR_tensor([1.0, 0.0, 0.0])
})
`, trPath(trtPath))); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runDir := buildWithReader(t)
			step2Dir := filepath.Join(runDir, "checkpoints", "step_0000000002")
			tc.tamper(t, step2Dir)
			resignCheckpoint(t, filepath.Join(runDir, "checkpoints", "step_0000000002"), 2)

			_, err := run(t, fmt.Sprintf(`
TR_var w = TR_tensor([1.0, 2.0, 3.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w])
TR_var ö = TR_dataset(%q).TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1)
TR_var d = {"a": 1}
TR_checkpoint(%q, {"w": w, "op": op, "ö": ö, "d": d})
`, dataDir, trPath(runDir)))
			if err == nil {
				t.Fatal("bozuk içerikli kontrol noktası reddedilmeliydi")
			}
			if !strings.Contains(err.Error(), wantHint) {
				t.Errorf("bozuk içerikli son noktada bir önceki sağlam nokta (%s) önerilmedi:\n%v", wantHint, err)
			}
		})
	}

	// Karşıt vaka: noktaya dokunulmadı, kullanıcı okuyucunun tohumunu değiştirdi. Bu bozukluk
	// değil uyumsuzluktur: hata verilir ama önceki nokta önerilmez (o da aynı tohumla yazıldı).
	runDir := buildWithReader(t)
	_, err := run(t, fmt.Sprintf(`
TR_var w = TR_tensor([1.0, 2.0, 3.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w])
TR_var ö = TR_dataset(%q).TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 2)
TR_var d = {"a": 1}
TR_checkpoint(%q, {"w": w, "op": op, "ö": ö, "d": d})
`, dataDir, trPath(runDir)))
	if err == nil || strings.Contains(err.Error(), wantHint) {
		t.Errorf("tohum farkı uyumsuzluktur, önceki nokta önerilmemeli: %v", err)
	}
}

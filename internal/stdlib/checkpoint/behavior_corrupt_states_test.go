package checkpoint_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
)

// Kural (dil tanımı 20.7): tür ve parametre sayısı tutan iyileştiricinin bilinmeyen durum
// anahtarı, hiçbir okuyucunun yazmadığı okuyucu türü ve pozitif olmayan toplu/bağlam kullanıcının
// programındaki fark değil kaydın bozukluğudur: önceki geçerli nokta önerilir. (Gemini gerileme
// denetimi, Bulgu 5.)
func TestCorruptOptimizerAndReaderStatesSuggestPrevious(t *testing.T) {
	dataDir := trPath(filepath.Join(reporoot.Root(), "tests", "data", "text"))
	wantHint := messages.Text(messages.H7471, "step_0000000001", 1)

	build := func(t *testing.T) string {
		t.Helper()
		runDir := t.TempDir()
		out, err := run(t, fmt.Sprintf(`
TR_var w = TR_Tensor([1.0, 2.0, 3.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w])
TR_var ö = TR_Dataset(%q).TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1)
TR_var kn = TR_Checkpoint(%q, {"w": w, "op": op, "ö": ö})
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

	cases := []struct {
		name   string
		tamper func(t *testing.T, step2Dir string)
	}{
		{
			name: "iyilestirici_gecersiz_anahtar_adi",
			tamper: func(t *testing.T, step2Dir string) {
				trtPath := filepath.Join(step2Dir, "tensors.trt")
				if _, err := run(t, fmt.Sprintf(`
TR_save_tensors(%q, {
    "w/0": TR_Tensor([1.0, 2.0, 3.0]),
    "op/step": TR_Tensor([0.0, 0.0]),
    "op/bozuk": TR_Tensor([1.0, 2.0, 3.0])
})
`, trPath(trtPath))); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "okuyucu_gecersiz_tur_adi",
			tamper: func(t *testing.T, step2Dir string) {
				durumPath := filepath.Join(step2Dir, "state.json")
				b, err := os.ReadFile(durumPath)
				if err != nil {
					t.Fatal(err)
				}
				corrupted := strings.Replace(string(b), `"sampler"`, `"bozuk_tür"`, 1)
				if err := os.WriteFile(durumPath, []byte(corrupted), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "okuyucu_sifir_toplu_boyutu",
			tamper: func(t *testing.T, step2Dir string) {
				durumPath := filepath.Join(step2Dir, "state.json")
				b, err := os.ReadFile(durumPath)
				if err != nil {
					t.Fatal(err)
				}
				corrupted := strings.Replace(string(b), "\"batch\",\n            2", "\"batch\",\n            0", 1)
				if err := os.WriteFile(durumPath, []byte(corrupted), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runDir := build(t)
			step2Dir := filepath.Join(runDir, "checkpoints", "step_0000000002")
			tc.tamper(t, step2Dir)
			resignCheckpoint(t, step2Dir, 2)

			_, err := run(t, fmt.Sprintf(`
TR_var w = TR_Tensor([1.0, 2.0, 3.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w])
TR_var ö = TR_Dataset(%q).TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1)
TR_Checkpoint(%q, {"w": w, "op": op, "ö": ö})
`, dataDir, trPath(runDir)))
			if err == nil {
				t.Fatal("bozuk kontrol noktası reddedilmeliydi")
			}
			msg := strings.ReplaceAll(err.Error(), trPath(runDir), "<KOŞU>")
			if !strings.Contains(msg, wantHint) {
				t.Errorf("%s: bozuk son noktada önceki geçerli nokta (%s) önerilmeliydi, alınan hata:\n%s", tc.name, wantHint, msg)
			}
		})
	}
}

// Kural (3.7a, 20.2): state.json'daki düz değerde ve sözlük anahtarında eşi olmayan vekil kaçışı
// ya da geçersiz bayt, özeti yeniden yazılmış bir noktada da yüklemede hatadır: H7423'ün içinde
// H2570 ya da H2571, yeri dosyadaki satır ve sütun. Sessizce U+FFFD olmaz; kesilen eğitim aynı
// değerle sürer ya da durur.
func TestTamperedStateTextIsAnError(t *testing.T) {
	cases := []struct {
		name, from, to string
		skip           int // kusurun from'un başından uzaklığı (tırnaktan sonrası)
		code           messages.Code
	}{
		{"degerde_vekil", `"abc"`, `"\ud800"`, 1, messages.H2570},
		{"degerde_gecersiz_bayt", `"abc"`, "\"a\xffb\"", 2, messages.H2571},
		{"anahtarda_vekil", `"kkk"`, `"\udc00"`, 1, messages.H2570},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runDir := t.TempDir()
			program := fmt.Sprintf(`
TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var izlenen = {"ad": "abc", "s": {"kkk": 1}}
TR_var kn = TR_Checkpoint(%q, {"w": w, "izlenen": izlenen})
`, trPath(runDir))
			if out, err := run(t, program+"kn.TR_update(1)\nkn.TR_save()\n"); err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			stepDir := filepath.Join(runDir, "checkpoints", "step_0000000001")
			statePath := filepath.Join(stepDir, "state.json")
			b, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			at := strings.Index(string(b), c.from)
			if at < 0 {
				t.Fatalf("state.json'da %s yok:\n%s", c.from, b)
			}
			tampered := string(b[:at]) + c.to + string(b[at+len(c.from):])
			if err := os.WriteFile(statePath, []byte(tampered), 0o644); err != nil {
				t.Fatal(err)
			}
			resignCheckpoint(t, stepDir, 1)
			_, err = run(t, program)
			inner := findMessage(err, c.code)
			if !messages.Has(err, messages.H7423) || inner == nil {
				t.Fatalf("H7423 içinde %v bekleniyordu: %v", c.code, err)
			}
			flaw := at + c.skip
			wantLine := strings.Count(tampered[:flaw], "\n") + 1
			wantColumn := utf8.RuneCountInString(tampered[strings.LastIndex(tampered[:flaw], "\n")+1:flaw]) + 1
			line, column := inner.Args[len(inner.Args)-2], inner.Args[len(inner.Args)-1]
			if line != wantLine || column != wantColumn {
				t.Errorf("yer satır %v sütun %v; beklenen satır %d sütun %d", line, column, wantLine, wantColumn)
			}
		})
	}
}

// findMessage err'in zincirinde c kodlu iletidir (yoksa nil).
func findMessage(err error, c messages.Code) *messages.Error {
	if m, ok := err.(*messages.Error); ok && m.Code == c {
		return m
	}
	switch u := err.(type) {
	case interface{ Unwrap() error }:
		return findMessage(u.Unwrap(), c)
	case interface{ Unwrap() []error }:
		for _, e := range u.Unwrap() {
			if m := findMessage(e, c); m != nil {
				return m
			}
		}
	}
	return nil
}

// Kural (3.7a, 20.2): taramanın iç içelik sınırı (1000) düz değerin sınırını (100) kesmez. Her
// sözlük düzeyi JSON'da üç kaptır ({"dict": [[k, v]]}); yazıcının kabul ettiği en derin sözlük,
// state.json'un onu saran düzeyleriyle birlikte taramadan geçer ve geri okunur.
func TestDeepPlainValueStillLoads(t *testing.T) {
	nested := func(n int) interp.Value {
		var v interp.Value = interp.NewInt(1)
		for range n {
			d := interp.NewDictVal()
			d.Set("x", v)
			v = d
		}
		return v
	}
	deepest, raw := 0, []byte(nil)
	for n := 1; n <= 200; n++ {
		b, err := checkpoint.WritePlain(nested(n))
		if err != nil {
			break
		}
		deepest, raw = n, b
	}
	if deepest < 100 || deepest == 200 {
		t.Fatalf("yazıcı %d düzeye kadar yazdı; sınır 100 civarında bekleniyordu", deepest)
	}
	t.Logf("yazıcının en derin sözlüğü %d düzey, JSON'u %d bayt", deepest, len(raw))
	state := []byte(`{"objects": [{"name": "d", "kind": "values", "value": ` + string(raw) + `}]}`)
	if err := interp.CheckJSON(state); err != nil {
		t.Fatalf("%d düzeylik düz değer taramada düştü: %v", deepest, err)
	}
	if _, err := checkpoint.ReadPlain(raw); err != nil {
		t.Fatalf("%d düzeylik düz değer geri okunmadı: %v", deepest, err)
	}
}

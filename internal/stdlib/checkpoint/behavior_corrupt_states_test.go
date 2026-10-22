package checkpoint_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (dil tanımı 20.7): tür ve parametre sayısı tutan iyileştiricinin bilinmeyen durum
// anahtarı, hiçbir okuyucunun yazmadığı okuyucu türü ve pozitif olmayan toplu/bağlam kullanıcının
// programındaki fark değil kaydın bozukluğudur: önceki geçerli nokta önerilir. (Gemini gerileme
// denetimi, Bulgu 5.)
func TestCorruptOptimizerAndReaderStatesSuggestPrevious(t *testing.T) {
	dataDir := trPath(filepath.Join(reporoot.Root(), "tests", "data", "text"))
	wantHint := messages.Text(messages.H7471, "adim_0000000001", 1)

	build := func(t *testing.T) string {
		t.Helper()
		runDir := t.TempDir()
		out, err := run(t, fmt.Sprintf(`
değişken w = tensör([1.0, 2.0, 3.0], türev_izle: doğru)
değişken op = AdamW([w])
değişken ö = veri_kümesi(%q).örnekleyici(toplu: 2, bağlam: 8, tohum: 1)
değişken kn = kontrol_noktası(%q, {w: w, op: op, ö: ö})
ö.sonraki()
kn.güncelle(1)
kn.kaydet()
ö.sonraki()
kn.güncelle(2)
kn.kaydet()
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
				trtPath := filepath.Join(step2Dir, "tensorler.trt")
				if _, err := run(t, fmt.Sprintf(`
tensör_kaydet(%q, {
    "w/0": tensör([1.0, 2.0, 3.0]),
    "op/adim": tensör([0.0, 0.0]),
    "op/bozuk": tensör([1.0, 2.0, 3.0])
})
`, trPath(trtPath))); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "okuyucu_gecersiz_tur_adi",
			tamper: func(t *testing.T, step2Dir string) {
				durumPath := filepath.Join(step2Dir, "durum.json")
				b, err := os.ReadFile(durumPath)
				if err != nil {
					t.Fatal(err)
				}
				corrupted := strings.Replace(string(b), `"örnekleyici"`, `"bozuk_tür"`, 1)
				if err := os.WriteFile(durumPath, []byte(corrupted), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "okuyucu_sifir_toplu_boyutu",
			tamper: func(t *testing.T, step2Dir string) {
				durumPath := filepath.Join(step2Dir, "durum.json")
				b, err := os.ReadFile(durumPath)
				if err != nil {
					t.Fatal(err)
				}
				corrupted := strings.Replace(string(b), "\"toplu\",\n            2", "\"toplu\",\n            0", 1)
				if err := os.WriteFile(durumPath, []byte(corrupted), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runDir := build(t)
			step2Dir := filepath.Join(runDir, "kontrol_noktalari", "adim_0000000002")
			tc.tamper(t, step2Dir)
			resignCheckpoint(t, step2Dir, 2)

			_, err := run(t, fmt.Sprintf(`
değişken w = tensör([1.0, 2.0, 3.0], türev_izle: doğru)
değişken op = AdamW([w])
değişken ö = veri_kümesi(%q).örnekleyici(toplu: 2, bağlam: 8, tohum: 1)
kontrol_noktası(%q, {w: w, op: op, ö: ö})
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

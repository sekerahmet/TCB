package checkpoint_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (dil tanımı 20.7): örnekleyici durumundaki pencere, gruptaki pencere sayısını aşamaz;
// aşan konum bu veride geçersizdir (bozuk kayıt), sessizce grup atlanmaz. (Gemini gerileme
// denetimi, Bulgu 6.)
func TestSamplerRejectsWindowBeyondGroup(t *testing.T) {
	dataDir := trPath(filepath.Join(reporoot.Root(), "tests", "data", "text"))
	runDir := t.TempDir()

	out, err := run(t, fmt.Sprintf(`
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken ö = veri_kümesi(%q).örnekleyici(toplu: 2, bağlam: 8, tohum: 1)
değişken kn = kontrol_noktası(%q, {w: w, ö: ö})
ö.sonraki()
kn.güncelle(1)
kn.kaydet()
`, dataDir, trPath(runDir)))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	step1Dir := filepath.Join(runDir, "kontrol_noktalari", "adim_0000000001")
	durumPath := filepath.Join(step1Dir, "durum.json")
	b, err := os.ReadFile(durumPath)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := strings.Replace(string(b), "\"pencere\",\n            2", "\"pencere\",\n            999999", 1)
	if corrupted == string(b) {
		t.Fatalf("pencere alanı durum.json içinde bulunamadı:\n%s", string(b))
	}
	if err := os.WriteFile(durumPath, []byte(corrupted), 0o644); err != nil {
		t.Fatal(err)
	}

	resignCheckpoint(t, step1Dir, 1)

	loadOut, err := run(t, fmt.Sprintf(`
değişken w = tensör([0.0, 0.0], türev_izle: doğru)
değişken ö = veri_kümesi(%q).örnekleyici(toplu: 2, bağlam: 8, tohum: 1)
değişken kn = kontrol_noktası(%q, {w: w, ö: ö})
ö.sonraki()
yazdır("DURUM:", ö.durum)
`, dataDir, trPath(runDir)))
	loadOut = strings.ReplaceAll(loadOut, trPath(runDir), "<KOŞU>")
	if err == nil {
		t.Fatalf("veri kümesi sınırını aşan pencere konumu (999999) reddedilmeliydi; hatasız yüklendi ve grubu atladı:\n%s", loadOut)
	}
}

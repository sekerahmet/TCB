package testkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Yazım dilinden bağımsızlık testlerinin ortak iskeleti (Parti 9 D3): bir çıktı (dizin, grafik) TR_DIL=tr ve TR_DIL=en
// ile ayrı alt süreçlerde kurulur, iki sürecin olguları karşılaştırılır. Alt süreç gerekir, çünkü etkin dil süreç
// başlarken bir kez seçilir (codes.LanguageEnvVar). codeindex ve tools/codegraph aynı iskeleti kullanır.

// RunWithLanguageFile bu test ikilisini yalnızca helperTest testini çalıştıracak biçimde TR_DIL=lang ortamında yeniden
// başlatır; çıktı dosyasının yolunu envKey ortam değişkeniyle verir ve alt sürecin o dosyaya yazdığını döner.
// helperTest, envKey boşsa kendini atlayan (t.Skip) bir "test"tir.
func RunWithLanguageFile(t testing.TB, helperTest, envKey, lang string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out_"+lang)
	cmd := exec.Command(os.Args[0], "-test.run=^"+helperTest+"$")
	cmd.Env = append(os.Environ(), "TR_DIL="+lang, envKey+"="+out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("TR_DIL=%s: %v\n%s", lang, err, b)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("TR_DIL=%s: the helper wrote no output: %v", lang, err)
	}
	return data
}

// AssertSameFactSet iki dilde kurulan çıktının olgularını (sıralı metin satırları) karşılaştırır: tr'de minFacts'tan az olgu
// varsa karşılaştırma anlamsızdır (fikstür bozulmuştur), test durur; olgular farklıysa yalnızca tr'de olanlar yazılır.
// label çıktının adıdır ("the index", "the .tr graph").
func AssertSameFactSet(t testing.TB, label string, minFacts int, tr, en []string) {
	t.Helper()
	if len(tr) < minFacts {
		t.Fatalf("too few facts in %s to compare: %d (want at least %d)", label, len(tr), minFacts)
	}
	if strings.Join(tr, "\n") == strings.Join(en, "\n") {
		return
	}
	have := map[string]bool{}
	for _, f := range en {
		have[f] = true
	}
	var missing []string
	for _, f := range tr {
		if !have[f] {
			missing = append(missing, f)
		}
	}
	t.Errorf("TR_DIL=en changes %s (%d facts vs %d); only in tr:\n%s", label, len(tr), len(en), strings.Join(missing, "\n"))
}

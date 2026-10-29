package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// speedFiles hız tabanının metinleridir: büyük, orta ve küçük bir dosya
// (internal/lang/source'taki BenchmarkPrepare de aynılarını kullanır).
var speedFiles = []string{"lib/layers.tr", "examples/tr/35_transformer.tr", "examples/tr/08_fonksiyonlar.tr"}

// BenchmarkCheckAndRun aynı dosyanın kodsuz ve kodlu kopyasında 'tr kontrol' ile çalıştırmanın
// süreç başından sonuna süresini ölçer (derleme ve süreç açılışı dahil). Madde 15'in tabanıdır,
// ad kuralından ve görünümün kaldırılmasından sonra aynen koşar:
// go test -run XXX -bench CheckAndRun -count 5 ./cmd/tr/e2e/
func BenchmarkCheckAndRun(b *testing.B) {
	dir := b.TempDir()
	for _, file := range speedFiles {
		data, err := os.ReadFile(reporoot.Path(file))
		if err != nil {
			b.Fatal(err)
		}
		coded, err := source.Encode(string(data))
		if err != nil {
			b.Fatal(err)
		}
		name := filepath.Base(file)
		for _, form := range []struct{ name, text string }{{"plain", string(data)}, {"coded", coded}} {
			copyName := form.name + "_" + name
			testkit.WriteFile(b, dir, copyName, form.text)
			for _, command := range [][]string{{"kontrol", copyName}, {copyName}} {
				label := "run"
				if len(command) == 2 {
					label = "check"
				}
				b.Run(name+"/"+form.name+"/"+label, func(b *testing.B) {
					for b.Loop() {
						if s := testkit.RunBinary(b, trBinary, dir, "", nil, command...); s.Code != 0 {
							b.Fatalf("%v: %d\n%s%s", command, s.Code, s.Stdout, s.Stderr)
						}
					}
				})
			}
		}
	}
}

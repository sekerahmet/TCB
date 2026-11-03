package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// speedFiles hız tabanının metinleridir: büyük, orta ve küçük bir dosya
// (internal/lang/source'taki BenchmarkPrepare de aynılarını kullanır).
var speedFiles = []string{"lib/layers.tr", "examples/tr/35_transformer.tr", "examples/tr/08_fonksiyonlar.tr"}

// BenchmarkCheckAndRun kodlu dosyada 'tr check' ile çalıştırmanın süreç başından sonuna süresini
// ölçer (derleme ve süreç açılışı dahil). Madde 15'in tabanıdır:
// go test -run XXX -bench CheckAndRun -count 5 ./cmd/tr/e2e/
func BenchmarkCheckAndRun(b *testing.B) {
	dir := b.TempDir()
	for _, file := range speedFiles {
		data, err := os.ReadFile(reporoot.Path(file))
		if err != nil {
			b.Fatal(err)
		}
		name := filepath.Base(file)
		testkit.WriteFile(b, dir, name, string(data))
		for _, command := range [][]string{{"check", name}, {name}} {
			label := "run"
			if len(command) == 2 {
				label = "check"
			}
			b.Run(name+"/"+label, func(b *testing.B) {
				for b.Loop() {
					if s := testkit.RunBinary(b, trBinary, dir, "", nil, command...); s.Code != 0 {
						b.Fatalf("%v: %d\n%s%s", command, s.Code, s.Stdout, s.Stderr)
					}
				}
			})
		}
	}
}

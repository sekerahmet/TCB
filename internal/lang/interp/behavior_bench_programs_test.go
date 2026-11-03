package interp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Hız kıyas setinin (tools/bench/tr) programları, trlang run'ın yolunda: program source.Prepare ile
// denetlenip hazırlanır, hata ayıklayıcısız VM.Run ile koşar. trlang profile bu yolu ölçmez: örnekleyici
// hata ayıklayıcı yuvasına takılır, her deyimde kanca çalışır ve ortam havuzu kapanır.
//
//	go test -run XXX -bench Programs/fib -benchtime 300x -count 5 -cpuprofile fib.prof -benchmem ./internal/lang/interp/
//	go tool pprof -top -diff_base once.prof interp.test sonra.prof
//
// Boylar küçültülmüştür (yerelde bir koşu milisaniyeler sürsün); oranlar büyük boydakinden farklı
// olabilir. Süre kararını Colab'daki tools/bench kıyası verir.
var benchPrograms = []struct {
	name  string
	sizes []string // eski, yeni çiftleri; eski metnin her geçtiği yer değişir
}{
	{"dongu", []string{"3000000", "30000"}},
	{"fib", []string{"fib(27)", "fib(20)"}},
	{"sirala", []string{"300000", "30000", "150000", "15000", "299999", "29999"}},
	{"metin", []string{"200000", "20000"}},
	{"sozluk", []string{"i < 200000", "i < 20000"}},
	{"matris_dongu", []string{"n = 120", "n = 30"}},
}

// aralik kıyas setinde olmayan aralıklı döngüdür (TR_for i TR_in): sayacın ayırması (compiler_loop.go).
const rangeLoopProgram = "# TR 4\nTR_var t = 0\nTR_for i TR_in 0..20000 {\n    t += i\n}\n"

func BenchmarkPrograms(b *testing.B) {
	for _, p := range benchPrograms {
		text, err := os.ReadFile(filepath.Join(reporoot.Root(), "tools", "bench", "tr", p.name+".tr"))
		if err != nil {
			b.Fatal(err)
		}
		code := string(text)
		for i := 0; i < len(p.sizes); i += 2 {
			if !strings.Contains(code, p.sizes[i]) {
				b.Fatalf("%s: %q programda yok; boyu küçültme çiftini güncelleyin", p.name, p.sizes[i])
			}
			code = strings.ReplaceAll(code, p.sizes[i], p.sizes[i+1])
		}
		b.Run(p.name, func(b *testing.B) { benchProgram(b, code) })
	}
	b.Run("aralik", func(b *testing.B) { benchProgram(b, rangeLoopProgram) })
}

func benchProgram(b *testing.B, code string) {
	s := source.Prepare(code, source.Options{TypeCheck: true})
	if len(s.Errors) > 0 {
		b.Fatal(s.Errors)
	}
	b.ReportAllocs()
	for b.Loop() {
		machine := New()
		var out bytes.Buffer
		machine.Out = &out
		if _, err := machine.Run(s.Program); err != nil {
			b.Fatal(err)
		}
		machine.WaitAsync()
	}
}

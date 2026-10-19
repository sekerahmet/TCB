package tokenize

import (
	"encoding/json"
	"errors"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

// kucukYigin sınama süresince goroutine yığınını küçük bir sınıra indirir:
// girdi uzunluğuyla büyüyen bir özyineleme, süreci (kurtarılamaz) yığın
// taşmasıyla hemen çökertir.
func smallStack(t *testing.T) {
	prevVal := debug.SetMaxStack(16 << 20)
	t.Cleanup(func() { debug.SetMaxStack(prevVal) })
}

// Grup yinelemesi (tek karakterlik olmayan alt desen) girdi uzunluğundan
// bağımsız yığın kullanmalı; eskiden karakter başına ~1,3 KB yığın harcanıyor,
// birkaç MB'lık girdi süreci "stack overflow" ile çökertiyordu.
func TestGroupRepeatNoStackOverflow(t *testing.T) {
	smallStack(t)
	s := strings.Repeat("ab", 1<<20) // 2 MB
	cases := []struct {
		pattern     string
		begin, last int
	}{
		{`(?:ab)+`, 0, len(s)},
		{`(?:ab)+?$`, 0, len(s)},
		{`(?:a(?:b))*`, 0, len(s)},
		{`(?:ab|a)+b`, 0, len(s)},
		{`(?:ab){2,}`, 0, len(s)},
		{`(?:(?:ab)+)?x|(?:ab)+`, 0, len(s)},
	}
	for _, d := range cases {
		expr, err := CompileRegex(d.pattern)
		if err != nil {
			t.Fatalf("%q: %v", d.pattern, err)
		}
		b, e, ok, err := expr.Find(s, 0)
		if err != nil || !ok || b != d.begin || e != d.last {
			t.Errorf("%q: (%d, %d, %v, %v), beklenen (%d, %d)", d.pattern, b, e, ok, err, d.begin, d.last)
		}
	}
}

// Geri izleme noktalarının belleği sınırlıdır; aşılırsa yakalanabilir hata.
func TestBacktrackMemoryLimit(t *testing.T) {
	setAndRestore(t, RegexSettings{BacktrackLimit: 1000})
	expr, err := CompileRegex(`(?:ab)+`)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = expr.Find(strings.Repeat("ab", 10_000), 0)
	var h *StepLimitError
	if !errors.As(err, &h) || !h.Memory || !strings.Contains(err.Error(), "geri izleme belleği") {
		t.Fatalf("geri izleme belleği hatası bekleniyordu: %v", err)
	}
	// Sınırın altında kalan girdi sorunsuz eşleşir.
	if _, e, ok, err := expr.Find(strings.Repeat("ab", 100), 0); err != nil || !ok || e != 200 {
		t.Fatalf("kısa girdi: %d %v %v", e, ok, err)
	}
}

// Çok derin iç içe desenler derlemede açık hatayla reddedilir (eskiden
// ayrıştırıcının özyinelemesi yığını taşırıyordu).
func TestDeepPatternRejected(t *testing.T) {
	smallStack(t)
	n := 3_000_000
	worst := map[string]string{
		"grup":       strings.Repeat("(?:", n) + "a" + strings.Repeat(")", n),
		"bakma":      strings.Repeat("(?=", n) + "a" + strings.Repeat(")", n),
		"sınıf":      strings.Repeat("[", n) + "a" + strings.Repeat("]", n),
		"niceleyici": "a" + strings.Repeat("{1}", n),
	}
	for ad, d := range worst {
		_, err := CompileRegex(d)
		if err == nil || !strings.Contains(err.Error(), "sınırı") {
			t.Errorf("%s: derinlik hatası bekleniyordu: %v", ad, err)
		}
		if err != nil && len(err.Error()) > 1000 {
			t.Errorf("%s: hata iletisi deseni kısaltmalı (%d bayt)", ad, len(err.Error()))
		}
	}
	// Sınırın altındaki derinlik derlenir ve doğru eşleşir.
	deep := strings.Repeat("(?:", reMaxDepth-1) + "a" + strings.Repeat(")", reMaxDepth-1)
	expr, err := CompileRegex(deep)
	if err != nil {
		t.Fatal(err)
	}
	if b, e, ok, err := expr.Find("xa", 0); err != nil || !ok || b != 1 || e != 2 {
		t.Fatalf("derin desen: %d %d %v %v", b, e, ok, err)
	}
}

// Parçalayıcı düzeyinde: Split deseninde grup yinelemesi olan bir dosya,
// uzun girdide süreci çökertmeden kodlar.
func TestTokenizerGroupRepeat(t *testing.T) {
	smallStack(t)
	p := patternTokenizer(t, `(?:ab)+|\s+|.`)
	text := strings.Repeat("ab", 1<<20)
	ids, err := p.SafeEncode(text, Options{})
	if err != nil || len(ids) == 0 {
		t.Fatalf("GuvenliKodla: %d kimlik, %v", len(ids), err)
	}
}

// desenliParcalayici küçük sınama parçalayıcısının ön bölücüsünü verilen
// Split deseniyle (önce boşluklarla bölüp) değiştirir.
func patternTokenizer(t *testing.T, pattern string) *Tokenizer {
	t.Helper()
	data, err := os.ReadFile(smallPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["pre_tokenizer"] = map[string]any{"type": "Sequence", "pretokenizers": []any{
		map[string]any{"type": "WhitespaceSplit"},
		map[string]any{"type": "Split", "pattern": map[string]any{"Regex": pattern}, "behavior": "Isolated", "invert": false},
	}}
	fresh, _ := json.Marshal(raw)
	p, err := Parse(fresh)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Adım bütçesi aramaların toplamına uygulanır: her araması kendi sınırında
// kalan ama toplamda karesel büyüyen bir desen açık hatayla durur (eskiden
// 40 bin karakterde ~11 sn hatasız sürüyordu, 1 MB'ta saatler).
func TestQuadraticGrowthStopped(t *testing.T) {
	expr, err := CompileRegex(`.(?=.*)`)
	if err != nil {
		t.Fatal(err)
	}
	// Kısa girdide sorun yok.
	countFn := 0
	if err := expr.FindAllErr(strings.Repeat("x", 100), func(int, int) { countFn++ }); err != nil || countFn != 100 {
		t.Fatalf("kısa girdi: %d eşleşme, %v", countFn, err)
	}
	begin := time.Now()
	err = expr.FindAllErr(strings.Repeat("x", 40_000), func(int, int) {})
	var h *StepLimitError
	if !errors.As(err, &h) {
		t.Fatalf("adım sınırı hatası bekleniyordu: %v", err)
	}
	if duration := time.Since(begin); duration > 3*time.Second {
		t.Errorf("sınır geç devreye girdi: %v", duration)
	}
}

// Bir kodlama çağrısının bütün parçaları tek bütçeyi paylaşır: ön bölücünün
// ürettiği her kısa parçada sınırın altında kalan üstel arama, toplamda
// durdurulur.
func TestEncodeBudgetShared(t *testing.T) {
	setAndRestore(t, RegexSettings{AllowRiskyPatterns: true})
	p := patternTokenizer(t, `(a|a)*b|\S`)
	piece := strings.Repeat("a", 14)
	// Tek parça kendi başına bütçeyi aşmaz.
	if _, err := p.SafeEncode(piece, Options{}); err != nil {
		t.Fatalf("tek parça: %v", err)
	}
	text := strings.Repeat(piece+" ", 300)
	begin := time.Now()
	_, err := p.SafeEncode(text, Options{})
	var h *StepLimitError
	if !errors.As(err, &h) {
		t.Fatalf("paylaşılan bütçe aşılmalıydı: %v", err)
	}
	if duration := time.Since(begin); duration > 3*time.Second {
		t.Errorf("sınır geç devreye girdi: %v", duration)
	}
}

// Rastgele desen ve girdilerde motor asla çökmemeli: ya derleme hatası, ya
// sonuç, ya da *AdimSiniriHatasi.
func FuzzRegexNoCrash(f *testing.F) {
	for _, d := range []string{`(?:ab)+`, `(a|a)*b`, `\s+(?!\S)`, `(?<=a{1,3})b`, `(?>a+)b`, `[[:alpha:]]+`, `(?i:ÇAY)`, `\p{Han}`} {
		f.Add(d, "ab abab aaab ÇaY 中文")
	}
	f.Fuzz(func(t *testing.T, pattern, input string) {
		if len(pattern) > 200 || len(input) > 2000 {
			return
		}
		expr, err := CompileRegex(pattern)
		if err != nil {
			return
		}
		prev := -1
		err = expr.FindAllErr(input, func(b, e int) {
			if b < prev || e < b || e > len(input) {
				t.Fatalf("%q: geçersiz eşleşme (%d, %d)", pattern, b, e)
			}
			prev = e
		})
		var h *StepLimitError
		if err != nil && !errors.As(err, &h) {
			t.Fatalf("%q: beklenmeyen hata %v", pattern, err)
		}
	})
}

// Düzenli ifade motorunun ham hızı (bayt düzeyi ve Llama 3 tarzı desenler).
func BenchmarkRegex(b *testing.B) {
	text := benchText(b)
	for _, d := range []struct{ ad, pattern string }{
		{"bayt", ByteLevelPattern}, {"llama3", llama3Pattern}, {"grup", `(?:\p{L}\p{L})+|\s+|.`},
	} {
		expr, err := CompileRegex(d.pattern)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(d.ad, func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			for i := 0; i < b.N; i++ {
				expr.FindAll(text, func(int, int) {})
			}
		})
	}
}

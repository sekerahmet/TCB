package tokenize

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Gerçek parçalayıcılarda görülen desenler.
const (
	llama3Pattern = `(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}{1,3}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`
	qwenPattern   = `(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`
)

// setAndRestore sınama süresince motor ayarlarını değiştirir.
func setAndRestore(t *testing.T, a RegexSettings) {
	prevVal := CurrentRegexSettings()
	SetRegexSettings(a)
	t.Cleanup(func() { SetRegexSettings(prevVal) })
}

// riskRejected err'in, pattern için iç içe niceleyici nedeniyle reddedilmiş olduğunu söyler.
func riskRejected(err error, pattern string) bool {
	m, ok := messages.As(err)
	return ok && m.Code == messages.H5248 && m.Args[0] == pattern && m.Args[1] == messages.Text(messages.H5288)
}

func TestRiskyPatternsRejected(t *testing.T) {
	risky := []string{`(a+)+$`, `(a*)*b`, `(\w+\s*)+$`, `(a+|b)+c`, `((ab)*)*x`, `(?:x?y*)+z`, `(a{1,})+`, `(x+x+)+y`, `(\d+\.?\d*)+$`}
	for _, d := range risky {
		_, err := CompileRegex(d)
		if !riskRejected(err, d) {
			t.Errorf("%q reddedilmeliydi, hata: %v", d, err)
		}
	}
	safe := []string{SplitPattern, ByteLevelPattern, llama3Pattern, qwenPattern, `(ab+)+`, `(?>a+)+`, `(a++)+`,
		`\d+(?=\D)`, `(?:\s*\S+)`, `[a-z]+(?:-[a-z]+)*`}
	for _, d := range safe {
		expr, err := CompileRegex(d)
		if err != nil {
			t.Errorf("%q kabul edilmeliydi: %v", d, err)
			continue
		}
		if u := expr.Warnings(); len(u) > 0 {
			t.Errorf("%q için beklenmeyen uyarı: %v", d, u)
		}
	}
	// İzin verilince derlenir ama risk uyarı olarak kalır.
	setAndRestore(t, RegexSettings{AllowRiskyPatterns: true})
	expr, err := CompileRegex(`(a+)+$`)
	if err != nil || len(expr.Warnings()) == 0 {
		t.Fatalf("izinli derleme: %v, uyarılar %v", err, expr.Warnings())
	}
}

func TestStepLimitTriggers(t *testing.T) {
	setAndRestore(t, RegexSettings{AllowRiskyPatterns: true})
	cases := []struct{ pattern, input string }{
		{`(a+)+$`, strings.Repeat("a", 40) + "!"},
		{`(a|a)*b`, strings.Repeat("a", 40)},
		{`(\w+\s?)+$`, strings.Repeat("kelime ", 15) + "!"},
		{`(x+x+)+y`, strings.Repeat("x", 50)},
	}
	for _, d := range cases {
		expr, err := CompileRegex(d.pattern)
		if err != nil {
			t.Fatal(err)
		}
		begin := time.Now()
		err = expr.FindAllErr(d.input, func(int, int) {})
		duration := time.Since(begin)
		var h *StepLimitError
		if !errors.As(err, &h) {
			t.Errorf("%q: adım sınırı hatası bekleniyordu, alınan %v", d.pattern, err)
			continue
		}
		if h.Pattern != d.pattern || err.Error() != messages.Text(messages.H5270, reShorten(d.pattern), h.Pos, h.Budget) {
			t.Errorf("%q: hata iletisi deseni/konumu içermiyor: %v", d.pattern, err)
		}
		if duration > 5*time.Second {
			t.Errorf("%q: sınır çok geç devreye girdi (%v)", d.pattern, duration)
		}
		t.Logf("%q: %v sonra durdu: %v", d.pattern, duration, err)
	}
	// Bul da hata döndürür.
	expr, _ := CompileRegex(`(a|a)*b`)
	if _, _, _, err := expr.Find(strings.Repeat("a", 40), 0); err == nil {
		t.Error("Bul adım sınırı hatası döndürmeliydi")
	}
}

// Parçalayıcı düzeyinde: riskli desenli bir dosya uyarı verir, Guvenli...
// işlevleri hata döndürür, hata döndürmeyen işlevler panikler (paralel
// işçilerdeki panik de çağırana taşınır, süreç çökmez).
func TestTokenizerStepLimit(t *testing.T) {
	data, err := os.ReadFile(smallPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	pt := raw["pre_tokenizer"].(map[string]any)["pretokenizers"].([]any)
	pt[0].(map[string]any)["pattern"] = map[string]any{"Regex": `(a|a)*b|\s+|\S`}
	fresh, _ := json.Marshal(raw)
	p, err := Parse(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings()) == 0 {
		t.Error("çakışan seçenek uyarısı bekleniyordu")
	}
	worst := strings.Repeat("a", 40)
	if _, err := p.SafeEncode("merhaba dünya", Options{}); err != nil {
		t.Fatalf("zararsız girdi hata verdi: %v", err)
	}
	if ids, err := p.SafeEncode(worst, Options{}); err == nil || ids != nil {
		t.Fatalf("GuvenliKodla hata döndürmeliydi: %v %v", ids, err)
	}
	_, errs := p.SafeEncodeBatch([]string{"iyi", worst, "yine iyi"}, Options{}, 2)
	if errs == nil || errs[1] == nil || errs[0] != nil || errs[2] != nil {
		t.Fatalf("GuvenliTopluKodla hataları: %v", errs)
	}
	expectPanic := func(ad string, f func()) {
		defer func() {
			if _, ok := recover().(*StepLimitError); !ok {
				t.Errorf("%s: *AdimSiniriHatasi paniği bekleniyordu", ad)
			}
		}()
		f()
	}
	expectPanic("Kodla", func() { p.Encode(worst) })
	expectPanic("TopluKodla", func() { p.EncodeBatch([]string{"x", worst, "y", "z"}, Options{}, 3) })
	// Riskli (iç içe) desenli dosya varsayılan ayarla yüklenmez.
	pt[0].(map[string]any)["pattern"] = map[string]any{"Regex": `(\w+\s*)+$`}
	fresh, _ = json.Marshal(raw)
	if _, err := Parse(fresh); !messages.Has(err, messages.H5248) {
		t.Fatalf("riskli desenli dosya reddedilmeliydi: %v", err)
	}
}

// Gerçek parçalayıcı desenleri bütçeye takılmamalı ve adım sayısı girdi
// uzunluğuyla doğrusal kalmalı (zararlı biçimde seçilmiş girdiler dahil).
func TestKnownPatternsLinear(t *testing.T) {
	sample, err := os.ReadFile("testdata/ornek_metin.txt")
	if err != nil {
		t.Fatal(err)
	}
	genInput := func(n int) map[string]string {
		repeat := func(s string) string { return strings.Repeat(s, n/len(s)+1)[:n] }
		return map[string]string{
			"metin":        repeat(string(sample)),
			"bosluk+x":     strings.Repeat(" ", n) + "x",
			"bosluk":       strings.Repeat(" ", n),
			"satir-bosluk": repeat(" \n\t\r "),
			"noktalama":    repeat("!?.,;"),
			"harf":         strings.Repeat("ğ", n/2),
			"rakam":        repeat("0123456789"),
			"harf-bosluk":  repeat("a "),
			"kesme":        repeat("'s'S'x'"),
			"karisik":      repeat("a1 !\n ş’e"),
		}
	}
	maxN := 0.0
	defer func() { t.Logf("en yüksek: %.1f adım/bayt", maxN) }()
	patterns := map[string]string{"bolme": SplitPattern, "bayt": ByteLevelPattern, "llama3": llama3Pattern, "qwen": qwenPattern}
	for dad, d := range patterns {
		expr, err := CompileRegex(d)
		if err != nil {
			t.Fatal(err)
		}
		lower, upper := genInput(20_000), genInput(80_000)
		for gad := range lower {
			var a1, a2 int64
			if err := catchStepError(func() {
				a1 = expr.allMatches(lower[gad], func(int, int) {}, nil)
				a2 = expr.allMatches(upper[gad], func(int, int) {}, nil)
			}); err != nil {
				t.Errorf("%s/%s: bütçeye takıldı: %v", dad, gad, err)
				continue
			}
			byteVal := float64(len(upper[gad]))
			if v := float64(a2) / byteVal; v > maxN {
				maxN = v
			}
			ratio := float64(a2) / float64(a1) * float64(len(lower[gad])) / byteVal
			if float64(a2)/byteVal > 40 || ratio > 1.5 {
				t.Errorf("%s/%s: doğrusal değil: %d bayt -> %d adım (%.1f adım/bayt, büyüme oranı %.2f)",
					dad, gad, len(upper[gad]), a2, float64(a2)/byteVal, ratio)
			}
		}
	}
}

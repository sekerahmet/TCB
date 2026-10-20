package tokenize

import (
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Düzenli ifade motorunun güvenlik sınırları
// ============================================================================
//
// Geri izlemeli bir motor, iç içe niceleyicili ya da çakışan seçenekli bir
// desende kötü niyetli bir girdiyle üstel zamana düşebilir (ReDoS). Desenler
// parçalayıcı dosyasından geldiği için (yarı güvenilir) iki önlem alınır:
//
//  1. Adım bütçesi: bir çağrının bütün aramaları (bir metnin kodlanmasında
//     normalleştirici ve ön bölücülerdeki bütün desenler) toplam
//     AdimTabani + AdimCarpani·(işlenen bayt) adımı paylaşır. Böylece her
//     araması kendi bütçesinde kalan ama toplamda karesel büyüyen desenler
//     de durdurulur. Aşılırsa AdimSiniriHatasi oluşur; sessizce yanlış bölme
//     yapılmaz. Geri izleme noktalarının belleği de GeriIzlemeSiniri ile
//     sınırlıdır.
//  2. Derleme anında risk çözümlemesi: iç içe sınırsız niceleyiciler (ör.
//     "(a+)+", "(\w+\s*)*") varsayılan olarak reddedilir; sınırsız bir
//     yinelemenin içinde aynı karakterle başlayan seçenekler (ör. "(a|ab)*")
//     uyarı olarak kaydedilir.

// RegexSettings motorun güvenlik ayarlarıdır.
type RegexSettings struct {
	// StepBase ve AdimCarpani bir çağrının (ör. bir metnin kodlanması)
	// bütün aramalarının paylaştığı adım bütçesini belirler:
	// AdimTabani + AdimCarpani * (işlenen bayt sayısı).
	StepBase   int64
	StepFactor int64
	// AllowRiskyPatterns doğruysa iç içe sınırsız niceleyicili desenler
	// reddedilmez (adım bütçesi yine geçerlidir).
	AllowRiskyPatterns bool
	// BacktrackLimit bir aramanın aynı anda tutabileceği en çok geri
	// izleme noktası sayısıdır (bellek sınırı; kayıt başına ~24 bayt).
	BacktrackLimit int
}

// DefaultRegexSettings varsayılan güvenlik ayarlarıdır.
var DefaultRegexSettings = RegexSettings{StepBase: 1_000_000, StepFactor: 100, BacktrackLimit: 1 << 22}

var regexSetting atomic.Pointer[RegexSettings]

func init() {
	a := DefaultRegexSettings
	regexSetting.Store(&a)
}

// SetRegexSettings motorun güvenlik ayarlarını değiştirir (bütün desenler için,
// eşzamanlı kullanımda güvenlidir). Sıfır ya da negatif bütçe değerleri
// varsayılana çevrilir.
func SetRegexSettings(a RegexSettings) {
	if a.StepBase <= 0 {
		a.StepBase = DefaultRegexSettings.StepBase
	}
	if a.StepFactor <= 0 {
		a.StepFactor = DefaultRegexSettings.StepFactor
	}
	if a.BacktrackLimit <= 0 {
		a.BacktrackLimit = DefaultRegexSettings.BacktrackLimit
	}
	regexSetting.Store(&a)
}

// CurrentRegexSettings geçerli güvenlik ayarlarını verir.
func CurrentRegexSettings() RegexSettings { return *regexSetting.Load() }

// StepLimitError bir çağrının düzenli ifade adım bütçesini ya da geri
// izleme belleği sınırını aştığını bildirir.
type StepLimitError struct {
	Pattern string // desenin kaynağı (200 bayttan uzunsa kısaltılmış)
	Pos     int    // sınırın aşıldığı aramanın başladığı bayt konumu
	Budget  int64  // aşılan adım bütçesi
	Memory  bool   // aşılan, geri izleme belleği sınırıysa (GeriIzlemeSiniri) doğru
}

func (e *StepLimitError) Error() string {
	if e.Memory {
		return messages.Text(messages.H5269, reShorten(e.Pattern), e.Pos, CurrentRegexSettings().BacktrackLimit)
	}
	return messages.Text(messages.H5270, reShorten(e.Pattern), e.Pos, e.Budget)
}

// catchStepError f'yi çalıştırır; adım sınırı aşılırsa hatayı döndürür
// (diğer panikler olduğu gibi sürer).
func catchStepError(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if h, ok := r.(*StepLimitError); ok {
				err = h
				return
			}
			panic(r)
		}
	}()
	f()
	return nil
}

// ---------------------------------------------------------------------------
// Derleme anında risk çözümlemesi
// ---------------------------------------------------------------------------

// reSaturation uzunluk hesaplarının doyduğu (taşmadığı) üst sınırdır.
const reSaturation = 1 << 40

func reSatMul(a, b int) int {
	if a <= 0 || b <= 0 {
		return 0
	}
	if a > reSaturation/b {
		return reSaturation
	}
	return a * b
}

func reSatSum(a, b int) int {
	if a+b > reSaturation {
		return reSaturation
	}
	return a + b
}

// sinirsiz niceleyicinin "sınırsıza yakın" sayıldığı üst sınır.
const riskyUpperLimit = 100

func reUnbounded(t *reRepeat) bool {
	return (t.parent < 0 || t.parent >= riskyUpperLimit) && !t.claimer
}

// reMinLen düğümün eşleyebileceği en az karakter sayısı.
func reMinLen(d reNode) int {
	switch x := d.(type) {
	case *reChar, *reClass, *reDot:
		return 1
	case *reUnion:
		t := 0
		for _, o := range x.items {
			t = reSatSum(t, reMinLen(o))
		}
		return t
	case *reOption:
		m := -1
		for _, o := range x.options {
			if n := reMinLen(o); m < 0 || n < m {
				m = n
			}
		}
		if m < 0 {
			return 0
		}
		return m
	case *reRepeat:
		return reSatMul(x.en, reMinLen(x.sub))
	case *reAtomic:
		return reMinLen(x.sub)
	}
	return 0
}

// reFreeRepeat: düğümün bir yinelemesi, kendisi de sınırsız olan bir iç
// niceleyiciyle (geri kalanı boş eşleşebilecek biçimde) tamamen
// karşılanabiliyorsa doğru. Bu durumda dış yineleme, girdiyi üstel sayıda
// farklı biçimde bölebilir.
func reFreeRepeat(d reNode) bool {
	switch x := d.(type) {
	case *reRepeat:
		return reUnbounded(x)
	case *reUnion:
		for i, o := range x.items {
			if !reFreeRepeat(o) {
				continue
			}
			empty := true
			for j, p := range x.items {
				if j != i && reMinLen(p) > 0 {
					empty = false
					break
				}
			}
			if empty {
				return true
			}
		}
		// Arka arkaya (araları boş eşleşebilen) iki sınırsız niceleyici aynı
		// karakterleri paylaşıyorsa (ör. "x+x+") girdiyi bölmenin yolu çoktur.
		for i, o := range x.items {
			ti, ok := o.(*reRepeat)
			if !ok || !reUnbounded(ti) {
				continue
			}
			for j := i + 1; j < len(x.items); j++ {
				if tj, ok := x.items[j].(*reRepeat); ok && reUnbounded(tj) && reOverlaps(ti.sub, tj.sub) {
					return true
				}
				if reMinLen(x.items[j]) > 0 {
					break
				}
			}
		}
	case *reOption:
		for _, o := range x.options {
			if reFreeRepeat(o) {
				return true
			}
		}
	}
	return false
}

// reFirstChars düğümün ilk karakteri için olası düz karakterleri ve
// sınıf koşullarını verir (yaklaşık; çakışma uyarısı için).
func reFirstChars(d reNode) ([]rune, []func(rune) bool) {
	switch x := d.(type) {
	case *reChar:
		return append([]rune{x.r}, x.fold...), nil
	case *reClass:
		return nil, []func(rune) bool{x.f}
	case *reDot:
		return nil, []func(rune) bool{func(r rune) bool { return r != '\n' || x.includeNewline }}
	case *reUnion:
		var rs []rune
		var fs []func(rune) bool
		for _, o := range x.items {
			r, f := reFirstChars(o)
			rs, fs = append(rs, r...), append(fs, f...)
			if reMinLen(o) > 0 {
				break
			}
		}
		return rs, fs
	case *reOption:
		var rs []rune
		var fs []func(rune) bool
		for _, o := range x.options {
			r, f := reFirstChars(o)
			rs, fs = append(rs, r...), append(fs, f...)
		}
		return rs, fs
	case *reRepeat:
		return reFirstChars(x.sub)
	case *reAtomic:
		return reFirstChars(x.sub)
	}
	return nil, nil
}

func reOverlaps(a, b reNode) bool {
	ra, fa := reFirstChars(a)
	rb, fb := reFirstChars(b)
	for _, x := range ra {
		for _, y := range rb {
			if x == y {
				return true
			}
		}
		for _, f := range fb {
			if f(x) {
				return true
			}
		}
	}
	for _, y := range rb {
		for _, f := range fa {
			if f(y) {
				return true
			}
		}
	}
	return false
}

// reRiskAnalyze desen ağacını gezer; reddedilecek yapı için hata iletisi,
// ayrıca uyarılar döndürür.
func reRiskAnalyze(d reNode) (failure string, warnings []string) {
	var walk func(d reNode)
	walk = func(d reNode) {
		switch x := d.(type) {
		case *reUnion:
			for _, o := range x.items {
				walk(o)
			}
		case *reOption:
			for _, o := range x.options {
				walk(o)
			}
		case *reAtomic:
			walk(x.sub)
		case *reLook:
			walk(x.sub)
		case *reRepeat:
			if reUnbounded(x) {
				if failure == "" && reFreeRepeat(x.sub) {
					failure = messages.Text(messages.H5288)
				}
				if s, ok := x.sub.(*reOption); ok {
					for i := 0; i < len(s.options); i++ {
						for j := i + 1; j < len(s.options); j++ {
							if reOverlaps(s.options[i], s.options[j]) {
								warnings = append(warnings, messages.Text(messages.H5271, i+1, j+1))
							}
						}
					}
				}
			}
			walk(x.sub)
		}
	}
	walk(d)
	return failure, warnings
}

// Warnings desenin derlenirken bulunan risk uyarılarını verir.
func (d *Regex) Warnings() []string { return append([]string(nil), d.warnings...) }

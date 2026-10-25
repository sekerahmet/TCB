package profiler

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// Raporların ortak yazım yardımcıları (Türkçe sayı yazımı: ondalık virgül, binlik boşluk).

// percent payı yüzde olarak kişinin dilinde yazar (H8293; işaretin yeri ve ondalık ayırıcı
// dilindir): tr %41, %3,5; en 41%, 3.5%.
func percent(f float64) string { return messages.Text(messages.H8293, percentNumber(f)) }

// percentNumber yüzdenin sayısıdır (Go noktasıyla): 10'un altında bir ondalık, 0,05'in altı 0.
func percentNumber(f float64) string {
	y := f * 100
	if y < 0.05 {
		return "0"
	}
	if y >= 9.95 {
		return strconv.Itoa(int(math.Round(y)))
	}
	return strconv.FormatFloat(y, 'f', 1, 64)
}

// count bir ölçüyü 3 anlamlı basamakla yazar (çubukla aynı kural).
func count(f float64) string { return logging.FormatNumber(f) }

// msText milisaniyeyi tablo sütununa uygun yazar: 21,3 · 0,355 · 0,002 (üslü yazım yok).
func msText(ms float64) string {
	digits := 3
	switch {
	case ms >= 100:
		digits = 0
	case ms >= 10:
		digits = 1
	case ms >= 1:
		digits = 2
	}
	return messages.Decimal(strconv.FormatFloat(ms, 'f', digits, 64))
}

// full bir tam sayıyı binlik boşluklarla yazar: 15 000.
func full(n int64) string {
	s := strconv.FormatInt(n, 10)
	minus := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	if minus {
		return "-" + b.String()
	}
	return b.String()
}

// durationText saniyeyi okunur yazar: 850 ms, 12,3 sn, 3 dk 2 sn.
func durationText(s float64) string {
	switch {
	case s < 1:
		return count(s*1000) + " ms"
	case s < 60:
		return messages.Text(messages.H8218, count(s))
	}
	return logging.FormatDuration(s)
}

// nBytes bayt sayısını okunur yazar (1,2 GB).
func nBytes(n int64) string { return bind.FormatBytes(n).MessageText(codes.Display().Code) }

// trim metni en çok n karaktere kırpar (sona … koyarak).
func trim(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// left metni n karaktere sağdan boşlukla tamamlar (kırparak).
func left(s string, n int) string {
	s = trim(s, n)
	return s + strings.Repeat(" ", max(0, n-utf8.RuneCountInString(s)))
}

// right metni n karaktere soldan boşlukla tamamlar.
func right(s string, n int) string {
	return strings.Repeat(" ", max(0, n-utf8.RuneCountInString(s))) + s
}

// noStep program adım bildirmiyorsa gösterilen nottur (adlar kod listesinden).
func noStep() string {
	return messages.Text(messages.H8200, codes.NameRef(codes.G811), codes.NameRef(codes.Y010))
}

// linePos bir satırın "dosya:satır" yazımıdır ('aktar' ile yüklenen modülde modülün dosyası);
// dosyası bilinmeyen satır (dosya alanı boş eski bir görüntü) "fonksiyon():satır" yazılır.
func linePos(s LineInfo) string {
	if s.File != "" {
		return filepath.Base(s.File) + ":" + strconv.Itoa(s.Line)
	}
	return s.Function + "():" + strconv.Itoa(s.Line)
}

// lineText satırın metnidir; dosyası bilinmiyorsa bunu söyler.
func lineText(s LineInfo) string {
	if s.File == "" {
		return messages.Text(messages.H8201)
	}
	return s.Text
}

// functionName üst düzey kodu adlandırır.
func functionName(ad string) string {
	if ad == "" {
		return messages.Text(messages.H8202)
	}
	return ad
}

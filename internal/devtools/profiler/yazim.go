package profiler

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// Raporların ortak yazım yardımcıları (Türkçe sayı yazımı: ondalık virgül, binlik boşluk).

// yuzde payı yüzde olarak yazar: %41, %3,5 (10'un altında bir ondalık).
func yuzde(f float64) string {
	y := f * 100
	if y < 0.05 {
		return "%0"
	}
	if y >= 9.95 {
		return "%" + strconv.Itoa(int(math.Round(y)))
	}
	return "%" + strings.Replace(strconv.FormatFloat(y, 'f', 1, 64), ".", ",", 1)
}

// sayi bir ölçüyü 3 anlamlı basamakla yazar (çubukla aynı kural).
func sayi(f float64) string { return logging.SayiYazimi(f) }

// msMetni milisaniyeyi tablo sütununa uygun yazar: 21,3 · 0,355 · 0,002 (üslü yazım yok).
func msMetni(ms float64) string {
	basamak := 3
	switch {
	case ms >= 100:
		basamak = 0
	case ms >= 10:
		basamak = 1
	case ms >= 1:
		basamak = 2
	}
	return strings.Replace(strconv.FormatFloat(ms, 'f', basamak, 64), ".", ",", 1)
}

// tam bir tam sayıyı binlik boşluklarla yazar: 15 000.
func tam(n int64) string {
	s := strconv.FormatInt(n, 10)
	eksi := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	if eksi {
		return "-" + b.String()
	}
	return b.String()
}

// sureMetni saniyeyi okunur yazar: 850 ms, 12,3 sn, 3 dk 2 sn.
func sureMetni(s float64) string {
	switch {
	case s < 1:
		return sayi(s*1000) + " ms"
	case s < 60:
		return sayi(s) + " sn"
	}
	return logging.SureYazimi(s)
}

// bayt bayt sayısını okunur yazar (1,2 GB).
func bayt(n int64) string { return bind.FormatBytes(n) }

// kirp metni en çok n karaktere kırpar (sona … koyarak).
func kirp(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// sola metni n karaktere sağdan boşlukla tamamlar (kırparak).
func sola(s string, n int) string {
	s = kirp(s, n)
	return s + strings.Repeat(" ", max(0, n-utf8.RuneCountInString(s)))
}

// saga metni n karaktere soldan boşlukla tamamlar.
func saga(s string, n int) string {
	return strings.Repeat(" ", max(0, n-utf8.RuneCountInString(s))) + s
}

// adimYok program adım bildirmiyorsa gösterilen nottur (adlar kod listesinden).
func adimYok() string {
	return fmt.Sprintf("adım bilgisi yok: eğitim döngüsünde %s(...) ve .%s(...) kullanın", codes.Name(codes.G811), codes.Name(codes.Y010))
}

// satirKonumu bir satırın "dosya:satır" yazımıdır; ana programın dışındaki (aktarılan
// modüldeki) satır "fonksiyon():satır" yazılır (satırın dosyası örnekte yoktur).
func satirKonumu(s SatirBilgisi) string {
	if s.Dosya != "" {
		return filepath.Base(s.Dosya) + ":" + strconv.Itoa(s.Satir)
	}
	return s.Fonksiyon + "():" + strconv.Itoa(s.Satir)
}

// satirMetni satırın metnidir; ana programın dışındaysa bunu söyler.
func satirMetni(s SatirBilgisi) string {
	if s.Dosya == "" {
		return "(aktarılan modülde)"
	}
	return s.Metin
}

// fonksiyonAdi üst düzey kodu adlandırır.
func fonksiyonAdi(ad string) string {
	if ad == "" {
		return "(üst düzey)"
	}
	return ad
}

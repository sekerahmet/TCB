package turkish

import "testing"

// Kural (8.4): hata ve uyarı çıktısında denetim karakterleri 2.6'nın kaçışıyla (\u{1b}) görünür
// biçime çevrilir: dışarıdan gelen bir veri (dosya adı, JSON değeri) iletiye girerse terminali
// silemez, metni gizleyemez. Satır sonu ve sekme korunur; Türkçe ve öteki yazdırılabilir
// karakterler değişmez.
func TestPrintable(t *testing.T) {
	for in, want := range map[string]string{
		"düz metin: şğüöçı İ":     "düz metin: şğüöçı İ",
		"satır\nsekme\t":          "satır\nsekme\t",
		"a\x1b[2Jb":               `a\u{1b}[2Jb`,
		"zil\a ve geri\b":         `zil\u{7} ve geri\u{8}`,
		"satır başı\r":            `satır başı\u{d}`,
		"sil\x7f":                 `sil\u{7f}`,
		"C1\u009b31m":             `C1\u{9b}31m`,
		"sıfır\x00":               `sıfır\u{0}`,
		"geçersiz UTF-8 \xff son": "geçersiz UTF-8 � son",
	} {
		if got := Printable(in); got != want {
			t.Errorf("Printable(%q) = %q, %q bekleniyordu", in, got, want)
		}
	}
}

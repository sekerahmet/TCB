package turkish

import "testing"

// Hata ve uyarı çıktısında denetim karakterleri görünür biçime çevrilir: dışarıdan gelen bir
// veri (dosya adı, JSON değeri) iletiye girerse terminali silemez, metni gizleyemez. Satır sonu
// ve sekme korunur; Türkçe ve öteki yazdırılabilir karakterler değişmez.
func TestPrintable(t *testing.T) {
	for in, want := range map[string]string{
		"düz metin: şğüöçı İ":     "düz metin: şğüöçı İ",
		"satır\nsekme\t":          "satır\nsekme\t",
		"a\x1b[2Jb":               `a\x1b[2Jb`,
		"zil\a ve geri\b":         `zil\x07 ve geri\x08`,
		"satır başı\r":            `satır başı\x0d`,
		"sil\x7f":                 `sil\x7f`,
		"C1\u009b31m":             `C1\u009b31m`,
		"sıfır\x00":               `sıfır\x00`,
		"geçersiz UTF-8 \xff son": "geçersiz UTF-8 � son",
	} {
		if got := Printable(in); got != want {
			t.Errorf("Printable(%q) = %q, %q bekleniyordu", in, got, want)
		}
	}
}

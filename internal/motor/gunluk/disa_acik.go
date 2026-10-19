package gunluk

import "os"

// Araçların (tr izle, tr profil) çubukla aynı kuralla yazması için dışa açılan yardımcılar.

// SayiYazimi bir ölçüyü 3 anlamlı basamakla Türkçe yazar (2,31 · 0,000312 → 3,12e-4).
func SayiYazimi(f float64) string { return sayiYazimi(f) }

// SureYazimi geçen süreyi yazar: "45 sn", "3 dk 2 sn", "2 sa 5 dk".
func SureYazimi(sn float64) string { return sureYazimi(sn) }

// TerminalMi dosya bir terminal mi (dosyaya ya da boruya yönlendirilmiş çıktı değil)?
func TerminalMi(f *os.File) bool { return terminalMi(f) }

// TerminalGenisligi terminalin sütun sayısıdır (COLUMNS, işletim sistemi; bulunamazsa 80).
func TerminalGenisligi(f *os.File) int { return terminalGenisligi(f) }

// IsKlasoru program bir arka plan işi olarak çalışıyorsa (tr başlat) iş klasörüdür
// (TR_İŞ ortam değişkeni); değilse boş.
func IsKlasoru() string { return os.Getenv(isOrtamDegiskeni) }

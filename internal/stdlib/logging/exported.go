package logging

import "os"

// Araçların (tr izle, tr profil) çubukla aynı kuralla yazması için dışa açılan yardımcılar.

// FormatNumber bir ölçüyü 3 anlamlı basamakla Türkçe yazar (2,31 · 0,000312 → 3,12e-4).
func FormatNumber(f float64) string { return formatNumber(f) }

// FormatDuration geçen süreyi yazar: "45 sn", "3 dk 2 sn", "2 sa 5 dk".
func FormatDuration(sec float64) string { return formatDuration(sec) }

// IsTerminal dosya bir terminal mi (dosyaya ya da boruya yönlendirilmiş çıktı değil)?
func IsTerminal(f *os.File) bool { return isTerminal(f) }

// TerminalWidth terminalin sütun sayısıdır (COLUMNS, işletim sistemi; bulunamazsa 80).
func TerminalWidth(f *os.File) int { return terminalWidth(f) }

// JobDir program bir arka plan işi olarak çalışıyorsa (tr başlat) iş klasörüdür
// (TR_İŞ ortam değişkeni); değilse boş.
func JobDir() string { return os.Getenv(jobEnvVar) }

package logging

import (
	"os"
	"strconv"
)

// defaultWidth terminal genişliği bulunamazsa kullanılır.
const defaultWidth = 80

// isTerminal dosya bir terminal (karakter aygıtı) mı? Dosyaya ya da boruya yönlendirilmiş
// çıktı terminal değildir.
func isTerminal(f *os.File) bool {
	b, err := f.Stat()
	return err == nil && b.Mode()&os.ModeCharDevice != 0
}

// terminalWidth terminalin sütun sayısıdır: önce COLUMNS ortam değişkeni, sonra işletim
// sistemine sorulur (systemWidth), bulunamazsa 80.
func terminalWidth(f *os.File) int {
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 10 {
		return n
	}
	if n := systemWidth(f); n > 10 {
		return n
	}
	return defaultWidth
}

package gunluk

import (
	"os"
	"strconv"
)

// varsayilanGenislik terminal genişliği bulunamazsa kullanılır.
const varsayilanGenislik = 80

// terminalMi dosya bir terminal (karakter aygıtı) mı? Dosyaya ya da boruya yönlendirilmiş
// çıktı terminal değildir.
func terminalMi(f *os.File) bool {
	b, err := f.Stat()
	return err == nil && b.Mode()&os.ModeCharDevice != 0
}

// terminalGenisligi terminalin sütun sayısıdır: önce COLUMNS ortam değişkeni, sonra işletim
// sistemine sorulur (sistemGenisligi), bulunamazsa 80.
func terminalGenisligi(f *os.File) int {
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 10 {
		return n
	}
	if n := sistemGenisligi(f); n > 10 {
		return n
	}
	return varsayilanGenislik
}

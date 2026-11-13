package sysmem

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

// Available şu an yeni işe verilebilecek belleği (bayt) /proc/meminfo'nun MemAvailable
// satırından okur. Sysinfo'nun Freeram'ı kullanılmaz: sayfa önbelleğini ve geri alınabilir
// belleği saymaz, kullanılabilir belleği olduğundan çok küçük gösterir.
func Available() (uint64, bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	defer f.Close()
	return parseMemAvailable(f)
}

// parseMemAvailable "MemAvailable:   123456 kB" satırını bulur ve baytı verir.
// Satır yoksa (çok eski çekirdek) ya da biçim bozuksa okunamamış sayılır.
func parseMemAvailable(r io.Reader) (uint64, bool) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		rest, ok := strings.CutPrefix(s.Text(), "MemAvailable:")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) != 2 || fields[1] != "kB" {
			return 0, false
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil || kb > (1<<64-1)/1024 {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}

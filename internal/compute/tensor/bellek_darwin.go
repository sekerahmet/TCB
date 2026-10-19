package tensor

import "syscall"

// fizikselBellek makinenin toplam fiziksel belleğini (bayt) sistem çağrısıyla okur.
// Değer 8 baytlık küçük sonlu bir tam sayıdır; sistem çağrısı sarmalayıcısı sondaki
// sıfır baytı kırpabildiğinden eksik baytlar sıfır sayılır.
func fizikselBellek() (uint64, bool) {
	s, err := syscall.Sysctl("hw.memsize")
	if err != nil || len(s) == 0 || len(s) > 8 {
		return 0, false
	}
	var b uint64
	for i := len(s) - 1; i >= 0; i-- {
		b = b<<8 | uint64(s[i])
	}
	return b, true
}

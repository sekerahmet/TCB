package gunluk

import (
	"os"
	"syscall"
	"unsafe"
)

var konsolBilgisi = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleScreenBufferInfo")

// konsolEkranBilgisi işletim sisteminin konsol ekran arabelleği bilgisidir (CONSOLE_SCREEN_BUFFER_INFO).
type konsolEkranBilgisi struct {
	boyut, imlec       [2]int16
	nitelik            uint16
	sol, ust, sag, alt int16
	enBuyuk            [2]int16
}

// sistemGenisligi konsol penceresinin görünen sütun sayısıdır; konsol değilse 0.
func sistemGenisligi(f *os.File) int {
	var b konsolEkranBilgisi
	r, _, _ := konsolBilgisi.Call(f.Fd(), uintptr(unsafe.Pointer(&b)))
	if r == 0 {
		return 0
	}
	return int(b.sag-b.sol) + 1
}

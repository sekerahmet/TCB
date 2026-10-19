package logging

import (
	"os"
	"syscall"
	"unsafe"
)

var consoleInfo = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleScreenBufferInfo")

// consoleScreenInfo işletim sisteminin konsol ekran arabelleği bilgisidir (CONSOLE_SCREEN_BUFFER_INFO).
type consoleScreenInfo struct {
	size, cursor             [2]int16
	attr                     uint16
	left, top, right, bottom int16
	maxSize                  [2]int16
}

// systemWidth konsol penceresinin görünen sütun sayısıdır; konsol değilse 0.
func systemWidth(f *os.File) int {
	var b consoleScreenInfo
	r, _, _ := consoleInfo.Call(f.Fd(), uintptr(unsafe.Pointer(&b)))
	if r == 0 {
		return 0
	}
	return int(b.right-b.left) + 1
}

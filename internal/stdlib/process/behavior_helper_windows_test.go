//go:build windows

package process_test

import (
	"os/exec"
	"syscall"
	"unsafe"
)

// detachedProcess süreci konsolsuz başlatır.
const detachedProcess = 0x00000008

// consoleAttached sürecin konsolu olduğunu söyler: Ctrl+Break ancak o zaman gider.
func consoleAttached() bool {
	var list [1]uint32
	n, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList").Call(uintptr(unsafe.Pointer(&list[0])), 1)
	return n != 0
}

// withoutConsole süreci konsolsuz başlatır.
func withoutConsole(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess}
}

// leaveGroup Windows'ta yoktur: iş nesnesinden çıkılmaz (TestEscapedGrandchildDoesNotBlockTheStop).
func leaveGroup(*exec.Cmd) {}

// killSelf Windows'ta sinyal olmadığından çağrılmaz (TestKilledBySignal).
func killSelf() {}

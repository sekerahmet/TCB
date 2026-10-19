//go:build windows

package uctanuca

import (
	"os/exec"
	"syscall"
	"testing"
)

// sureciHazirla süreci kendi konsol süreç grubunda başlatır: Ctrl+Break olayı yalnızca ona
// gider, testi çalıştıran sürece değil.
func sureciHazirla(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// kesmeGonder sürecin grubuna Ctrl+Break konsol olayı gönderir (Go'da os.Interrupt olarak
// gelir; yeni süreç grubunda Ctrl+C olayı kapalıdır). Test süreci bir konsola bağlı değilse
// olay gönderilemez; o zaman test atlanır.
func kesmeGonder(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	const ctrlBreakOlayi = 1
	islev := syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")
	if r, _, err := islev.Call(ctrlBreakOlayi, uintptr(cmd.Process.Pid)); r == 0 {
		t.Skipf("konsol olayı gönderilemedi (test süreci bir konsola bağlı değil olabilir): %v", err)
	}
}

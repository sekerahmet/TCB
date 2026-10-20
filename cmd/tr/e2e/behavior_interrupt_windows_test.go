//go:build windows

package e2e

import (
	"os/exec"
	"syscall"
	"testing"
)

// prepareProcess süreci kendi konsol süreç grubunda başlatır: Ctrl+Break olayı yalnızca ona
// gider, testi çalıştıran sürece değil.
func prepareProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// sendInterrupt sürecin grubuna Ctrl+Break konsol olayı gönderir (Go'da os.Interrupt olarak
// gelir; yeni süreç grubunda Ctrl+C olayı kapalıdır). Test süreci bir konsola bağlı değilse
// olay gönderilemez; o zaman test atlanır.
func sendInterrupt(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	const ctrlBreakEvent = 1
	function := syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")
	if r, _, err := function.Call(ctrlBreakEvent, uintptr(cmd.Process.Pid)); r == 0 {
		t.Skipf("konsol olayı gönderilemedi (test süreci bir konsola bağlı değil olabilir): %v", err)
	}
}

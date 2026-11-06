//go:build !windows

package process_test

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// leaveGroup süreci kendi oturumuna alır: programın grubundan kaçar.
func leaveGroup(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

// consoleAttached Unix'te nazik istek konsola bağlı değildir.
func consoleAttached() bool { return true }

// withoutConsole yalnız Windows'ta iş görür (TestNoConsoleUsesTheKillPath).
func withoutConsole(*exec.Cmd) {}

// killSelf yardımcı programı SIGKILL ile öldürür.
func killSelf() {
	_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
	time.Sleep(time.Minute)
}

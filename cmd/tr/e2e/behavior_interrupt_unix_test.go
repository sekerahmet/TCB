//go:build !windows

package e2e

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func prepareProcess(cmd *exec.Cmd) {}

// sendInterrupt sürece SIGINT (terminalde Ctrl+C) gönderir.
func sendInterrupt(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
}

// interruptEnd sürecin isteğin sinyaliyle (SIGINT) öldüğünü söyler: kabuk Ctrl+C'de 130 görür (1.9).
func interruptEnd(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGINT
}

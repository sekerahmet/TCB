//go:build !windows

package e2e

import (
	"os"
	"os/exec"
	"testing"
)

func sureciHazirla(cmd *exec.Cmd) {}

// sendInterrupt sürece SIGINT (terminalde Ctrl+C) gönderir.
func kesmeGonder(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
}

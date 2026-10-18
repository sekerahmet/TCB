//go:build !windows

package main

import (
	"os"
	"os/exec"
	"testing"
)

func sureciHazirla(cmd *exec.Cmd) {}

// kesmeGonder sürece SIGINT (terminalde Ctrl+C) gönderir.
func kesmeGonder(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
}

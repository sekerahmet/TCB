//go:build !windows

package process

import (
	"os"
	"syscall"
)

// exitSignal programı öldüren sinyalin adını verir (21.4).
func exitSignal(state *os.ProcessState) (string, bool) {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return "", false
	}
	return status.Signal().String(), true
}

//go:build !windows

package process_test

import (
	"os"
	"syscall"
	"time"
)

// killSelf yardımcı programı SIGKILL ile öldürür.
func killSelf() {
	_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
	time.Sleep(time.Minute)
}

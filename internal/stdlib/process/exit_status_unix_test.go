//go:build !windows

package process

import (
	"syscall"
	"testing"
)

// Sinyal adıyla söylenir, listede olmayan numarasıyla; Go'nun İngilizce metni iletiye girmez (21.4).
func TestSignalName(t *testing.T) {
	if got := signalName(syscall.SIGTERM); got != "SIGTERM" {
		t.Errorf("%q", got)
	}
	if got := signalName(syscall.Signal(99)); got != "99" {
		t.Errorf("%q", got)
	}
}

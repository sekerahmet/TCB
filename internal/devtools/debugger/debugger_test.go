package debugger

import (
	"testing"
)

func TestDebuggerBreakpointToggleIsSafe(t *testing.T) {
	d := NewDebugger("t.tr", "a\nb", false)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 2000; i++ {
			d.toggleBreakpoint(i % 7)
		}
		close(done)
	}()
	for i := 0; i < 2000; i++ {
		_ = d.hasBreakpoint(i % 7)
	}
	<-done
}

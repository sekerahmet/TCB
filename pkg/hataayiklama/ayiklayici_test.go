package hataayiklama

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
	"tr_dili/pkg/vm"
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

func TestDebuggerJumpMessageUsesCorrectSuffix(t *testing.T) {
	for line, want := range map[int]string{6: "Satır 6'ya zıplandı", 9: "Satır 9'a zıplandı", 3: "Satır 3'e zıplandı", 40: "Satır 40'a zıplandı"} {
		d := NewDebugger("t.tr", strings.Repeat("x\n", 50), false)
		d.Reader = bufio.NewReader(strings.NewReader(fmt.Sprintf("zıpla %d\n", line)))
		if got := d.promptCLI(vm.New(), 1, vm.NewEnvironment(nil)); got != line {
			t.Errorf("zıpla %d: hedef satır %d döndü", line, got)
		}
		if !strings.Contains(d.PauseReason, want) {
			t.Errorf("%q bekleniyordu, %q alındı", want, d.PauseReason)
		}
	}
}

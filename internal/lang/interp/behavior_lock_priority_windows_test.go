package interp

import (
	"syscall"
	"testing"
)

const highPriorityClass = 0x80

func init() {
	raiseTestPriority = func(t *testing.T) {
		kernel := syscall.NewLazyDLL("kernel32.dll")
		get, set := kernel.NewProc("GetPriorityClass"), kernel.NewProc("SetPriorityClass")
		self, err := syscall.GetCurrentProcess()
		if err != nil {
			t.Fatal(err)
		}
		old, _, err := get.Call(uintptr(self))
		if old == 0 {
			t.Fatal(err)
		}
		if ok, _, err := set.Call(uintptr(self), highPriorityClass); ok == 0 {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if ok, _, err := set.Call(uintptr(self), old); ok == 0 {
				t.Error(err)
			}
		})
	}
}

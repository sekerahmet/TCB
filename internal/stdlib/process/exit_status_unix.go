//go:build !windows

package process

import (
	"os"
	"strconv"
	"syscall"
)

// signalNames sinyallerin POSIX adlarıdır: Go'nun Signal.String metni İngilizcedir ("killed"), ad ise
// dilden bağımsızdır ve iletiyi okuyan onu arayabilir.
var signalNames = map[syscall.Signal]string{
	syscall.SIGABRT: "SIGABRT", syscall.SIGALRM: "SIGALRM", syscall.SIGBUS: "SIGBUS",
	syscall.SIGFPE: "SIGFPE", syscall.SIGHUP: "SIGHUP", syscall.SIGILL: "SIGILL",
	syscall.SIGINT: "SIGINT", syscall.SIGKILL: "SIGKILL", syscall.SIGPIPE: "SIGPIPE",
	syscall.SIGQUIT: "SIGQUIT", syscall.SIGSEGV: "SIGSEGV", syscall.SIGTERM: "SIGTERM",
	syscall.SIGUSR1: "SIGUSR1", syscall.SIGUSR2: "SIGUSR2",
}

// exitSignal programı öldüren sinyalin adını verir (21.4).
func exitSignal(state *os.ProcessState) (string, bool) {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return "", false
	}
	return signalName(status.Signal()), true
}

// signalName sinyalin adıdır; listede olmayan sinyal numarasıyla söylenir.
func signalName(s syscall.Signal) string {
	if name, ok := signalNames[s]; ok {
		return name
	}
	return strconv.Itoa(int(s))
}

//go:build windows

package process

import (
	"os/exec"
	"syscall"
	"unsafe"
)

// Windows'ta süreç grubu bir iş nesnesidir (21.5): kapanınca içindeki süreçler de ölür, TR çökse de
// (KILL_ON_JOB_CLOSE). Nazik istek Ctrl+Break'tir, programın konsol grubuna gider.
//
// Program askıda başlar: iş nesnesine girmeden torun açamasın. Go ilk iş parçacığının tanıtıcısını
// kapatıyor, ResumeThread olmaz; süreç NtResumeProcess ile sürer.

const (
	createSuspended = 0x00000004

	processTerminate     = 0x0001
	processSetQuota      = 0x0100
	processSuspendResume = 0x0800

	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x00002000

	ctrlBreakEvent = 1
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	ntdll                        = syscall.NewLazyDLL("ntdll.dll")
	procCreateJobObject          = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject       = kernel32.NewProc("TerminateJobObject")
	procGenerateConsoleCtrlEvent = kernel32.NewProc("GenerateConsoleCtrlEvent")
	procNtResumeProcess          = ntdll.NewProc("NtResumeProcess")
)

// jobLimits JOBOBJECT_EXTENDED_LIMIT_INFORMATION'dır.
type jobLimits struct {
	perProcessUserTimeLimit int64
	perJobUserTimeLimit     int64
	limitFlags              uint32
	minimumWorkingSetSize   uintptr
	maximumWorkingSetSize   uintptr
	activeProcessLimit      uint32
	affinity                uintptr
	priorityClass           uint32
	schedulingClass         uint32
	ioCounters              [6]uint64
	processMemoryLimit      uintptr
	jobMemoryLimit          uintptr
	peakProcessMemoryUsed   uintptr
	peakJobMemoryUsed       uintptr
}

// processGroup programın iş nesnesi ve konsol grubudur.
type processGroup struct {
	job syscall.Handle
	pid uint32
}

func (g *processGroup) prepare(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createSuspended}
}

// started askıdaki programı iş nesnesine alır ve sürdürür.
func (g *processGroup) started(c *exec.Cmd) error {
	g.pid = uint32(c.Process.Pid)
	process, err := syscall.OpenProcess(processTerminate|processSetQuota|processSuspendResume, false, g.pid)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(process)
	job, _, err := procCreateJobObject.Call(0, 0)
	if job == 0 {
		return err
	}
	g.job = syscall.Handle(job)
	if err := g.setLimits(jobObjectLimitKillOnJobClose); err != nil {
		g.release(true)
		return err
	}
	if ok, _, err := procAssignProcessToJobObject.Call(job, uintptr(process)); ok == 0 {
		g.release(true)
		return err
	}
	if status, _, _ := procNtResumeProcess.Call(uintptr(process)); status != 0 {
		g.release(true)
		return syscall.Errno(status)
	}
	return nil
}

// setLimits iş nesnesinin sınır bayraklarını yazar.
func (g *processGroup) setLimits(flags uint32) error {
	limits := jobLimits{limitFlags: flags}
	ok, _, err := procSetInformationJobObject.Call(uintptr(g.job), jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if ok == 0 {
		return err
	}
	return nil
}

// request Ctrl+Break'i programın konsol grubuna yollar. TR'nin konsolu yoksa ya da program başka
// konsoldaysa olay gitmez: false verir, öldürme yolu çalışır.
func (g *processGroup) request() bool {
	ok, _, _ := procGenerateConsoleCtrlEvent.Call(ctrlBreakEvent, uintptr(g.pid))
	return ok != 0
}

// kill iş nesnesindeki bütün süreçleri öldürür.
func (g *processGroup) kill() {
	_, _, _ = procTerminateJobObject.Call(uintptr(g.job), 1)
}

// release iş nesnesini kapatır. Program kendiliğinden bittiyse (stopped false) önce kapanışta öldürme
// kaldırılır: torun Unix'teki gibi yaşar.
func (g *processGroup) release(stopped bool) {
	if g.job == 0 {
		return
	}
	if !stopped {
		_ = g.setLimits(0)
	}
	_ = syscall.CloseHandle(g.job)
	g.job = 0
}

package machine

import (
	"syscall"
	"unsafe"
)

// basicLimitInformation JOBOBJECT_BASIC_LIMIT_INFORMATION'dır; okunmayan alanlar boş adlıdır.
type basicLimitInformation struct {
	_          int64 // PerProcessUserTimeLimit
	_          int64 // PerJobUserTimeLimit
	limitFlags uint32
	_          uintptr // MinimumWorkingSetSize
	_          uintptr // MaximumWorkingSetSize
	_          uint32  // ActiveProcessLimit
	_          uintptr // Affinity
	_          uint32  // PriorityClass
	_          uint32  // SchedulingClass
}

// extendedLimitInformation JOBOBJECT_EXTENDED_LIMIT_INFORMATION'dır (amd64'te 144 bayt).
type extendedLimitInformation struct {
	basic              basicLimitInformation
	_                  [6]uint64 // IoInfo
	processMemoryLimit uintptr
	jobMemoryLimit     uintptr
	_                  uintptr // PeakProcessMemoryUsed
	_                  uintptr // PeakJobMemoryUsed
}

// jobObjectExtendedLimitInformation QueryInformationJobObject'in sınıf numarasıdır.
const jobObjectExtendedLimitInformation = 9

// memoryLimit sürecin iş nesnesinin bellek sınırını okur. İş nesnesi dışındaki süreçte çağrı
// başarılıdır ve bayraklar sıfırdır: sınır yok (ölçüldü, Dost Interface 2026-11-13).
func memoryLimit() (bytes int64, limited, ok bool) {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("QueryInformationJobObject")
	if proc.Find() != nil {
		return 0, false, false
	}
	var info extendedLimitInformation
	r, _, _ := proc.Call(0, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), 0)
	if r == 0 {
		return 0, false, false
	}
	b, l := jobMemoryLimit(info.basic.limitFlags, uint64(info.processMemoryLimit), uint64(info.jobMemoryLimit))
	return b, l, true
}

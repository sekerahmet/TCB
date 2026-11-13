// Package machine, TR_machine()'dir (dil tanımı 19.12): sürecin çalıştığı makinenin o anki
// görüntüsü bir TR_Machine değeri olarak. Her çağrı yeni bir görüntü okur; üyeler o anın
// değeridir (alanlar kurulurken bir kez okunur). Okunamayan bilgi sessizce TR_none ya da 0
// olmaz: üyesi okununca H7735 verir (TR_none "sınır yok" demektir).
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package machine

import (
	"runtime"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/sysmem"
)

// MachineType TR_Machine'in tip adıdır.
var MachineType = interp.TypeName(codes.TypeMachine)

// Register TR_machine()'i VM'ye kaydeder.
func Register(v *interp.VM) {
	v.RegisterNoNamed(codes.FnMachine, machineBuiltin)
}

func machineBuiltin(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.FnMachine, 0, len(args))
	}
	return read(), nil
}

// member bir üyenin okunduğu andaki değeri ya da okunamamasının hatasıdır.
type member struct {
	v   interp.Value
	err error
}

// Machine bir anlık görüntüdür; değişmez.
type Machine struct {
	m map[codes.Code]member
}

var machineMembers = []codes.Code{codes.MemMemoryTotalBytes, codes.MemMemoryLimitBytes,
	codes.MemMemoryAvailableBytes, codes.MemUsableCpus, codes.MemCpuFeatures, codes.MemOs, codes.MemArch}

// read bütün üyeleri bir kez okur.
func read() *Machine {
	mc := &Machine{m: map[codes.Code]member{}}
	if b, ok := sysmem.Physical(); ok {
		mc.m[codes.MemMemoryTotalBytes] = member{v: interp.NewInt(clampInt(b))}
	} else {
		mc.m[codes.MemMemoryTotalBytes] = unreadable(codes.MemMemoryTotalBytes, "total")
	}
	switch b, limited, ok := memoryLimit(); {
	case !ok:
		mc.m[codes.MemMemoryLimitBytes] = unreadable(codes.MemMemoryLimitBytes, "limit")
	case limited:
		mc.m[codes.MemMemoryLimitBytes] = member{v: interp.NewInt(b)}
	default:
		mc.m[codes.MemMemoryLimitBytes] = member{v: interp.Null}
	}
	if b, ok := sysmem.Available(); ok {
		mc.m[codes.MemMemoryAvailableBytes] = member{v: interp.NewInt(clampInt(b))}
	} else {
		mc.m[codes.MemMemoryAvailableBytes] = unreadable(codes.MemMemoryAvailableBytes, "available")
	}
	mc.m[codes.MemUsableCpus] = member{v: interp.NewInt(int64(runtime.NumCPU()))}
	features := tensor.CPUFeatures()
	el := make([]interp.Value, len(features))
	for i, f := range features {
		el[i] = &interp.StringVal{Val: f}
	}
	mc.m[codes.MemCpuFeatures] = member{v: &interp.ListVal{Elements: el}}
	mc.m[codes.MemOs] = member{v: &interp.StringVal{Val: osName(runtime.GOOS)}}
	mc.m[codes.MemArch] = member{v: &interp.StringVal{Val: archName(runtime.GOARCH)}}
	return mc
}

// unreadable üyenin H7735'idir: item iletinin hangi bilgiyi okuyamadığını seçer, os macOS'ta
// "bu sürüm okumaz" cümlesini ekler.
func unreadable(kd codes.Code, item string) member {
	return member{err: messages.New(messages.H7735, codes.CallNameRef(kd), item, osName(runtime.GOOS))}
}

func clampInt(b uint64) int64 {
	if b > 1<<63-1 {
		return 1<<63 - 1
	}
	return int64(b)
}

func (mc *Machine) Type() interp.ValueType { return MachineType }
func (mc *Machine) IsTruthy() bool         { return true }

// Inspect değerin metnidir (3.14): tipin adı.
func (mc *Machine) Inspect() string { return codes.ValueNames().Name(codes.TypeMachine) }

// GetProperty üyenin okunduğu andaki değerini ya da okunamamasının hatasını verir.
func (mc *Machine) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	if x, ok := mc.m[kd]; ok {
		return x.v, x.err
	}
	return nil, bind.MemberError(MachineType, name, machineMembers)
}

var _ interp.PropertyGetter = (*Machine)(nil)

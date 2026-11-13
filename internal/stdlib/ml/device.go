package ml

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// ----------------------------------------------------------------------------
// Cihazlar: t.cihaz, t.cihaza(ad), varsayılan_cihaz, cihazlar
// ----------------------------------------------------------------------------
//
// Anlam (docs/spec/15_tensorler.md, cihaz kuralları):
//   - varsayılan_cihaz(ad) bundan sonra oluşturulan bütün tensörlerin (oluşturucular,
//     veri kümesi örnekleyicisinin toplu işleri) cihazını seçer.
//   - Türev izleyen bir YAPRAĞIN .cihaza(ad) ile taşınması, hedef cihazda türev izleyen
//     yeni bir yaprak verir: iyileştirici parametreleri gerçekten o cihazdaki yapraklar
//     olur, gradyan yeni yaprağın .gradyan'ında birikir. Bir işlemin sonucu (ara tensör)
//     taşınınca kopya türevlenebilirdir: gradyan kaynağa geri akar.
//   - Farklı cihazlardaki iki tensörle işlem açık hatadır.

// parseDevice "cpu", "gpu", "gpu:1" gibi bir adı cihaza çevirir ("gpu:0" = "gpu").
func parseDevice(ad string) (tensor.Device, error) {
	a := strings.ToLower(strings.TrimSpace(ad))
	switch {
	case a == "cpu":
		return tensor.CPU, nil
	case a == "gpu":
		return tensor.GPU(0), nil
	case strings.HasPrefix(a, "gpu:"):
		// Yalnızca rakamlar: işaret, boşluk ve sayıdan sonraki çöp ("gpu:0abc") kabul edilmez.
		if digits := a[4:]; digits != "" && strings.Trim(digits, "0123456789") == "" {
			if i, err := strconv.Atoi(digits); err == nil {
				return tensor.GPU(i), nil
			}
		}
	}
	return tensor.Device{}, messages.New(messages.H6800, ad)
}

// deviceNames kullanılabilir cihazların uzun adları ("cpu", "gpu:0", ...; 15.37: okunan ad tektir,
// .TR_device ile aynı). TR_devices() ve H6812/H6813'ün listesi bunu yazar.
func deviceNames() []string {
	ds := tensor.Devices()
	ad := make([]string, len(ds))
	for i, d := range ds {
		ad[i] = d.LongName()
	}
	return ad
}

// availableDevice adın çözülüp bu makinede kullanılabildiğini denetler.
func availableDevice(fn any, v interp.Value) (tensor.Device, error) {
	s, ok := v.(*interp.StringVal)
	if !ok {
		return tensor.Device{}, messages.New(messages.H6801, fn, typeName(v))
	}
	dev, err := parseDevice(s.Val)
	if err != nil {
		return dev, err
	}
	if !tensor.HasDevice(dev) {
		names := strings.Join(deviceNames(), ", ")
		if dev.Kind == tensor.KindGPU {
			return dev, messages.New(messages.H6813, dev, names, codes.NameRef(codes.FnDevices), gpu.Status())
		}
		return dev, messages.New(messages.H6812, dev, names, codes.NameRef(codes.FnDevices))
	}
	return dev, nil
}

func (d *TensorValue) toDevice(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(memberToDevice)
	if len(args) != 1 {
		return nil, messages.New(messages.H6802, fn)
	}
	dev, err := availableDevice(fn, args[0])
	if err != nil {
		return nil, err
	}
	leaf := d.t.IsLeaf() && d.t.RequiresGrad()
	r, err := safe(func() *tensor.Tensor {
		if leaf && d.t.Device() != dev {
			var y *tensor.Tensor
			tensor.NoGrad(func() { y = d.t.To(dev) })
			return y.SetRequiresGrad(true)
		}
		return d.t.To(dev)
	})
	if err != nil {
		return nil, err
	}
	if r == d.t {
		return d, nil // aynı cihaz: başlık yeniden sarılmaz (sahiplik tek değerde kalır)
	}
	return New(r), nil
}

// defaultDeviceBuiltin varsayılan_cihaz([ad]): argümanla cihazı seçer ve değer döndürmez
// (15.38, 13.10; denetleyici değer yerinde H1143 verir, no_value.go); argümansız çağrı geçerli
// cihazın uzun adını döndürür ("gpu:0", .TR_device gibi).
func defaultDeviceBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnDefaultDevice)
	if err := bind.CheckNamed(fn.Called(), named); err != nil {
		return nil, err
	}
	if len(args) > 1 {
		return nil, messages.New(messages.H6804, fn, len(args))
	}
	if len(args) == 1 {
		dev, err := availableDevice(fn, args[0])
		if err != nil {
			return nil, err
		}
		if _, err := safe(func() bool { tensor.SetDefaultDevice(dev); return true }); err != nil {
			return nil, err
		}
		return interp.Null, nil
	}
	return &interp.StringVal{Val: tensor.DefaultDevice().LongName()}, nil
}

// devicesBuiltin cihazlar(): kullanılabilir cihaz adları ("cpu" ilk).
func devicesBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnDevices)
	if err := bind.CheckNamed(fn.Called(), named); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, messages.New(messages.H2014, fn, len(args))
	}
	ad := deviceNames()
	el := make([]interp.Value, len(ad))
	for i, a := range ad {
		el[i] = &interp.StringVal{Val: a}
	}
	return &interp.ListVal{Elements: el}, nil
}

// ----------------------------------------------------------------------------
// Kartın bilgisi: cihaz_bilgisi(ad) (15.37a)
// ----------------------------------------------------------------------------

// DeviceInfoType TR_DeviceInfo'nun tip adıdır.
var DeviceInfoType = interp.TypeName(codes.TypeDeviceInfo)

// DeviceInfo bir GPU'nun okunduğu andaki bilgisidir: ad, toplam bellek (bayt), sürücü
// arayüzünün sürümü ("<büyük>.<küçük>"). Değişmez.
type DeviceInfo struct {
	name, driver string
	memory       uint64
}

// NewDeviceInfo bir kart bilgisi değeri kurar (sınamalar kartsız makinede bu değeri Go'dan kurar).
func NewDeviceInfo(name string, memory uint64, driver string) *DeviceInfo {
	return &DeviceInfo{name: name, memory: memory, driver: driver}
}

func (d *DeviceInfo) Type() interp.ValueType { return DeviceInfoType }
func (d *DeviceInfo) IsTruthy() bool         { return true }

// Inspect değerin metnidir (3.14): tipin adı.
func (d *DeviceInfo) Inspect() string { return codes.ValueNames().Name(codes.TypeDeviceInfo) }

var deviceInfoMembers = []codes.Code{codes.MemName, codes.MemMemoryTotalBytes, codes.MemDriverVersion}

// GetProperty .TR_name, .TR_memory_total_bytes ve .TR_driver_version'ı verir.
func (d *DeviceInfo) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	switch kd {
	case codes.MemName:
		return &interp.StringVal{Val: d.name}, nil
	case codes.MemMemoryTotalBytes:
		m := d.memory
		if m > 1<<63-1 {
			m = 1<<63 - 1
		}
		return interp.NewInt(int64(m)), nil
	case codes.MemDriverVersion:
		return &interp.StringVal{Val: d.driver}, nil
	}
	return nil, bind.MemberError(DeviceInfoType, name, deviceInfoMembers)
}

var _ interp.PropertyGetter = (*DeviceInfo)(nil)

// deviceInfoBuiltin cihaz_bilgisi(ad): bir GPU'nun bilgisi. "cpu" H7734 ile makine()'yi önerir
// (işlemcinin bilgisi tek yerde, 19.12); olmayan cihaz 15.38'deki gibi H6812 ya da H6813. Sürücü
// 15.37'deki gibi ilk soruda yüklenir (availableDevice cihazı açtırır).
func deviceInfoBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnDeviceInfo)
	if err := bind.CheckNamed(fn.Called(), named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.FnDeviceInfo, 1, len(args))
	}
	if s, ok := args[0].(*interp.StringVal); ok {
		if d, err := parseDevice(s.Val); err == nil && d.Kind == tensor.KindCPU {
			return nil, messages.New(messages.H7734, fn)
		}
	}
	dev, err := availableDevice(fn, args[0])
	if err != nil {
		return nil, err
	}
	name, memory, driver, ok := gpu.DeviceInfo(dev)
	if !ok {
		return nil, messages.New(messages.H7735, fn, "other", runtime.GOOS)
	}
	return NewDeviceInfo(name, memory, driver), nil
}

// Close program sonunda çağrılır (motor.Kapat): GPU ölçümü açıksa (TR_GPU_PROFILE=1) özeti
// standart hataya yazar ve bağlı dış profilleyicilere topladıkları veriyi yazdırır.
func Close() {
	gpu.Close()
}

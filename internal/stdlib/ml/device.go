package ml

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
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
		var i int
		if _, err := fmt.Sscanf(a[4:], "%d", &i); err == nil && i >= 0 {
			return tensor.GPU(i), nil
		}
	}
	return tensor.Device{}, fmt.Errorf("bilinmeyen cihaz %q; \"cpu\", \"gpu\" ya da \"gpu:1\" gibi bir ad verin", ad)
}

// deviceNames kullanılabilir cihazların adları.
func deviceNames() []string {
	ds := tensor.Devices()
	ad := make([]string, len(ds))
	for i, d := range ds {
		ad[i] = d.String()
	}
	return ad
}

// availableDevice adın çözülüp bu makinede kullanılabildiğini denetler.
func availableDevice(fn string, v interp.Value) (tensor.Device, error) {
	s, ok := v.(*interp.StringVal)
	if !ok {
		return tensor.Device{}, fmt.Errorf("%s() cihaz adını metin olarak bekler (\"cpu\" ya da \"gpu\"), %s verildi", fn, typeName(v))
	}
	dev, err := parseDevice(s.Val)
	if err != nil {
		return dev, err
	}
	if !tensor.HasDevice(dev) {
		m := fmt.Sprintf("%s cihazı kullanılamıyor; bu makinede kullanılabilir cihazlar: %s (%s())",
			dev, strings.Join(deviceNames(), ", "), codes.Name(codes.T721))
		if dev.Kind == tensor.KindGPU {
			m += ". " + gpu.Status()
		}
		return dev, fmt.Errorf("%s", m)
	}
	return dev, nil
}

func (d *TensorValue) toDevice(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(memberToDevice)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(\"cpu\") 1 parametre bekler", fn)
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
	result := New(r)
	result.hasInts = d.hasInts
	return result, nil
}

// defaultDeviceBuiltin varsayılan_cihaz([ad]): argümanla cihazı seçer; her durumda
// geçerli cihazın adını döndürür.
func defaultDeviceBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T720)
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) > 1 {
		return nil, fmt.Errorf("%s(\"gpu\") en çok 1 parametre bekler, %d verildi", fn, len(args))
	}
	if len(args) == 1 {
		dev, err := availableDevice(fn, args[0])
		if err != nil {
			return nil, err
		}
		if _, err := safe(func() bool { tensor.SetDefaultDevice(dev); return true }); err != nil {
			return nil, err
		}
	}
	return &interp.StringVal{Val: tensor.DefaultDevice().String()}, nil
}

// devicesBuiltin cihazlar(): kullanılabilir cihaz adları ("cpu" ilk).
func devicesBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T721)
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, fmt.Errorf("%s() parametre almaz, %d verildi", fn, len(args))
	}
	ad := deviceNames()
	el := make([]interp.Value, len(ad))
	for i, a := range ad {
		el[i] = &interp.StringVal{Val: a}
	}
	return &interp.ListVal{Elements: el}, nil
}

// Close program sonunda çağrılır (motor.Kapat): GPU ölçümü açıksa (TR_GPU_PROFIL=1) özeti
// standart hataya yazar ve bağlı dış profilleyicilere topladıkları veriyi yazdırır; programın
// kurduğu tohumsuz rastgelelik kancasını kaldırır (aynı süreçte sonraki programa geçmesin).
func Close() {
	SetUnseededHook(nil)
	gpu.Close()
}

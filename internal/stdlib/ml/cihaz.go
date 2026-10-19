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

// cihazCoz "cpu", "gpu", "gpu:1" gibi bir adı cihaza çevirir ("gpu:0" = "gpu").
func cihazCoz(ad string) (tensor.Device, error) {
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

// cihazAdlari kullanılabilir cihazların adları.
func cihazAdlari() []string {
	ds := tensor.Devices()
	ad := make([]string, len(ds))
	for i, d := range ds {
		ad[i] = d.String()
	}
	return ad
}

// kullanilabilirCihaz adın çözülüp bu makinede kullanılabildiğini denetler.
func kullanilabilirCihaz(fn string, v interp.Value) (tensor.Device, error) {
	s, ok := v.(*interp.StringVal)
	if !ok {
		return tensor.Device{}, fmt.Errorf("%s() cihaz adını metin olarak bekler (\"cpu\" ya da \"gpu\"), %s verildi", fn, tipAdi(v))
	}
	dev, err := cihazCoz(s.Val)
	if err != nil {
		return dev, err
	}
	if !tensor.HasDevice(dev) {
		m := fmt.Sprintf("%s cihazı kullanılamıyor; bu makinede kullanılabilir cihazlar: %s (%s())",
			dev, strings.Join(cihazAdlari(), ", "), codes.Name(codes.T721))
		if dev.Kind == tensor.KindGPU {
			m += ". " + gpu.Durum()
		}
		return dev, fmt.Errorf("%s", m)
	}
	return dev, nil
}

func (d *Deger) cihaza(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(uyeCihaza)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(\"cpu\") 1 parametre bekler", fn)
	}
	dev, err := kullanilabilirCihaz(fn, args[0])
	if err != nil {
		return nil, err
	}
	yaprak := d.t.IsLeaf() && d.t.RequiresGrad()
	r, err := guvenli(func() *tensor.Tensor {
		if yaprak && d.t.Device() != dev {
			var y *tensor.Tensor
			tensor.NoGrad(func() { y = d.t.To(dev) })
			return y.SetRequiresGrad(true)
		}
		return d.t.To(dev)
	})
	if err != nil {
		return nil, err
	}
	sonuc := Yeni(r)
	sonuc.tamSayili = d.tamSayili
	return sonuc, nil
}

// varsayilanCihazYerlesik varsayılan_cihaz([ad]): argümanla cihazı seçer; her durumda
// geçerli cihazın adını döndürür.
func varsayilanCihazYerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T720)
	if err := bind.AdliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) > 1 {
		return nil, fmt.Errorf("%s(\"gpu\") en çok 1 parametre bekler, %d verildi", fn, len(args))
	}
	if len(args) == 1 {
		dev, err := kullanilabilirCihaz(fn, args[0])
		if err != nil {
			return nil, err
		}
		if _, err := guvenli(func() bool { tensor.SetDefaultDevice(dev); return true }); err != nil {
			return nil, err
		}
	}
	return &interp.StringVal{Val: tensor.DefaultDevice().String()}, nil
}

// cihazlarYerlesik cihazlar(): kullanılabilir cihaz adları ("cpu" ilk).
func cihazlarYerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T721)
	if err := bind.AdliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, fmt.Errorf("%s() parametre almaz, %d verildi", fn, len(args))
	}
	ad := cihazAdlari()
	el := make([]interp.Value, len(ad))
	for i, a := range ad {
		el[i] = &interp.StringVal{Val: a}
	}
	return &interp.ListVal{Elements: el}, nil
}

// Kapat program sonunda çağrılır (motor.Kapat): GPU ölçümü açıksa (TR_GPU_PROFIL=1) özeti
// standart hataya yazar ve bağlı dış profilleyicilere topladıkları veriyi yazdırır; programın
// kurduğu tohumsuz rastgelelik kancasını kaldırır (aynı süreçte sonraki programa geçmesin).
func Kapat() {
	TohumsuzKancasiAyarla(nil)
	gpu.Kapat()
}

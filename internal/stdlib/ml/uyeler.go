package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"math"
	"slices"
	"strconv"
)

// ----------------------------------------------------------------------------
// Özellikler ve metotlar
// ----------------------------------------------------------------------------

// Tensör üyeleri (adları kod listesinde, internal/lang/codes Y4xx).
const (
	uyeSekil           = codes.Y400
	uyeCihaz           = codes.Y401
	uyeTur             = codes.Y402
	uyeGradyan         = codes.Y403
	uyeDeger           = codes.Y404
	uyeSekillendir     = codes.Y410
	uyeDevrik          = codes.Y411
	uyeEksenleriSirala = codes.Y412
	uyeDilim           = codes.Y413
	uyeCihaza          = codes.Y414
	uyeTure            = codes.Y415
)

// tensorUyeKodlari öneri listesinin sırasıdır.
var tensorUyeKodlari = []codes.Code{uyeSekil, uyeCihaz, uyeTur, uyeGradyan, uyeDeger, uyeSekillendir, uyeDevrik, uyeEksenleriSirala, uyeDilim, uyeCihaza, uyeTure, uyeBol}

var tensorUyeleri = codes.Names(tensorUyeKodlari...)

func tensorUyesi(k codes.Code) bool { return slices.Contains(tensorUyeKodlari, k) }

func (d *Deger) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	t := d.t
	switch kd {
	case uyeSekil:
		return intListesi(t.Shape()), nil
	case uyeCihaz:
		return &interp.StringVal{Val: t.Device().String()}, nil
	case uyeTur:
		return &interp.StringVal{Val: turAdi(t.DType())}, nil
	case uyeTure:
		return d.adliMetot(uyeTure, d.ture), nil
	case uyeGradyan:
		if d.ara {
			return nil, fmt.Errorf("bu tensör (%s) bir işlemin sonucu; gradyan yalnızca %s: %s ile oluşturulan tensörlerde tutulur. Ağırlığı güncellemek için bir iyileştirici (%s, %s) kullanın ya da yeni değeri %s { } içinde hesaplayıp %s ile yeniden oluşturun",
				sekilMetni(t.Shape()), adTurevIzle.TR, codes.Name(codes.A040), codes.Name(codes.T400), codes.Name(codes.T401), codes.Name(codes.T301), turevIzleyenOrnek("..."))
		}
		g := t.Grad()
		if g == nil {
			return interp.NULL, nil
		}
		return Yeni(g), nil
	case uyeDeger:
		return d.degerListesi(), nil
	case uyeSekillendir:
		return d.metot(uyeSekillendir, d.sekillendir), nil
	case uyeDevrik:
		return d.metot(uyeDevrik, func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() argüman almaz", codes.Name(uyeDevrik))
			}
			return devrik(t)
		}), nil
	case uyeEksenleriSirala:
		return d.metot(uyeEksenleriSirala, d.eksenleriSirala), nil
	case uyeDilim:
		return d.metot(uyeDilim, d.dilim), nil
	case uyeCihaza:
		return d.metot(uyeCihaza, d.cihaza), nil
	case uyeBol:
		return d.adliMetot(uyeBol, d.bol), nil
	}
	oneri := ""
	if k, _, eski := codes.FindOld(codes.NamespaceMember, prop, tensorUyesi); eski {
		o := codes.Name(k)
		if k == uyeDevrik {
			o += "()" // metot
		}
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	} else if o := turkish.Closest(prop, tensorUyeleri); o != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return nil, fmt.Errorf("%s üzerinde %q özelliği ya da metodu bulunamadı%s", interp.VAL_TENSOR, prop, oneri)
}

// metot adlı argüman almayan bir tensör metodunun bağlı değeridir; adliMetot adlı
// argümanlarını kendisi denetleyen bir metodunkidir.
func (d *Deger) metot(kd codes.Code, fn interp.BuiltinFunc) *interp.BuiltinVal {
	return &interp.BuiltinVal{Name: codes.Name(kd), Fn: fn, NoNamedArgs: true}
}

func (d *Deger) adliMetot(kd codes.Code, fn interp.BuiltinFunc) *interp.BuiltinVal {
	return &interp.BuiltinVal{Name: codes.Name(kd), Fn: fn}
}

func intListesi(sh []int) *interp.ListVal {
	el := make([]interp.Value, len(sh))
	for i, x := range sh {
		el[i] = interp.NewInt(int64(x))
	}
	return &interp.ListVal{Elements: el}
}

// kisaFloat64 float32 değeri, onu yazan en kısa ondalık gösterime karşılık gelen
// float64'e çevirir: 0.1 (float32) → 0.1, 0.10000000149011612 değil. Böylece
// a.değer[0] == 0.1 doğru olur.
func kisaFloat64(x float32) float64 {
	f := float64(x)
	if f == math.Trunc(f) || math.IsInf(f, 0) || math.IsNaN(f) {
		return f
	}
	var tampon [32]byte
	s := strconv.AppendFloat(tampon[:0], f, 'g', -1, 32)
	if g, err := strconv.ParseFloat(string(s), 64); err == nil {
		return g
	}
	return f
}

// degerListesi tensörün değerlerini Ondalık (0 boyutlu) ya da iç içe liste olarak verir.
// Büyük tensörlerde her liste ve sayı için ayrı ayırma yapmamak için tüm listeler,
// eleman dilimleri ve sayılar önceden ayrılmış üç dilimden dağıtılır. Eleman dilimleri
// kapasiteyle sınırlıdır (a[i:j:j]); listeye eleman eklemek komşu listeyi bozmaz.
func (d *Deger) degerListesi() interp.Value {
	veri := d.t.Data()
	sh := d.t.Shape()
	if len(sh) == 0 {
		if d.tamSayili {
			return interp.NewInt(int64(veri[0]))
		}
		return &interp.FloatVal{Val: kisaFloat64(veri[0])}
	}
	nd := len(sh)
	listeSay, degerSay, p := 0, 0, 1
	for k := 0; k < nd; k++ {
		listeSay += p
		p *= sh[k]
		degerSay += p
	}
	listeler := make([]interp.ListVal, listeSay)
	degerler := make([]interp.Value, degerSay)
	var sayilar []interp.FloatVal
	if !d.tamSayili {
		sayilar = make([]interp.FloatVal, len(veri))
		for i, x := range veri {
			sayilar[i].Val = kisaFloat64(x)
		}
	}
	li, di := 0, 0 // bu seviyenin ilk listesi ve ilk eleman yeri
	p = 1          // bu seviyedeki liste sayısı
	for k := 0; k < nd; k++ {
		n := sh[k]
		sonrakiListe := li + p
		for j := 0; j < p; j++ {
			bas := di + j*n
			el := degerler[bas : bas+n : bas+n]
			for i := range el {
				m := j*n + i // bu seviyedeki elemanın sırası
				if k == nd-1 && d.tamSayili {
					el[i] = interp.NewInt(int64(veri[m]))
				} else if k == nd-1 {
					el[i] = &sayilar[m]
				} else {
					el[i] = &listeler[sonrakiListe+m]
				}
			}
			listeler[li+j].Elements = el
		}
		li = sonrakiListe
		di += p * n
		p *= n
	}
	return &listeler[0]
}

func (d *Deger) sekillendir(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	var hedef []interp.Value
	if len(args) == 1 {
		if l, ok := args[0].(*interp.ListVal); ok {
			hedef = l.Elements
		}
	}
	if hedef == nil {
		hedef = args
	}
	if len(hedef) == 0 {
		return nil, fmt.Errorf("%s([4, 6]) yeni şekli liste olarak bekler", codes.Name(uyeSekillendir))
	}
	sh := make([]int, len(hedef))
	bilinmeyen := 0
	for i, v := range hedef {
		n, ok := tamSayi(v)
		if !ok || n < -1 {
			return nil, fmt.Errorf("%s: şeklin %d. elemanı negatif olmayan bir tam sayı (ya da çıkarsanacak eksen için -1) olmalıdır, %s verildi", codes.Name(uyeSekillendir), i+1, v.Inspect())
		}
		if n == -1 {
			bilinmeyen++
		}
		sh[i] = n
	}
	if bilinmeyen > 1 {
		return nil, fmt.Errorf("%s: yalnızca bir eksen -1 olabilir, %s verildi", codes.Name(uyeSekillendir), sekilMetni(sh))
	}
	n := d.t.Numel()
	carpim := 1
	for _, x := range sh {
		if x != -1 {
			carpim *= x
		}
	}
	if (bilinmeyen == 0 && carpim != n) || (bilinmeyen == 1 && (carpim == 0 || n%carpim != 0)) {
		return nil, fmt.Errorf("%s şeklindeki tensör (%d eleman) %s şekline sokulamaz", sekilMetni(d.t.Shape()), n, sekilMetni(sh))
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.Reshape(sh...) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func devrik(t *tensor.Tensor) (interp.Value, error) {
	sh := t.Shape()
	if len(sh) < 2 {
		return nil, fmt.Errorf("%s() en az 2 boyutlu tensör ister, %s verildi", codes.Name(uyeDevrik), sekilMetni(sh))
	}
	r, err := guvenli(func() *tensor.Tensor { return t.T() })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func (d *Deger) eksenleriSirala(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	var liste []interp.Value
	if len(args) == 1 {
		if l, ok := args[0].(*interp.ListVal); ok {
			liste = l.Elements
		}
	}
	if liste == nil {
		liste = args
	}
	fn := codes.Name(uyeEksenleriSirala)
	sh := d.t.Shape()
	if len(liste) != len(sh) {
		return nil, fmt.Errorf("%s: %s şeklindeki %d boyutlu tensör için %d eksen verilmeli, %d verildi", fn, sekilMetni(sh), len(sh), len(sh), len(liste))
	}
	dims := make([]int, len(liste))
	gorulen := make([]bool, len(sh))
	for i, v := range liste {
		n, ok := tamSayi(v)
		if !ok {
			return nil, fmt.Errorf("%s: eksenler tam sayı olmalıdır, %s verildi", fn, v.Inspect())
		}
		e, err := eksenCoz(fn, n, sh)
		if err != nil {
			return nil, err
		}
		if gorulen[e] {
			return nil, fmt.Errorf("%s: %d. eksen birden çok kez verildi; her eksen bir kez yer almalı", fn, e)
		}
		gorulen[e] = true
		dims[i] = e
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.Permute(dims...) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func (d *Deger) dilim(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(uyeDilim)
	if len(args) != 3 {
		return nil, fmt.Errorf("%s(eksen, baş, son) 3 parametre bekler, %d verildi", fn, len(args))
	}
	var n [3]int
	for i, a := range args {
		x, ok := tamSayi(a)
		if !ok {
			return nil, fmt.Errorf("%s(eksen, baş, son) tam sayılar bekler, %s verildi", fn, a.Inspect())
		}
		n[i] = x
	}
	sh := d.t.Shape()
	e, err := eksenCoz(fn, n[0], sh)
	if err != nil {
		return nil, err
	}
	bas, son := n[1], n[2]
	uz := sh[e]
	if bas < 0 {
		bas += uz
	}
	if son < 0 {
		son += uz
	}
	if bas < 0 || bas > uz || son < bas || son > uz {
		return nil, fmt.Errorf("%s: %d. eksenin uzunluğu %d (%s); [%d, %d) aralığı geçersiz (0 ≤ baş ≤ son ≤ %d olmalı)", fn, e, uz, sekilMetni(sh), n[1], n[2], uz)
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.Slice(e, bas, son) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

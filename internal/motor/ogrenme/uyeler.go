package ogrenme

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
)

// ----------------------------------------------------------------------------
// Özellikler ve metotlar
// ----------------------------------------------------------------------------

// Tensör üyeleri (adları kod listesinde, internal/dil/kod Y4xx).
const (
	uyeSekil           = kod.Y400
	uyeCihaz           = kod.Y401
	uyeTur             = kod.Y402
	uyeGradyan         = kod.Y403
	uyeDeger           = kod.Y404
	uyeSekillendir     = kod.Y410
	uyeDevrik          = kod.Y411
	uyeEksenleriSirala = kod.Y412
	uyeDilim           = kod.Y413
	uyeCihaza          = kod.Y414
	uyeTure            = kod.Y415
)

// tensorUyeKodlari öneri listesinin sırasıdır.
var tensorUyeKodlari = []kod.Kod{uyeSekil, uyeCihaz, uyeTur, uyeGradyan, uyeDeger, uyeSekillendir, uyeDevrik, uyeEksenleriSirala, uyeDilim, uyeCihaza, uyeTure, uyeBol}

var tensorUyeleri = kod.Adlar(tensorUyeKodlari...)

func tensorUyesi(k kod.Kod) bool { return slices.Contains(tensorUyeKodlari, k) }

func (d *Deger) GetProperty(kd kod.Kod, prop string) (yurut.Value, error) {
	t := d.t
	switch kd {
	case uyeSekil:
		return intListesi(t.Shape()), nil
	case uyeCihaz:
		return &yurut.StringVal{Val: t.Device().String()}, nil
	case uyeTur:
		return &yurut.StringVal{Val: turAdi(t.DType())}, nil
	case uyeTure:
		return d.adliMetot(uyeTure, d.ture), nil
	case uyeGradyan:
		if d.ara {
			return nil, fmt.Errorf("bu tensör (%s) bir işlemin sonucu; gradyan yalnızca %s: %s ile oluşturulan tensörlerde tutulur. Ağırlığı güncellemek için bir iyileştirici (%s, %s) kullanın ya da yeni değeri %s { } içinde hesaplayıp %s ile yeniden oluşturun",
				sekilMetni(t.Shape()), adTurevIzle.TR, kod.Ad(kod.A040), kod.Ad(kod.T400), kod.Ad(kod.T401), kod.Ad(kod.T301), turevIzleyenOrnek("..."))
		}
		g := t.Grad()
		if g == nil {
			return yurut.NULL, nil
		}
		return Yeni(g), nil
	case uyeDeger:
		return d.degerListesi(), nil
	case uyeSekillendir:
		return d.metot(uyeSekillendir, d.sekillendir), nil
	case uyeDevrik:
		return d.metot(uyeDevrik, func(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() argüman almaz", kod.Ad(uyeDevrik))
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
	if k, _, eski := kod.EskiBul(kod.AlanUye, prop, tensorUyesi); eski {
		o := kod.Ad(k)
		if k == uyeDevrik {
			o += "()" // metot
		}
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	} else if o := turkce.EnYakin(prop, tensorUyeleri); o != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return nil, fmt.Errorf("%s üzerinde %q özelliği ya da metodu bulunamadı%s", yurut.VAL_TENSOR, prop, oneri)
}

// metot adlı argüman almayan bir tensör metodunun bağlı değeridir; adliMetot adlı
// argümanlarını kendisi denetleyen bir metodunkidir.
func (d *Deger) metot(kd kod.Kod, fn yurut.BuiltinFunc) *yurut.BuiltinVal {
	return &yurut.BuiltinVal{Name: kod.Ad(kd), Fn: fn, AdliAlmaz: true}
}

func (d *Deger) adliMetot(kd kod.Kod, fn yurut.BuiltinFunc) *yurut.BuiltinVal {
	return &yurut.BuiltinVal{Name: kod.Ad(kd), Fn: fn}
}

func intListesi(sh []int) *yurut.ListVal {
	el := make([]yurut.Value, len(sh))
	for i, x := range sh {
		el[i] = yurut.NewInt(int64(x))
	}
	return &yurut.ListVal{Elements: el}
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
func (d *Deger) degerListesi() yurut.Value {
	veri := d.t.Data()
	sh := d.t.Shape()
	if len(sh) == 0 {
		if d.tamSayili {
			return yurut.NewInt(int64(veri[0]))
		}
		return &yurut.FloatVal{Val: kisaFloat64(veri[0])}
	}
	nd := len(sh)
	listeSay, degerSay, p := 0, 0, 1
	for k := 0; k < nd; k++ {
		listeSay += p
		p *= sh[k]
		degerSay += p
	}
	listeler := make([]yurut.ListVal, listeSay)
	degerler := make([]yurut.Value, degerSay)
	var sayilar []yurut.FloatVal
	if !d.tamSayili {
		sayilar = make([]yurut.FloatVal, len(veri))
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
					el[i] = yurut.NewInt(int64(veri[m]))
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

func (d *Deger) sekillendir(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
	var hedef []yurut.Value
	if len(args) == 1 {
		if l, ok := args[0].(*yurut.ListVal); ok {
			hedef = l.Elements
		}
	}
	if hedef == nil {
		hedef = args
	}
	if len(hedef) == 0 {
		return nil, fmt.Errorf("%s([4, 6]) yeni şekli liste olarak bekler", kod.Ad(uyeSekillendir))
	}
	sh := make([]int, len(hedef))
	bilinmeyen := 0
	for i, v := range hedef {
		n, ok := tamSayi(v)
		if !ok || n < -1 {
			return nil, fmt.Errorf("%s: şeklin %d. elemanı negatif olmayan bir tam sayı (ya da çıkarsanacak eksen için -1) olmalıdır, %s verildi", kod.Ad(uyeSekillendir), i+1, v.Inspect())
		}
		if n == -1 {
			bilinmeyen++
		}
		sh[i] = n
	}
	if bilinmeyen > 1 {
		return nil, fmt.Errorf("%s: yalnızca bir eksen -1 olabilir, %s verildi", kod.Ad(uyeSekillendir), sekilMetni(sh))
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

func devrik(t *tensor.Tensor) (yurut.Value, error) {
	sh := t.Shape()
	if len(sh) < 2 {
		return nil, fmt.Errorf("%s() en az 2 boyutlu tensör ister, %s verildi", kod.Ad(uyeDevrik), sekilMetni(sh))
	}
	r, err := guvenli(func() *tensor.Tensor { return t.T() })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func (d *Deger) eksenleriSirala(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
	var liste []yurut.Value
	if len(args) == 1 {
		if l, ok := args[0].(*yurut.ListVal); ok {
			liste = l.Elements
		}
	}
	if liste == nil {
		liste = args
	}
	fn := kod.Ad(uyeEksenleriSirala)
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

func (d *Deger) dilim(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(uyeDilim)
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

// cihazCoz "cpu", "gpu", "gpu:1" gibi bir adı cihaza çevirir.
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

func (d *Deger) cihaza(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(\"cpu\") 1 parametre bekler", kod.Ad(uyeCihaza))
	}
	s, ok := args[0].(*yurut.StringVal)
	if !ok {
		return nil, fmt.Errorf("%s() cihaz adını metin olarak bekler (\"cpu\" ya da \"gpu\"), %s verildi", kod.Ad(uyeCihaza), tipAdi(args[0]))
	}
	dev, err := cihazCoz(s.Val)
	if err != nil {
		return nil, err
	}
	if !tensor.HasDevice(dev) {
		return nil, fmt.Errorf("%s cihazı kullanılamıyor: bu TR derlemesinde GPU arka ucu yok; tensörler şimdilik yalnızca \"cpu\" üzerinde çalışır", dev)
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.To(dev) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

package ml

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bu dosya kullanıcı tanımlı türevi (özel_türev) ve sayısal gradyan denetimini
// (türev_denetle) TR'ye bağlar.
//
//	değişken kare = özel_türev(
//	    ileri: fonksiyon(x) { sonuç [x * x, [x]] },        // [çıktı, saklananlar]
//	    geri: fonksiyon(saklananlar, g) { sonuç [g * 2 * saklananlar[0]] }
//	)
//
// ileri türevsiz çalışır ve [çıktı, saklananlar] döndürür; çıktı bir tensör ya da tensör
// listesidir, saklananlar geri hesabın ihtiyaç duyacağı değerlerin listesidir. Girdilerden
// biri türev izliyorsa çıktı(lar) çekirdekte tek bir dış düğüm olarak kaydedilir (bkz.
// github.com/sekerahmet/TCB/internal/compute/tensor CustomOp); geri yayılımda geri(saklananlar, g) türevsiz çalışır ve her
// konumlu girdi için bir gradyan (ya da yok) döndürür. Çok çıktıda g her çıktı için bir
// gradyan içeren listedir; kayba bağlı olmayan çıktının gradyanı sıfır tensördür (geri
// hesap 'yok' denetimi yapmadan aritmetik kullanabilsin diye).
//
// geri, geri_yayılım'ı çağıran görevin VM'iyle (yorumlayıcı kilidini tutan görev) çalışır:
// görev, çekirdeğe BackwardCtx ile bağlam olarak verilir.

var (
	adIleri    = bind.NamedArg(codes.T980)
	adGeri     = bind.NamedArg(codes.T981)
	adAdim     = bind.NamedArg(codes.T982)
	adTolerans = bind.NamedArg(codes.T983)
)

// ozelTurevKaydet özel_türev ve türev_denetle yerleşiklerini kaydeder (bkz. Kaydet).
func ozelTurevKaydet(v *interp.VM) {
	sar(v, codes.T310, yeniYerlesik(ozelTurev))
	sar(v, codes.T311, yeniYerlesik(turevDenetle))
}

// ozelTurevOrnek hata mesajlarındaki kullanım örneğidir; adlar koddan kurulur.
var ozelTurevOrnek = fmt.Sprintf("%s(%s: %s(x) { %s [x * x, [x]] }, %s: %s(saklananlar, g) { %s [g * 2 * saklananlar[0]] })",
	codes.Name(codes.T310), adIleri.TR, codes.Name(codes.A020), codes.Name(codes.A021), adGeri.TR, codes.Name(codes.A020), codes.Name(codes.A021))

// geriHatasi, geri yayılım sırasında (çekirdeğin içinden) kullanıcının geri hesabında
// oluşan hatayı taşır; geriYay paniği yakalayıp hatayı olduğu gibi döndürür.
type geriHatasi struct{ err error }

// geriYay t'den geri yayılım yapar; dış düğümlerin geri hesapları m göreviyle çalışır.
// Çekirdek paniği Türkçe bir hataya, kullanıcının geri hesabındaki hata ise olduğu gibi
// (satır bilgisiyle) döner.
func geriYay(m *interp.VM, t, tohum *tensor.Tensor) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if interp.IsInterrupt(r) {
				panic(r) // kesilme (Ctrl+C) hata değildir; yığın çözülmeye devam eder
			}
			if g, ok := r.(geriHatasi); ok {
				err = g.err
				return
			}
			err = panikHatasi(r)
		}
	}()
	t.BackwardCtx(tohum, m)
	return nil
}

// cagrilabilirMi değer bir fonksiyon (TR ya da yerleşik) mi?
func cagrilabilirMi(v interp.Value) bool {
	switch v.(type) {
	case *interp.FuncVal, *interp.BuiltinVal:
		return true
	}
	return false
}

// ozelIslem özel_türev'in döndürdüğü çağrılabilir değerin durumudur.
type ozelIslem struct {
	ileri, geri interp.Value
}

// ozelTurev özel_türev(ileri:, geri:) yerleşiğidir.
func ozelTurev(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T310)
	if err := bind.CheckNamed(fn+"()", named, adIleri, adGeri); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, fmt.Errorf("%s yalnızca adlı argüman alır (%s: ve %s:), %d konumlu parametre verildi; örnek: %s", fn, adIleri.TR, adGeri.TR, len(args), ozelTurevOrnek)
	}
	ileri, ok1 := bind.GetNamed(named, adIleri)
	geri, ok2 := bind.GetNamed(named, adGeri)
	if !ok1 || !ok2 {
		eksik := adIleri.TR + ":"
		if ok1 {
			eksik = adGeri.TR + ":"
		} else if !ok2 {
			eksik = adIleri.TR + ": ve " + adGeri.TR + ":"
		}
		return nil, fmt.Errorf("%s: %s eksik; örnek: %s", fn, eksik, ozelTurevOrnek)
	}
	if !cagrilabilirMi(ileri) {
		return nil, fmt.Errorf("%s: %s bir fonksiyon olmalıdır, %s verildi", fn, adIleri.TR, tipAdi(ileri))
	}
	if !cagrilabilirMi(geri) {
		return nil, fmt.Errorf("%s: %s bir fonksiyon olmalıdır, %s verildi", fn, adGeri.TR, tipAdi(geri))
	}
	if f, ok := geri.(*interp.FuncVal); ok {
		gerekli := 0
		for _, p := range f.Parameters {
			if p.DefaultValue == nil {
				gerekli++
			}
		}
		if len(f.Parameters) < 2 || gerekli > 2 {
			return nil, fmt.Errorf("%s: %s iki parametre almalıdır: %s(saklananlar, g), %d parametreli fonksiyon verildi", fn, adGeri.TR, codes.Name(codes.A020), len(f.Parameters))
		}
	}
	o := &ozelIslem{ileri: ileri, geri: geri}
	return &interp.BuiltinVal{Name: fn, Fn: o.cagir}, nil
}

// cagir özel işlemi çalıştırır: ileri türevsiz çalışır, sonuç tek dış düğümle çizgeye
// bağlanır. Adlı argümanlar ileri'ye olduğu gibi geçer.
func (o *ozelIslem) cagir(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	args = append([]interp.Value(nil), args...)
	girdiler := make([]*tensor.Tensor, len(args))
	for i, a := range args {
		if d, ok := a.(*Deger); ok {
			girdiler[i] = d.t
		}
	}
	var sonuc interp.Value
	var err error
	tensor.NoGrad(func() { sonuc, err = m.ApplyFunction(o.ileri, args, named, 0) })
	if err != nil {
		return nil, err
	}
	ciktilar, cok, saklananlar, err := ileriSonucu(sonuc)
	if err != nil {
		return nil, err
	}
	var saklanan []*tensor.Tensor
	tensorleriTopla(saklananlar, &saklanan, 0)
	outs, err := guvenli(func() []*tensor.Tensor {
		return tensor.CustomOp(codes.Name(codes.T310), girdiler, ciktilar, saklanan, o.geriHesabi(m, args, saklananlar, cok))
	})
	if err != nil {
		return nil, err
	}
	if !cok {
		return Yeni(outs[0]), nil
	}
	el := make([]interp.Value, len(outs))
	for i, t := range outs {
		el[i] = Yeni(t)
	}
	return &interp.ListVal{Elements: el}, nil
}

// ileriSonucu ileri'nin [çıktı, saklananlar] dönüşünü çözer.
func ileriSonucu(v interp.Value) (ciktilar []*tensor.Tensor, cok bool, saklananlar *interp.ListVal, err error) {
	fn, sonuc := codes.Name(codes.T310), codes.Name(codes.A021)
	ilerinin := turkish.Suffix(adIleri.TR, turkish.Genitive)
	l, ok := v.(*interp.ListVal)
	if !ok || len(l.Elements) != 2 {
		return nil, false, nil, fmt.Errorf("%s: %s [çıktı, saklananlar] döndürmeli (ör. %s [x * x, [x]]; saklanacak bir şey yoksa %s [y, []]), %s verildi", fn, adIleri.TR, sonuc, sonuc, kisaGoster(v))
	}
	saklananlar, ok = l.Elements[1].(*interp.ListVal)
	if !ok {
		return nil, false, nil, fmt.Errorf("%s: %s döndürdüğü [çıktı, saklananlar] içinde saklananlar bir liste olmalıdır (ör. [x] ya da []), %s verildi", fn, ilerinin, tipAdi(l.Elements[1]))
	}
	switch c := l.Elements[0].(type) {
	case *Deger:
		return []*tensor.Tensor{c.t}, false, saklananlar, nil
	case *interp.ListVal:
		if len(c.Elements) == 0 {
			return nil, false, nil, fmt.Errorf("%s: %s çıktı listesi boş; en az bir tensör döndürün", fn, ilerinin)
		}
		for i, e := range c.Elements {
			d, ok := e.(*Deger)
			if !ok {
				return nil, false, nil, fmt.Errorf("%s: %s çıktı listesinin %d. öğesi tensör olmalıdır, %s verildi", fn, ilerinin, i+1, tipAdi(e))
			}
			ciktilar = append(ciktilar, d.t)
		}
		return ciktilar, true, saklananlar, nil
	}
	return nil, false, nil, fmt.Errorf("%s: %s çıktısı bir tensör ya da tensör listesi olmalıdır, %s verildi", fn, ilerinin, tipAdi(l.Elements[0]))
}

// kisaGoster hata mesajı için değerin tipini ve (kısaysa) kendisini yazar.
func kisaGoster(v interp.Value) string {
	if v == nil {
		return interp.NULL.Inspect()
	}
	s := v.Inspect()
	if len(s) > 60 || strings.Contains(s, "\n") {
		return tipAdi(v)
	}
	return fmt.Sprintf("%s (%s)", s, tipAdi(v))
}

// tensorleriTopla saklananlar içindeki (iç içe listeler dahil) tensörleri toplar; bunların
// sürümleri kaydedilir ve geri yayılımdan önce yerinde değiştirilmedikleri denetlenir.
func tensorleriTopla(v interp.Value, out *[]*tensor.Tensor, derinlik int) {
	if derinlik > 16 {
		return
	}
	switch x := v.(type) {
	case *Deger:
		*out = append(*out, x.t)
	case *interp.ListVal:
		for _, e := range x.Elements {
			tensorleriTopla(e, out, derinlik+1)
		}
	}
}

// geriHesabi çekirdeğin dış düğümüne verilen geri çağrıyı kurar. Bağlam (ctx) geri
// yayılımı başlatan görevin VM'idir; verilmemişse (Go'dan doğrudan geri yayılım) ileri
// hesabı yapan görev kullanılır.
func (o *ozelIslem) geriHesabi(ileriVM *interp.VM, args []interp.Value, saklananlar *interp.ListVal, cok bool) tensor.CustomBackward {
	return func(ctx any, grads []*tensor.Tensor) []*tensor.Tensor {
		m, _ := ctx.(*interp.VM)
		if m == nil {
			m = ileriVM
		}
		var g interp.Value
		if cok {
			el := make([]interp.Value, len(grads))
			for i, t := range grads {
				el[i] = Yeni(t)
			}
			g = &interp.ListVal{Elements: el}
		} else {
			g = Yeni(grads[0])
		}
		sonuc, err := m.ApplyFunction(o.geri, []interp.Value{saklananlar, g}, nil, 0)
		if err != nil {
			panic(geriHatasi{err})
		}
		gs, err := geriSonucu(sonuc, args)
		if err != nil {
			panic(geriHatasi{err})
		}
		return gs
	}
}

// geriSonucu geri'nin döndürdüğü gradyan listesini denetler: her konumlu girdi için bir
// öğe; tensör girdilerde girdiyle aynı şekilde bir tensör ya da yok, tensör olmayan
// girdilerde yok.
func geriSonucu(v interp.Value, args []interp.Value) ([]*tensor.Tensor, error) {
	fn, geri, yok := codes.Name(codes.T310), adGeri.TR, codes.Name(codes.A042)
	l, ok := v.(*interp.ListVal)
	if !ok {
		return nil, fmt.Errorf("%s: %s her girdi için bir gradyan (ya da %s) içeren bir liste döndürmeli (ör. %s [g * 2 * x]), %s verildi", fn, geri, yok, codes.Name(codes.A021), kisaGoster(v))
	}
	if len(l.Elements) != len(args) {
		return nil, fmt.Errorf("%s: %s %d öğeli liste döndürdü ama işlemin %d konumlu girdisi var; her girdi için sırasıyla bir gradyan ya da %s verin", fn, geri, len(l.Elements), len(args), yok)
	}
	out := make([]*tensor.Tensor, len(args))
	for i, e := range l.Elements {
		if _, yok := e.(*interp.NullVal); yok {
			continue
		}
		d, ok := e.(*Deger)
		if !ok {
			return nil, fmt.Errorf("%s: %s %d. öğesi (%d. girdinin gradyanı) tensör ya da %s olmalıdır, %s verildi", fn, turkish.Suffix(geri, turkish.Genitive), i+1, i+1, yok, tipAdi(e))
		}
		girdi, ok := args[i].(*Deger)
		if !ok {
			return nil, fmt.Errorf("%s: %d. girdi tensör değil (%s); %s bu girdi için %s döndürmeli", fn, i+1, tipAdi(args[i]), geri, yok)
		}
		if gs, is := d.t.Shape(), girdi.t.Shape(); !sekilEsitMi(gs, is) {
			return nil, fmt.Errorf("%s: %s %d. girdi için %s şeklinde gradyan döndürdü, beklenen %s (girdinin şekli); gradyan girdiyle aynı şekilde olmalı, yayınlama yapılmaz", fn, geri, i+1, sekilMetni(gs), sekilMetni(is))
		}
		out[i] = d.t
	}
	return out, nil
}

func sekilEsitMi(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------------
// türev_denetle
// ----------------------------------------------------------------------------

// turevDenetle türev_denetle(f, girdiler, adım: 0.001, tolerans: 0.01) yerleşiğidir:
// f'nin geri yayılımla bulunan gradyanlarını merkezi farkla bulunan sayısal gradyanlarla
// karşılaştırır. Denetim türev izleyen girdilerin float32 kopyalarıyla yapılır (kullanıcının
// tensörleri ve gradyanları değişmez). Skaler olmayan çıktılar sabit tohumlu rastgele bir
// yön vektörüyle iç çarpımla skalere indirgenir; kayıp ve farklar float64'te hesaplanır.
// Eleman başına koşul |analitik - sayısal| ≤ tolerans · max(1, |analitik|, |sayısal|).
func turevDenetle(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T311)
	if err := bind.CheckNamed(fn+"()", named, adAdim, adTolerans); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, fmt.Errorf("%s(f, girdiler, %s: 0.001, %s: 0.01) 2 konumlu parametre bekler, %d verildi", fn, adAdim.TR, adTolerans.TR, len(args))
	}
	f := args[0]
	if !cagrilabilirMi(f) {
		return nil, fmt.Errorf("%s: ilk parametre bir fonksiyon olmalıdır, %s verildi", fn, tipAdi(f))
	}
	liste, ok := args[1].(*interp.ListVal)
	if !ok {
		return nil, fmt.Errorf("%s: girdiler bir liste olmalıdır (ör. [%s([[0.3, -1.2]], %s: %s)]), %s verildi", fn, codes.Name(codes.T100), codes.Name(codes.T900), codes.Name(codes.A040), tipAdi(args[1]))
	}
	adim, err := pozitifSayi(fn, named, adAdim, 1e-3)
	if err != nil {
		return nil, err
	}
	tol, err := pozitifSayi(fn, named, adTolerans, 1e-2)
	if err != nil {
		return nil, err
	}
	if !tensor.GradEnabled() {
		return nil, fmt.Errorf("%s %s { ... } bloğu içinde çalışamaz", fn, codes.Name(codes.T301))
	}

	// Türev izleyen girdilerin float32 yaprak kopyaları.
	girdiler := make([]interp.Value, len(liste.Elements))
	kopyalar := make([]*tensor.Tensor, len(liste.Elements))
	denetlenen := 0
	for i, e := range liste.Elements {
		girdiler[i] = e
		if d, ok := e.(*Deger); ok && d.t.RequiresGrad() {
			c := tensor.New(d.t.Data(), d.t.Shape()...).SetRequiresGrad(true)
			kopyalar[i] = c
			girdiler[i] = Yeni(c)
			denetlenen++
		}
	}
	if denetlenen == 0 {
		return nil, fmt.Errorf("%s: girdilerden hiçbiri türev izlemiyor; denetlenecek tensörleri %s(..., %s: %s) ile oluşturun", fn, codes.Name(codes.T100), codes.Name(codes.T900), codes.Name(codes.A040))
	}

	// Analitik gradyan: L = Σ_k <v_k, y_k> üzerinden geri yayılım.
	sonuc, err := m.ApplyFunction(f, girdiler, nil, 0)
	if err != nil {
		return nil, err
	}
	ciktilar, err := denetimCiktilari(fn, sonuc)
	if err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewSource(20240917))
	yonler := make([][]float64, len(ciktilar))
	for k, y := range ciktilar {
		yonler[k] = make([]float64, y.Numel())
		for j := range yonler[k] {
			if len(ciktilar) == 1 && y.Numel() == 1 {
				yonler[k][j] = 1
			} else {
				yonler[k][j] = rng.NormFloat64()
			}
		}
	}
	var kayip *tensor.Tensor
	if _, err := guvenli(func() bool {
		for k, y := range ciktilar {
			if !y.RequiresGrad() {
				continue
			}
			v := tensor.FromFloat64(yonler[k], y.Shape()...)
			p := y.Cast(tensor.Float32).Mul(v).SumAll()
			if kayip == nil {
				kayip = p
			} else {
				kayip = kayip.Add(p)
			}
		}
		return true
	}); err != nil {
		return nil, err
	}
	if kayip != nil {
		if err := geriYay(m, kayip, nil); err != nil {
			return nil, err
		}
	}
	analitik := make([][]float64, len(kopyalar))
	for i, c := range kopyalar {
		if c == nil {
			continue
		}
		if g := c.Grad(); g != nil {
			analitik[i] = g.Float64s()
		} else {
			analitik[i] = make([]float64, c.Numel())
		}
	}

	// Sayısal gradyan: merkezi fark, kayıp float64'te.
	sayisalKayip := func() (float64, error) {
		var s interp.Value
		var err error
		tensor.NoGrad(func() { s, err = m.ApplyFunction(f, girdiler, nil, 0) })
		if err != nil {
			return 0, err
		}
		ys, err := denetimCiktilari(fn, s)
		if err != nil {
			return 0, err
		}
		if len(ys) != len(yonler) {
			return 0, fmt.Errorf("%s: f her çağrıda aynı sayıda çıktı vermeli", fn)
		}
		var top float64
		for k, y := range ys {
			if y.Numel() != len(yonler[k]) {
				return 0, fmt.Errorf("%s: f her çağrıda aynı şekilde çıktı vermeli", fn)
			}
			for j, x := range y.Data() {
				top += float64(x) * yonler[k][j]
			}
		}
		return top, nil
	}
	type enKotu struct {
		girdi      int
		indeks     []int
		an, say    float64
		oran, fark float64
		bulundu    bool
	}
	var kotu enKotu
	for i, c := range kopyalar {
		if c == nil {
			continue
		}
		sh := c.Shape()
		veri := c.Data()
		idx := make([]int, len(sh))
		for j, orig := range veri {
			duzIndeks(j, sh, idx)
			xp, xm := orig+float32(adim), orig-float32(adim)
			if err := setGuvenli(c, xp, idx); err != nil {
				return nil, err
			}
			lp, err := sayisalKayip()
			if err == nil {
				if err = setGuvenli(c, xm, idx); err == nil {
					var lm float64
					lm, err = sayisalKayip()
					if err == nil {
						num := (lp - lm) / (float64(xp) - float64(xm))
						an := analitik[i][j]
						fark := math.Abs(an - num)
						oran := fark / math.Max(1, math.Max(math.Abs(an), math.Abs(num)))
						if math.IsNaN(oran) {
							oran = math.Inf(1)
						}
						if !kotu.bulundu || oran > kotu.oran {
							kotu = enKotu{girdi: i, indeks: append([]int(nil), idx...), an: an, say: num, oran: oran, fark: fark, bulundu: true}
						}
					}
				}
			}
			if serr := setGuvenli(c, orig, idx); err == nil {
				err = serr
			}
			if err != nil {
				return nil, err
			}
		}
	}
	if kotu.oran > tol {
		return nil, fmt.Errorf("%s: analitik (geri yayılım) ve sayısal gradyan uyuşmuyor; en büyük fark %d. girdinin %s elemanında: analitik %.6g, sayısal %.6g (fark %.3g, tolerans %g); geri hesabını denetleyin",
			fn, kotu.girdi+1, sekilMetni(kotu.indeks), kotu.an, kotu.say, kotu.fark, tol)
	}
	return interp.TRUE, nil
}

// denetimCiktilari f'nin dönüşünü tensör listesine çevirir (tek tensör ya da tensör listesi).
func denetimCiktilari(fn string, v interp.Value) ([]*tensor.Tensor, error) {
	switch x := v.(type) {
	case *Deger:
		return []*tensor.Tensor{x.t}, nil
	case *interp.ListVal:
		if len(x.Elements) == 0 {
			return nil, fmt.Errorf("%s: f boş liste döndürdü; bir tensör ya da tensör listesi döndürmeli", fn)
		}
		out := make([]*tensor.Tensor, len(x.Elements))
		for i, e := range x.Elements {
			d, ok := e.(*Deger)
			if !ok {
				return nil, fmt.Errorf("%s: f'nin döndürdüğü listenin %d. öğesi tensör olmalıdır, %s verildi", fn, i+1, tipAdi(e))
			}
			out[i] = d.t
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s: f bir tensör ya da tensör listesi döndürmeli, %s verildi", fn, kisaGoster(v))
}

// pozitifSayi adlı pozitif sayısal argümanı okur.
func pozitifSayi(fn string, named map[string]interp.Value, ad bind.Named, varsayilan float64) (float64, error) {
	v, ok := bind.GetNamed(named, ad)
	if !ok {
		return varsayilan, nil
	}
	f, ok := interp.ToFloat(v)
	if !ok || !(f > 0) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%s: %s pozitif bir sayı olmalıdır, %s verildi", fn, ad.TR, v.Inspect())
	}
	return f, nil
}

// duzIndeks satır öncelikli düz indeksi çok boyutlu indekse çevirir.
func duzIndeks(j int, sh []int, idx []int) {
	for a := len(sh) - 1; a >= 0; a-- {
		idx[a] = j % sh[a]
		j /= sh[a]
	}
}

func setGuvenli(t *tensor.Tensor, v float32, idx []int) error {
	_, err := guvenli(func() bool { t.Set(v, idx...); return true })
	return err
}

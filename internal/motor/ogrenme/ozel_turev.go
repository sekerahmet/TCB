package ogrenme

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/ortak/baglama"
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
// tr_dili/internal/hesap/tensor CustomOp); geri yayılımda geri(saklananlar, g) türevsiz çalışır ve her
// konumlu girdi için bir gradyan (ya da yok) döndürür. Çok çıktıda g her çıktı için bir
// gradyan içeren listedir; kayba bağlı olmayan çıktının gradyanı sıfır tensördür (geri
// hesap 'yok' denetimi yapmadan aritmetik kullanabilsin diye).
//
// geri, geri_yayılım'ı çağıran görevin VM'iyle (yorumlayıcı kilidini tutan görev) çalışır:
// görev, çekirdeğe BackwardCtx ile bağlam olarak verilir.

var (
	adIleri    = baglama.KodAd(kod.T980)
	adGeri     = baglama.KodAd(kod.T981)
	adAdim     = baglama.KodAd(kod.T982)
	adTolerans = baglama.KodAd(kod.T983)
)

// ozelTurevKaydet özel_türev ve türev_denetle yerleşiklerini kaydeder (bkz. Kaydet).
func ozelTurevKaydet(v *yurut.VM) {
	sar(v, kod.T310, yeniYerlesik(ozelTurev))
	sar(v, kod.T311, yeniYerlesik(turevDenetle))
}

const ozelTurevOrnek = "özel_türev(ileri: fonksiyon(x) { sonuç [x * x, [x]] }, geri: fonksiyon(saklananlar, g) { sonuç [g * 2 * saklananlar[0]] })"

// geriHatasi, geri yayılım sırasında (çekirdeğin içinden) kullanıcının geri hesabında
// oluşan hatayı taşır; geriYay paniği yakalayıp hatayı olduğu gibi döndürür.
type geriHatasi struct{ err error }

// geriYay t'den geri yayılım yapar; dış düğümlerin geri hesapları m göreviyle çalışır.
// Çekirdek paniği Türkçe bir hataya, kullanıcının geri hesabındaki hata ise olduğu gibi
// (satır bilgisiyle) döner.
func geriYay(m *yurut.VM, t, tohum *tensor.Tensor) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if yurut.KesilmeMi(r) {
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
func cagrilabilirMi(v yurut.Value) bool {
	switch v.(type) {
	case *yurut.FuncVal, *yurut.BuiltinVal:
		return true
	}
	return false
}

// ozelIslem özel_türev'in döndürdüğü çağrılabilir değerin durumudur.
type ozelIslem struct {
	ileri, geri yurut.Value
}

// ozelTurev özel_türev(ileri:, geri:) yerleşiğidir.
func ozelTurev(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	const fn = "özel_türev"
	if err := baglama.AdliDenetle(fn+"()", named, adIleri, adGeri); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, fmt.Errorf("%s yalnızca adlı argüman alır (ileri: ve geri:), %d konumlu parametre verildi; örnek: %s", fn, len(args), ozelTurevOrnek)
	}
	ileri, ok1 := baglama.AdliAl(named, adIleri)
	geri, ok2 := baglama.AdliAl(named, adGeri)
	if !ok1 || !ok2 {
		eksik := "ileri:"
		if ok1 {
			eksik = "geri:"
		} else if !ok2 {
			eksik = "ileri: ve geri:"
		}
		return nil, fmt.Errorf("%s: %s eksik; örnek: %s", fn, eksik, ozelTurevOrnek)
	}
	if !cagrilabilirMi(ileri) {
		return nil, fmt.Errorf("%s: ileri bir fonksiyon olmalıdır, %s verildi", fn, tipAdi(ileri))
	}
	if !cagrilabilirMi(geri) {
		return nil, fmt.Errorf("%s: geri bir fonksiyon olmalıdır, %s verildi", fn, tipAdi(geri))
	}
	if f, ok := geri.(*yurut.FuncVal); ok {
		gerekli := 0
		for _, p := range f.Parameters {
			if p.DefaultValue == nil {
				gerekli++
			}
		}
		if len(f.Parameters) < 2 || gerekli > 2 {
			return nil, fmt.Errorf("%s: geri iki parametre almalıdır: fonksiyon(saklananlar, g), %d parametreli fonksiyon verildi", fn, len(f.Parameters))
		}
	}
	o := &ozelIslem{ileri: ileri, geri: geri}
	return &yurut.BuiltinVal{Name: fn, Fn: o.cagir}, nil
}

// cagir özel işlemi çalıştırır: ileri türevsiz çalışır, sonuç tek dış düğümle çizgeye
// bağlanır. Adlı argümanlar ileri'ye olduğu gibi geçer.
func (o *ozelIslem) cagir(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	args = append([]yurut.Value(nil), args...)
	girdiler := make([]*tensor.Tensor, len(args))
	for i, a := range args {
		if d, ok := a.(*Deger); ok {
			girdiler[i] = d.t
		}
	}
	var sonuc yurut.Value
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
		return tensor.CustomOp("özel_türev", girdiler, ciktilar, saklanan, o.geriHesabi(m, args, saklananlar, cok))
	})
	if err != nil {
		return nil, err
	}
	if !cok {
		return Yeni(outs[0]), nil
	}
	el := make([]yurut.Value, len(outs))
	for i, t := range outs {
		el[i] = Yeni(t)
	}
	return &yurut.ListVal{Elements: el}, nil
}

// ileriSonucu ileri'nin [çıktı, saklananlar] dönüşünü çözer.
func ileriSonucu(v yurut.Value) (ciktilar []*tensor.Tensor, cok bool, saklananlar *yurut.ListVal, err error) {
	l, ok := v.(*yurut.ListVal)
	if !ok || len(l.Elements) != 2 {
		return nil, false, nil, fmt.Errorf("özel_türev: ileri [çıktı, saklananlar] döndürmeli (ör. sonuç [x * x, [x]]; saklanacak bir şey yoksa sonuç [y, []]), %s verildi", kisaGoster(v))
	}
	saklananlar, ok = l.Elements[1].(*yurut.ListVal)
	if !ok {
		return nil, false, nil, fmt.Errorf("özel_türev: ileri'nin döndürdüğü [çıktı, saklananlar] içinde saklananlar bir liste olmalıdır (ör. [x] ya da []), %s verildi", tipAdi(l.Elements[1]))
	}
	switch c := l.Elements[0].(type) {
	case *Deger:
		return []*tensor.Tensor{c.t}, false, saklananlar, nil
	case *yurut.ListVal:
		if len(c.Elements) == 0 {
			return nil, false, nil, fmt.Errorf("özel_türev: ileri'nin çıktı listesi boş; en az bir tensör döndürün")
		}
		for i, e := range c.Elements {
			d, ok := e.(*Deger)
			if !ok {
				return nil, false, nil, fmt.Errorf("özel_türev: ileri'nin çıktı listesinin %d. öğesi tensör olmalıdır, %s verildi", i+1, tipAdi(e))
			}
			ciktilar = append(ciktilar, d.t)
		}
		return ciktilar, true, saklananlar, nil
	}
	return nil, false, nil, fmt.Errorf("özel_türev: ileri'nin çıktısı bir tensör ya da tensör listesi olmalıdır, %s verildi", tipAdi(l.Elements[0]))
}

// kisaGoster hata mesajı için değerin tipini ve (kısaysa) kendisini yazar.
func kisaGoster(v yurut.Value) string {
	if v == nil {
		return "yok"
	}
	s := v.Inspect()
	if len(s) > 60 || strings.Contains(s, "\n") {
		return tipAdi(v)
	}
	return fmt.Sprintf("%s (%s)", s, tipAdi(v))
}

// tensorleriTopla saklananlar içindeki (iç içe listeler dahil) tensörleri toplar; bunların
// sürümleri kaydedilir ve geri yayılımdan önce yerinde değiştirilmedikleri denetlenir.
func tensorleriTopla(v yurut.Value, out *[]*tensor.Tensor, derinlik int) {
	if derinlik > 16 {
		return
	}
	switch x := v.(type) {
	case *Deger:
		*out = append(*out, x.t)
	case *yurut.ListVal:
		for _, e := range x.Elements {
			tensorleriTopla(e, out, derinlik+1)
		}
	}
}

// geriHesabi çekirdeğin dış düğümüne verilen geri çağrıyı kurar. Bağlam (ctx) geri
// yayılımı başlatan görevin VM'idir; verilmemişse (Go'dan doğrudan geri yayılım) ileri
// hesabı yapan görev kullanılır.
func (o *ozelIslem) geriHesabi(ileriVM *yurut.VM, args []yurut.Value, saklananlar *yurut.ListVal, cok bool) tensor.CustomBackward {
	return func(ctx any, grads []*tensor.Tensor) []*tensor.Tensor {
		m, _ := ctx.(*yurut.VM)
		if m == nil {
			m = ileriVM
		}
		var g yurut.Value
		if cok {
			el := make([]yurut.Value, len(grads))
			for i, t := range grads {
				el[i] = Yeni(t)
			}
			g = &yurut.ListVal{Elements: el}
		} else {
			g = Yeni(grads[0])
		}
		sonuc, err := m.ApplyFunction(o.geri, []yurut.Value{saklananlar, g}, nil, 0)
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
func geriSonucu(v yurut.Value, args []yurut.Value) ([]*tensor.Tensor, error) {
	l, ok := v.(*yurut.ListVal)
	if !ok {
		return nil, fmt.Errorf("özel_türev: geri her girdi için bir gradyan (ya da yok) içeren bir liste döndürmeli (ör. sonuç [g * 2 * x]), %s verildi", kisaGoster(v))
	}
	if len(l.Elements) != len(args) {
		return nil, fmt.Errorf("özel_türev: geri %d öğeli liste döndürdü ama işlemin %d konumlu girdisi var; her girdi için sırasıyla bir gradyan ya da yok verin", len(l.Elements), len(args))
	}
	out := make([]*tensor.Tensor, len(args))
	for i, e := range l.Elements {
		if _, yok := e.(*yurut.NullVal); yok {
			continue
		}
		d, ok := e.(*Deger)
		if !ok {
			return nil, fmt.Errorf("özel_türev: geri'nin %d. öğesi (%d. girdinin gradyanı) tensör ya da yok olmalıdır, %s verildi", i+1, i+1, tipAdi(e))
		}
		girdi, ok := args[i].(*Deger)
		if !ok {
			return nil, fmt.Errorf("özel_türev: %d. girdi tensör değil (%s); geri bu girdi için yok döndürmeli", i+1, tipAdi(args[i]))
		}
		if gs, is := d.t.Shape(), girdi.t.Shape(); !sekilEsitMi(gs, is) {
			return nil, fmt.Errorf("özel_türev: geri %d. girdi için %s şeklinde gradyan döndürdü, beklenen %s (girdinin şekli); gradyan girdiyle aynı şekilde olmalı, yayınlama yapılmaz", i+1, sekilMetni(gs), sekilMetni(is))
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
func turevDenetle(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	const fn = "türev_denetle"
	if err := baglama.AdliDenetle(fn+"()", named, adAdim, adTolerans); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, fmt.Errorf("%s(f, girdiler, adım: 0.001, tolerans: 0.01) 2 konumlu parametre bekler, %d verildi", fn, len(args))
	}
	f := args[0]
	if !cagrilabilirMi(f) {
		return nil, fmt.Errorf("%s: ilk parametre bir fonksiyon olmalıdır, %s verildi", fn, tipAdi(f))
	}
	liste, ok := args[1].(*yurut.ListVal)
	if !ok {
		return nil, fmt.Errorf("%s: girdiler bir liste olmalıdır (ör. [tensör([[0.3, -1.2]], türev_izle: doğru)]), %s verildi", fn, tipAdi(args[1]))
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
		return nil, fmt.Errorf("%s türevsiz { ... } bloğu içinde çalışamaz", fn)
	}

	// Türev izleyen girdilerin float32 yaprak kopyaları.
	girdiler := make([]yurut.Value, len(liste.Elements))
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
		return nil, fmt.Errorf("%s: girdilerden hiçbiri türev izlemiyor; denetlenecek tensörleri tensör(..., türev_izle: doğru) ile oluşturun", fn)
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
		var s yurut.Value
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
	return yurut.TRUE, nil
}

// denetimCiktilari f'nin dönüşünü tensör listesine çevirir (tek tensör ya da tensör listesi).
func denetimCiktilari(fn string, v yurut.Value) ([]*tensor.Tensor, error) {
	switch x := v.(type) {
	case *Deger:
		return []*tensor.Tensor{x.t}, nil
	case *yurut.ListVal:
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
func pozitifSayi(fn string, named map[string]yurut.Value, ad baglama.Ad, varsayilan float64) (float64, error) {
	v, ok := baglama.AdliAl(named, ad)
	if !ok {
		return varsayilan, nil
	}
	f, ok := yurut.ToFloat(v)
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

package ogrenme

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/ortak/baglama"
)

// Bu dosya model yazmak için gereken yapı taşlarını TR'ye bağlar: tensör birleştirme ve
// bölme, birleşik dikkat, token örnekleme ve bir modelin parametrelerini toplama.

// Adlı argümanlar (adları kod listesinde, internal/dil/kod T9xx).
var (
	adBoyutuKoru = baglama.KodAd(kod.T906)
	adNedensel   = baglama.KodAd(kod.T950)
	adPencere    = baglama.KodAd(kod.T951)
	adMaske      = baglama.KodAd(kod.T952)
	adSicaklik   = baglama.KodAd(kod.T960)
	adEnIyiK     = baglama.KodAd(kod.T961)
	adAdlariyla  = baglama.KodAd(kod.T970)
)

// uyeBol tensörün .böl(parça_sayısı, eksen: -1) metodudur. Üye adları tipten bağımsız
// benzersiz olduğundan Metin'in .böl'üyle aynı koddur (Y103).
const uyeBol = kod.Y103

// adliMantik adlı bir Mantık argümanını okur; verilmemişse varsayilan döner.
func adliMantik(fn string, named map[string]yurut.Value, a baglama.Ad, varsayilan bool) (bool, error) {
	v, ok := baglama.AdliAl(named, a)
	if !ok {
		return varsayilan, nil
	}
	return baglama.Mantik(fn, a.TR, v)
}

// ----------------------------------------------------------------------------
// birleştir ve .böl
// ----------------------------------------------------------------------------

// birlestirYerlesik birleştir([a, b, ...], eksen: -1) yerleşiğidir: tensörleri bir eksen
// boyunca birleştirir. Birleşen eksen dışındaki eksenler eşit olmalıdır. Türevlenebilir.
func birlestirYerlesik(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T121)
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("%s([a, b, ...], %s: -1) bir tensör listesi bekler, %d parametre verildi", fn, adEksen.TR, len(args))
	}
	if err := baglama.AdliDenetle(fn+"()", named, adEksen); err != nil {
		return nil, err
	}
	l, ok := args[0].(*yurut.ListVal)
	if !ok {
		return nil, fmt.Errorf("%s: ilk parametre tensörlerin listesi olmalıdır ([a, b]), %s verildi", fn, tipAdi(args[0]))
	}
	if len(l.Elements) == 0 {
		return nil, fmt.Errorf("%s: liste boş; en az bir tensör verin", fn)
	}
	ts := make([]*tensor.Tensor, len(l.Elements))
	for i, el := range l.Elements {
		t, err := tensoreCevir(fmt.Sprintf("%s: %d. eleman", fn, i+1), el)
		if err != nil {
			return nil, err
		}
		ts[i] = t
	}
	e, verildi, err := eksenArg(fn, args, named)
	if err != nil {
		return nil, err
	}
	if !verildi {
		e = -1
	}
	ilk := ts[0].Shape()
	if e, err = eksenCoz(fn, e, ilk); err != nil {
		return nil, err
	}
	for i, t := range ts[1:] {
		sh := t.Shape()
		if len(sh) != len(ilk) {
			return nil, fmt.Errorf("%s: %d. tensör %d boyutlu (%s), 1. tensör %d boyutlu (%s); hepsi aynı sayıda eksenli olmalı", fn, i+2, len(sh), sekilMetni(sh), len(ilk), sekilMetni(ilk))
		}
		for d := range sh {
			if d != e && sh[d] != ilk[d] {
				return nil, fmt.Errorf("%s: %d. tensörün şekli %s, 1. tensörünki %s; %d. eksen dışındaki eksenler eşit olmalı (%d. eksende %d ≠ %d)", fn, i+2, sekilMetni(sh), sekilMetni(ilk), e, d, sh[d], ilk[d])
			}
		}
		if err := cihazDenetle(fn, ts[0], t); err != nil {
			return nil, err
		}
	}
	r, err := guvenli(func() *tensor.Tensor { return tensor.Concat(ts, e) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// bol .böl(parça_sayısı, eksen: -1) metodudur: tensörü verilen eksende eşit parçalara
// böler; parçalar veriyi paylaşan, türev izleyen görünümlerdir.
func (d *Deger) bol(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.Y103)
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("%s(parça_sayısı, %s: -1) parça sayısını bekler, %d parametre verildi", fn, adEksen.TR, len(args))
	}
	if err := baglama.AdliDenetle(fn+"()", named, adEksen); err != nil {
		return nil, err
	}
	n, ok := tamSayi(args[0])
	if !ok || n <= 0 {
		return nil, fmt.Errorf("%s: parça sayısı pozitif bir tam sayı olmalıdır, %s verildi", fn, args[0].Inspect())
	}
	e, verildi, err := eksenArg(fn, args, named)
	if err != nil {
		return nil, err
	}
	if !verildi {
		e = -1
	}
	sh := d.t.Shape()
	if e, err = eksenCoz(fn, e, sh); err != nil {
		return nil, err
	}
	if sh[e]%n != 0 {
		return nil, fmt.Errorf("%s: %s şeklinde %d. eksenin uzunluğu %d, %d eşit parçaya bölünemez", fn, sekilMetni(sh), e, sh[e], n)
	}
	parcalar, err := guvenli(func() []*tensor.Tensor { return d.t.Split(n, e) })
	if err != nil {
		return nil, err
	}
	el := make([]yurut.Value, len(parcalar))
	for i, p := range parcalar {
		y := Yeni(p)
		y.tamSayili = d.tamSayili
		el[i] = y
	}
	return &yurut.ListVal{Elements: el}, nil
}

// ----------------------------------------------------------------------------
// dikkat
// ----------------------------------------------------------------------------

// dikkatYerlesik dikkat(q, k, v, nedensel: doğru, pencere: 0, maske: yok) yerleşiğidir:
// birleşik ölçekli nokta çarpımı dikkati (bkz. tensor.Attention). Ölçek 1/√dh'dir.
func dikkatYerlesik(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T503)
	if len(args) != 3 {
		return nil, fmt.Errorf("%s(q, k, v, %s: %s, %s: 0, %s: %s) 3 konumlu parametre bekler, %d verildi",
			fn, adNedensel.TR, kod.Ad(kod.A040), adPencere.TR, adMaske.TR, kod.Ad(kod.A042), len(args))
	}
	if err := baglama.AdliDenetle(fn+"()", named, adNedensel, adPencere, adMaske); err != nil {
		return nil, err
	}
	var qkv [3]*tensor.Tensor
	for i, ad := range []string{"q", "k", "v"} {
		d, ok := args[i].(*Deger)
		if !ok {
			return nil, fmt.Errorf("%s: %s bir tensör olmalıdır, %s verildi", fn, ad, tipAdi(args[i]))
		}
		qkv[i] = d.t
	}
	q, k, v := qkv[0], qkv[1], qkv[2]
	opt := tensor.AttentionOptions{Causal: true}
	var err error
	if opt.Causal, err = adliMantik(fn, named, adNedensel, true); err != nil {
		return nil, err
	}
	if pv, ok := baglama.AdliAl(named, adPencere); ok {
		w, tam := tamSayi(pv)
		if !tam || w < 0 {
			return nil, fmt.Errorf("%s: pencere negatif olmayan bir tam sayı olmalıdır (0: pencere yok), %s verildi", fn, pv.Inspect())
		}
		opt.Window = w
	}
	if err := dikkatSekilDenetle(fn, q, k, v); err != nil {
		return nil, err
	}
	if mv, ok := baglama.AdliAl(named, adMaske); ok && mv != yurut.NULL {
		mt, err := tensoreCevir(fn+": maske", mv)
		if err != nil {
			return nil, err
		}
		if err := maskeDenetle(fn, mt, q, k); err != nil {
			return nil, err
		}
		opt.Mask = mt
	}
	for _, t := range []*tensor.Tensor{k, v} {
		if err := cihazDenetle(fn, q, t); err != nil {
			return nil, err
		}
	}
	karisik := karisikTuru(m)
	r, err := guvenli(func() *tensor.Tensor {
		if karisik != tensor.Float32 {
			// Karışık hassasiyet: girdiler 16 bite çevrilir, biriktirme ve çıktı float32.
			return tensor.AttentionOut(q.Cast(karisik), k.Cast(karisik), v.Cast(karisik), opt, tensor.Float32)
		}
		return tensor.Attention(q, k, v, opt)
	})
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// dikkatSekilDenetle q [..., H, Tq, dh], k [..., Hkv, Tk, dh], v [..., Hkv, Tk, dv]
// şekillerini denetler ve yol gösteren bir hata verir.
func dikkatSekilDenetle(fn string, q, k, v *tensor.Tensor) error {
	qs, ks, vs := q.Shape(), k.Shape(), v.Shape()
	n := len(qs)
	if n < 2 {
		return fmt.Errorf("%s: q en az 2 boyutlu olmalıdır ([..., baş, T, dh]), %s verildi", fn, sekilMetni(qs))
	}
	if len(ks) != n || len(vs) != n {
		return fmt.Errorf("%s: q, k ve v aynı sayıda eksenli olmalıdır; %s, %s, %s verildi", fn, sekilMetni(qs), sekilMetni(ks), sekilMetni(vs))
	}
	on := max(n-3, 0)
	if sekilMetni(qs[:on]) != sekilMetni(ks[:on]) || sekilMetni(qs[:on]) != sekilMetni(vs[:on]) {
		return fmt.Errorf("%s: öndeki (toplu) eksenler eşit olmalıdır; q %s, k %s, v %s", fn, sekilMetni(qs), sekilMetni(ks), sekilMetni(vs))
	}
	if ks[n-1] != qs[n-1] {
		return fmt.Errorf("%s: q ve k'nin son ekseni (dh) eşit olmalıdır; q %s, k %s", fn, sekilMetni(qs), sekilMetni(ks))
	}
	if vs[n-2] != ks[n-2] {
		return fmt.Errorf("%s: k ve v'nin anahtar sayısı (sondan 2. eksen) eşit olmalıdır; k %s, v %s", fn, sekilMetni(ks), sekilMetni(vs))
	}
	if n >= 3 {
		H, Hkv := qs[n-3], ks[n-3]
		if vs[n-3] != Hkv {
			return fmt.Errorf("%s: k ve v'nin baş sayısı eşit olmalıdır; k %s, v %s", fn, sekilMetni(ks), sekilMetni(vs))
		}
		if Hkv == 0 || H%Hkv != 0 {
			return fmt.Errorf("%s: q'nun baş sayısı (%d), k/v baş sayısının (%d) katı olmalıdır (gruplu sorgu dikkati)", fn, H, Hkv)
		}
	}
	return nil
}

// maskeDenetle maskenin [..., H, Tq, Tk] şekline yayınlanabildiğini denetler.
func maskeDenetle(fn string, mt, q, k *tensor.Tensor) error {
	qs, ms := q.Shape(), mt.Shape()
	hedef := append(qs[:len(qs)-1:len(qs)-1], k.Shape()[k.Dim()-2])
	if len(ms) > len(hedef) {
		return fmt.Errorf("%s: maske %s, puan şekline %s yayınlanamaz", fn, sekilMetni(ms), sekilMetni(hedef))
	}
	for i := 1; i <= len(ms); i++ {
		if d := ms[len(ms)-i]; d != 1 && d != hedef[len(hedef)-i] {
			return fmt.Errorf("%s: maske %s, puan şekline %s yayınlanamaz; sondan %d. eksende %d ≠ %d (eksenler sağdan hizalanır, eşit ya da 1 olmalı)", fn, sekilMetni(ms), sekilMetni(hedef), i, d, hedef[len(hedef)-i])
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// örnekle
// ----------------------------------------------------------------------------

// orneklemeYerlesik örnekle(logit, sıcaklık: 1.0, en_iyi_k: 0, tohum: yok) yerleşiğidir:
// son eksen üzerindeki softmax(logit / sıcaklık) dağılımından bir indeks örnekler.
// en_iyi_k > 0 ise yalnızca en olası k arasından seçer; sıcaklık 0 en olasıyı verir
// (maks_indeks ile aynı). Sonuç son ekseni düşürülmüş, tam sayılı işaretli ve türev
// izlemeyen bir tensördür.
func orneklemeYerlesik(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T611)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(logit, %s: 1.0, %s: 0, %s: %s) 1 konumlu parametre bekler, %d verildi",
			fn, adSicaklik.TR, adEnIyiK.TR, adTohum.TR, kod.Ad(kod.A042), len(args))
	}
	if err := baglama.AdliDenetle(fn+"()", named, adSicaklik, adEnIyiK, adTohum); err != nil {
		return nil, err
	}
	d, ok := args[0].(*Deger)
	if !ok {
		return nil, fmt.Errorf("%s: logit bir tensör olmalıdır, %s verildi; liste için %s(%s(liste)) yazın", fn, tipAdi(args[0]), fn, kod.Ad(kod.T100))
	}
	sh := d.t.Shape()
	if len(sh) == 0 {
		return nil, fmt.Errorf("%s: logit en az 1 boyutlu olmalıdır (son eksen sınıflar)", fn)
	}
	C := sh[len(sh)-1]
	if C == 0 {
		return nil, fmt.Errorf("%s: %s şeklinde son eksen boş", fn, sekilMetni(sh))
	}
	sicaklik := 1.0
	if sv, ok := baglama.AdliAl(named, adSicaklik); ok {
		f, sayi := yurut.ToFloat(sv)
		if !sayi || f < 0 || math.IsNaN(f) {
			return nil, fmt.Errorf("%s: %s negatif olmayan bir sayı olmalıdır (0: en olası), %s verildi", fn, adSicaklik.TR, sv.Inspect())
		}
		sicaklik = f
	}
	k := 0
	if kv, ok := baglama.AdliAl(named, adEnIyiK); ok {
		n, tam := tamSayi(kv)
		if !tam || n < 0 {
			return nil, fmt.Errorf("%s: %s negatif olmayan bir tam sayı olmalıdır (0: hepsi), %s verildi", fn, adEnIyiK.TR, kv.Inspect())
		}
		k = n
	}
	if k == 0 || k > C {
		k = C
	}
	var rng *rand.Rand
	if tv, ok := baglama.AdliAl(named, adTohum); ok && tv != yurut.NULL {
		t, tam := tv.(*yurut.IntVal)
		if !tam {
			return nil, fmt.Errorf("%s: tohum tam sayı olmalıdır, %s verildi", fn, tipAdi(tv))
		}
		rng = rand.New(rand.NewSource(t.Val))
	}
	veri := d.t.Data()
	satir := len(veri) / C
	sonuc := make([]float32, satir)
	// Rastgele sayılar önce çekilir (paylaşılan üreteç kilit altında, kısa süre).
	u := make([]float64, satir)
	if sicaklik > 0 {
		if rng != nil {
			for i := range u {
				u[i] = rng.Float64()
			}
		} else {
			rastgeleMu.Lock()
			for i := range u {
				u[i] = rastgeleKay.Float64()
			}
			rastgeleMu.Unlock()
		}
	}
	idx := make([]int, C)
	agirlik := make([]float64, C)
	for r := 0; r < satir; r++ {
		x := veri[r*C : (r+1)*C]
		if sicaklik == 0 {
			en := 0
			for j, v := range x {
				if v > x[en] || (v != v && x[en] == x[en]) {
					en = j
				}
			}
			sonuc[r] = float32(en)
			continue
		}
		adaylar := idx[:C]
		for j := range adaylar {
			adaylar[j] = j
		}
		if k < C {
			// En olası k: büyükten küçüğe, eşitlikte küçük indeks önce.
			sort.SliceStable(adaylar, func(a, b int) bool { return x[adaylar[a]] > x[adaylar[b]] })
			adaylar = adaylar[:k]
			sort.Ints(adaylar)
		}
		enb := math.Inf(-1)
		for _, j := range adaylar {
			enb = math.Max(enb, float64(x[j]))
		}
		var top float64
		for i, j := range adaylar {
			w := math.Exp((float64(x[j]) - enb) / sicaklik)
			agirlik[i] = w
			top += w
		}
		hedef := u[r] * top
		secilen := adaylar[len(adaylar)-1]
		var birikim float64
		for i, j := range adaylar {
			birikim += agirlik[i]
			if hedef < birikim {
				secilen = j
				break
			}
		}
		sonuc[r] = float32(secilen)
	}
	cikti := sh[:len(sh)-1]
	r, err := guvenli(func() *tensor.Tensor { return tensor.NewOn(d.t.Device(), sonuc, cikti...) })
	if err != nil {
		return nil, err
	}
	return YeniTam(r), nil
}

// ----------------------------------------------------------------------------
// parametreler
// ----------------------------------------------------------------------------

// parametrelerYerlesik parametreler(model, adlarıyla: yanlış) yerleşiğidir: bir yapı
// nesnesini (iç içe yapılar, listeler ve sözlüklerle birlikte) gezip türev izleyen yaprak
// tensörleri sabit sırayla (yapı alanlarının tanım sırası, liste sırası, sözlük ekleme
// sırası) toplar. Aynı tensör bir kez yer alır. adlarıyla: doğru yol adı → tensör sözlüğü
// verir ("bloklar.0.dikkat.Wq"); bu sözlük tensör_kaydet ile kaydedilebilir.
func parametrelerYerlesik(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T530)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(model, %s: %s) 1 konumlu parametre bekler, %d verildi", fn, adAdlariyla.TR, kod.Ad(kod.A041), len(args))
	}
	if err := baglama.AdliDenetle(fn+"()", named, adAdlariyla); err != nil {
		return nil, err
	}
	adlariyla, err := adliMantik(fn, named, adAdlariyla, false)
	if err != nil {
		return nil, err
	}
	var bulunan []yurut.Value
	var adlar []string
	gorulen := map[*tensor.Tensor]bool{}
	gezilen := map[any]bool{}
	adSahibi := map[string]*tensor.Tensor{}
	var hata error
	var gez func(v yurut.Value, yol string)
	gez = func(v yurut.Value, yol string) {
		if hata != nil {
			return
		}
		altYol := func(ad string) string {
			if yol == "" {
				return ad
			}
			return yol + "." + ad
		}
		switch x := v.(type) {
		case *Deger:
			if x.ara || !x.t.RequiresGrad() || gorulen[x.t] {
				return
			}
			if o, ok := adSahibi[yol]; ok && o != x.t {
				hata = fmt.Errorf("%s: iki farklı tensör aynı yol adına düşüyor (%q); sözlük anahtarlarında '.' kullanmayın", fn, yol)
				return
			}
			gorulen[x.t] = true
			adSahibi[yol] = x.t
			bulunan = append(bulunan, x)
			adlar = append(adlar, yol)
		case *yurut.StructInstanceVal:
			if gezilen[x] {
				return
			}
			gezilen[x] = true
			x.AlanlariGez(func(ad string, d yurut.Value) { gez(d, altYol(ad)) })
		case *yurut.ListVal:
			if gezilen[x] {
				return
			}
			gezilen[x] = true
			for i, el := range x.Elements {
				gez(el, altYol(strconv.Itoa(i)))
			}
		case *yurut.DictVal:
			if gezilen[x] {
				return
			}
			gezilen[x] = true
			for _, k := range x.Order {
				gez(x.Pairs[k], altYol(x.DisplayKey(k)))
			}
		}
	}
	gez(args[0], "")
	if hata != nil {
		return nil, hata
	}
	if !adlariyla {
		return &yurut.ListVal{Elements: bulunan}, nil
	}
	s := yurut.NewDictVal()
	for i, ad := range adlar {
		if ad == "" {
			ad = "0" // model tek bir tensörse
		}
		s.Set(ad, bulunan[i])
	}
	return s, nil
}

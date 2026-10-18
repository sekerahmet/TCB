package parcalayici

// ============================================================================
// Hata döndüren (sunucu kullanımına uygun) kodlama/çözme işlevleri
// ============================================================================
//
// Düzenli ifade kullanan bileşenler (Split, Replace, bayt düzeyi desen) adım
// bütçesini aşarsa (bkz. duzifade_guvenlik.go) Kodla/Coz gibi hata
// döndürmeyen işlevler *AdimSiniriHatasi ile panikler. Kullanıcı girdisi
// işleyen sunucular aşağıdaki Guvenli... işlevlerini kullanmalıdır: bu
// durumda yarım ya da yanlış bir sonuç yerine açık bir hata döner.

// GuvenliKodla Kodla'nın hata döndüren biçimidir.
func (p *Parcalayici) GuvenliKodla(metin string, s Secenekler) (ids []int, err error) {
	err = adimHatasiniYakala(func() { ids = p.KodlaSecenekli(metin, s) })
	if err != nil {
		ids = nil
	}
	return
}

// GuvenliKodla32 Kodla32'nin hata döndüren biçimidir; hata olursa dst değişmeden döner.
func (p *Parcalayici) GuvenliKodla32(dst []int32, metin string, s Secenekler) ([]int32, error) {
	n := len(dst)
	err := adimHatasiniYakala(func() { dst = p.Kodla32(dst, metin, s) })
	if err != nil {
		return dst[:n], err
	}
	return dst, nil
}

// GuvenliKodlaCift KodlaCift'in hata döndüren biçimidir.
func (p *Parcalayici) GuvenliKodlaCift(birinci string, ikinci *string, s Secenekler) (k *Kodlama, err error) {
	err = adimHatasiniYakala(func() { k = p.KodlaCift(birinci, ikinci, s) })
	if err != nil {
		k = nil
	}
	return
}

// GuvenliCoz Coz'un hata döndüren biçimidir (çözücüde düzenli ifade varsa).
func (p *Parcalayici) GuvenliCoz(idler []int, ozelleriAtla bool) (m string, err error) {
	err = adimHatasiniYakala(func() { m = p.Coz(idler, ozelleriAtla) })
	return
}

// GuvenliTopluKodla TopluKodla'nın hata döndüren biçimidir. Hatalı metinlerin
// sonucu nil'dir; hatalar[i] i. metnin hatasıdır (hiç hata yoksa dilim nil).
func (p *Parcalayici) GuvenliTopluKodla(metinler []string, s Secenekler, isci int) ([][]int, []error) {
	out := make([][]int, len(metinler))
	hatalar := make([]error, len(metinler))
	paralel(len(metinler), isci, func(i int) {
		out[i], hatalar[i] = p.GuvenliKodla(metinler[i], s)
	})
	for _, h := range hatalar {
		if h != nil {
			return out, hatalar
		}
	}
	return out, nil
}

// Uyarilar parçalayıcının düzenli ifadelerinde derleme sırasında bulunan
// risk uyarılarını verir (ör. sınırsız yineleme içinde çakışan seçenekler).
func (p *Parcalayici) Uyarilar() []string {
	var out []string
	topla := func(d *DuzIfade) {
		for _, u := range d.Uyarilar() {
			out = append(out, d.kaynak+": "+u)
		}
	}
	ifadeleriGez(p.normallestirici, topla)
	ifadeleriGez(p.onBolucu, topla)
	ifadeleriGez(p.cozucu, topla)
	return out
}

// ifadeleriGez bir bileşen (zinciri) içindeki düzenli ifadeleri gezer.
func ifadeleriGez(b any, f func(*DuzIfade)) {
	switch x := b.(type) {
	case NormallestiriciSirasi:
		for _, o := range x {
			ifadeleriGez(o, f)
		}
	case OnBolucuSirasi:
		for _, o := range x {
			ifadeleriGez(o, f)
		}
	case CozucuSirasi:
		for _, o := range x {
			ifadeleriGez(o, f)
		}
	case degistirNorm:
		if x.d.ifade != nil {
			f(x.d.ifade)
		}
	case degistirCozucu:
		if x.d.ifade != nil {
			f(x.d.ifade)
		}
	case *desenleBol:
		if x.ifade != nil {
			f(x.ifade)
		}
	case *baytOnBol:
		if x.desenle && baytDuzeyiIfade != nil {
			f(baytDuzeyiIfade)
		}
	}
}

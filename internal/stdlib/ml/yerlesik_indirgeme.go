package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// ----------------------------------------------------------------------------
// İndirgemeler (toplam, ortalama, maks, min) ve en büyüğün indeksi
// ----------------------------------------------------------------------------

// indirgeme toplam/ortalama/maks/min'i kaydeder. Eksen verilmezse tüm elemanlar
// indirgenir ve 0 boyutlu (skaler) tensör döner; eksen verilirse o eksen kaldırılır.
// boyutu_koru: doğru indirgenen ekseni (eksen verilmezse bütün eksenleri) 1 uzunlukta
// korur: [B, T, D] → [B, T, 1].
func indirgeme(v *interp.VM, kd codes.Kod, fn func(t *tensor.Tensor, eksen int, tum, koru bool) *tensor.Tensor) {
	ad := codes.Ad(kd)
	maksMin := kd == codes.G402 || kd == codes.G401 // maks, min
	sar(v, kd, func(eski interp.BuiltinFunc) interp.BuiltinFunc {
		return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if len(args) == 0 {
				if err := bind.AdliDenetle(ad+"()", named, adEksen, adBoyutuKoru); err != nil {
					return nil, err
				}
				return eskiCagir(ad, eski, m, args, named)
			}
			if _, ok := args[0].(*Deger); !ok {
				return tensorOlmayanIndirgeme(kd, eski, m, args, named)
			}
			if maksMin && len(args) >= 2 {
				// maks(x, 0) eksen mi, eleman bazlı sınır mı belirsiz; sessizce eksen saymak yerine yol göster.
				sinir := map[bool]string{true: adEnAz.TR, false: adEnFazla.TR}[kd == codes.G402]
				return nil, fmt.Errorf("%s(x, %s): eksen için %s(x, %s: %s) yazın; eleman bazlı sınır için %s(x, %s: %s) kullanın",
					ad, args[1].Inspect(), ad, adEksen.TR, args[1].Inspect(), codes.Ad(codes.G409), sinir, args[1].Inspect())
			}
			if len(args) > 2 {
				return nil, fmt.Errorf("%s(x, %s: ...) en fazla 2 parametre alır, %d verildi", ad, adEksen.TR, len(args))
			}
			if err := bind.AdliDenetle(ad+"()", named, adEksen, adBoyutuKoru); err != nil {
				return nil, err
			}
			koru, err := adliMantik(ad, named, adBoyutuKoru, false)
			if err != nil {
				return nil, err
			}
			t := args[0].(*Deger).t
			if !maksMin {
				// Karışık hassasiyet bloğunda toplam/ortalama float32'de yapılır.
				if t, err = yuksekHassasiyet(m, t); err != nil {
					return nil, err
				}
			}
			e, verildi, err := eksenArg(ad, args, named)
			if err != nil {
				return nil, err
			}
			sh := t.Shape()
			if verildi {
				if e, err = eksenCoz(ad, e, sh); err != nil {
					return nil, err
				}
				if maksMin && sh[e] == 0 {
					return nil, fmt.Errorf("%s: %s şeklinde %d. eksen boş", ad, sekilMetni(sh), e)
				}
			} else if maksMin && t.Numel() == 0 {
				return nil, fmt.Errorf("%s: boş tensörün (%s) en büyük/küçük elemanı yoktur", ad, sekilMetni(sh))
			}
			if kd == codes.G410 { // ortalama: boş kümenin ortalaması tanımsızdır
				if !verildi && t.Numel() == 0 {
					return nil, fmt.Errorf("%s: boş tensörün ortalaması tanımsız (şekil %s)", ad, sekilMetni(sh))
				}
				if verildi && sh[e] == 0 {
					cikis := 1
					for i, d := range sh {
						if i != e {
							cikis *= d
						}
					}
					if cikis > 0 {
						return nil, fmt.Errorf("%s: %s şeklinde %d. eksen boş; boş kümenin ortalaması tanımsız", ad, sekilMetni(sh), e)
					}
				}
			}
			r, err := guvenli(func() *tensor.Tensor {
				r := fn(t, e, !verildi, koru)
				if koru && !verildi {
					birler := make([]int, len(sh))
					for i := range birler {
						birler[i] = 1
					}
					r = r.Reshape(birler...)
				}
				return r
			})
			if err != nil {
				return nil, err
			}
			return Yeni(r), nil
		}
	})
}

// maksIndeks verilen eksende en büyük elemanın indeksini verir: maks_indeks(x, eksen: -1).
// Sonuç her zaman tam sayı değerli bir tensördür (o eksen kaldırılmış şekilde; 1 boyutlu
// girdide 0 boyutlu) ve türev izlemez. Tensör tam sayılı olarak işaretlenir: .değer Tam
// (ya da Tam listesi) verir, böylece liste indeksi olarak doğrudan kullanılabilir; sonuç
// gömme(...) ve çapraz_entropi(...) indeksi olarak da verilebilir.
// Eşitlikte ilk en büyük eleman seçilir.
func maksIndeks(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Ad(codes.T230)
	if len(args) != 1 {
		if len(args) == 2 {
			return nil, fmt.Errorf("%s(x, %s: %s): eksen adlı verilir", ad, adEksen.TR, args[1].Inspect())
		}
		return nil, fmt.Errorf("%s(x, %s: -1) 1 konumlu parametre bekler, %d verildi", ad, adEksen.TR, len(args))
	}
	if err := bind.AdliDenetle(ad+"()", named, adEksen); err != nil {
		return nil, err
	}
	d, ok := args[0].(*Deger)
	if !ok {
		return nil, fmt.Errorf("%s: bir tensör bekler, %s verildi; liste için %s(%s(liste)) yazın", ad, tipAdi(args[0]), ad, codes.Ad(codes.T100))
	}
	t := d.t
	e, verildi, err := eksenArg(ad, args, named)
	if err != nil {
		return nil, err
	}
	if !verildi {
		e = -1
	}
	sh := t.Shape()
	if e, err = eksenCoz(ad, e, sh); err != nil {
		return nil, err
	}
	if sh[e] == 0 {
		return nil, fmt.Errorf("%s: %s şeklinde %d. eksen boş", ad, sekilMetni(sh), e)
	}
	r, err := guvenli(func() *tensor.Tensor {
		idx := t.ArgMax(e)
		veri := make([]float32, len(idx))
		for i, x := range idx {
			veri[i] = float32(x)
		}
		cikti := append(append([]int{}, sh[:e]...), sh[e+1:]...)
		return tensor.NewOn(t.Device(), veri, cikti...)
	})
	if err != nil {
		return nil, err
	}
	sonuc := Yeni(r)
	sonuc.tamSayili = true
	return sonuc, nil
}

// tensorOlmayanIndirgeme ilk argümanı tensör olmayan indirgeme çağrılarını işler. Eski
// yerleşikler iç içe listeleri ve listedeki tensörleri sessizce atladığı için (toplam([[1, 2]])
// → 0) bu durumlar burada yakalanır: toplam/ortalama listedeki tensörleri hesap çizgesini
// koruyarak katlar, diğer durumlar açık hata verir.
func tensorOlmayanIndirgeme(kd codes.Kod, eski interp.BuiltinFunc, m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Ad(kd)
	if err := bind.AdliDenetle(ad+"()", named, adEksen, adBoyutuKoru); err != nil {
		return nil, err
	}
	if _, ok := bind.AdliAl(named, adBoyutuKoru); ok {
		return nil, fmt.Errorf("%s: %s: yalnızca tensörlerde kullanılır, %s verildi; önce tensöre çevirin: %s(%s(...), %s: 0, %s: %s)",
			ad, adBoyutuKoru.TR, tipAdi(args[0]), ad, codes.Ad(codes.T100), adEksen.TR, adBoyutuKoru.TR, codes.Ad(codes.A040))
	}
	if _, ok := bind.AdliAl(named, adEksen); ok {
		return nil, fmt.Errorf("%s: %s: yalnızca tensörlerde kullanılır, %s verildi; önce tensöre çevirin: %s(%s(...), %s: 0)", ad, adEksen.TR, tipAdi(args[0]), ad, codes.Ad(codes.T100), adEksen.TR)
	}
	if l, ok := args[0].(*interp.ListVal); ok && len(args) == 1 {
		var tensorVar bool
		for _, el := range l.Elements {
			switch el.(type) {
			case *interp.ListVal:
				t := codes.Ad(codes.T100)
				return nil, fmt.Errorf("%s: iç içe liste indirgenemez (eski liste davranışı yalnızca düz sayı listesi alır); tensöre çevirin: %s(%s(liste)) ya da eksen için %s(%s(liste), %s: 0)", ad, ad, t, ad, t, adEksen.TR)
			case *Deger:
				tensorVar = true
			}
		}
		if tensorVar {
			if kd != codes.G400 && kd != codes.G410 { // toplam, ortalama
				return nil, fmt.Errorf("%s: tensör listesinin en %s elemanı tanımsız; tek bir tensör verin: %s(x) ya da %s(x, %s: 0)", ad, map[bool]string{true: "büyük", false: "küçük"}[kd == codes.G402], ad, ad, adEksen.TR)
			}
			return tensorListesiTopla(ad, kd == codes.G410, l.Elements)
		}
	}
	for _, a := range args[1:] {
		if _, ok := a.(*Deger); ok {
			return nil, fmt.Errorf("%s(x, %s: ...) ilk parametre olarak tensör bekler, %s verildi", ad, adEksen.TR, tipAdi(args[0]))
		}
	}
	return eskiCagir(ad, eski, m, args, named)
}

// tensorListesiTopla sayı ve tensörlerden oluşan bir listeyi yayınlamalı toplamla katlar;
// ortalama için eleman sayısına böler. Sonuç türev izlenen girdilere bağlı kalır.
func tensorListesiTopla(ad string, ortalama bool, el []interp.Value) (interp.Value, error) {
	var acc *tensor.Tensor
	sabit := 0.0
	for i, e := range el {
		switch x := e.(type) {
		case *Deger:
			if acc == nil {
				acc = x.t
				continue
			}
			if err := yayinDenetle("+", acc, x.t); err != nil {
				return nil, fmt.Errorf("%s: %d. eleman eklenemiyor: %v", ad, i+1, err)
			}
			a := acc
			r, err := guvenli(func() *tensor.Tensor { return tensor.Add(a, x.t) })
			if err != nil {
				return nil, err
			}
			acc = r
		default:
			f, ok := interp.ToFloat(e)
			if !ok {
				return nil, fmt.Errorf("%s: listenin %d. elemanı sayı ya da tensör olmalıdır, %s verildi", ad, i+1, tipAdi(e))
			}
			sabit += f
		}
	}
	r, err := guvenli(func() *tensor.Tensor {
		out := acc
		if sabit != 0 {
			out = out.AddScalar(float32(sabit))
		}
		if ortalama {
			out = tensor.Div(out, tensor.Scalar(float32(len(el))).To(out.Device()))
		}
		return out
	})
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

package ml

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// İyileştirici durumu: op.durum (Y502) momentleri, adım sayaçlarını ve ana kopyaları ad →
// tensör sözlüğü olarak verir; op.durumu_yükle(sözlük) (Y503) aynı parametre listesiyle
// kurulmuş bir iyileştiriciye geri yükler. Anahtarlar çekirdeğin anahtarlarıdır
// (internal/compute/tensor, iyileştirici durumu): "m.k", "v.k", "hiz.k", "ana.k", "adim.k",
// "adim"; k parametrenin listedeki sırasıdır. Kontrol noktası aynı Go yöntemlerini kullanır.

const (
	uyeDurum       = codes.Y502
	uyeDurumuYukle = codes.Y503
)

// durumUyesi durum ve durumu_yükle üyelerini verir; başka bir üyeyse ok yanlış.
func (o *Iyilestirici) durumUyesi(kd codes.Code) (interp.Value, bool, error) {
	switch kd {
	case uyeDurum:
		v, err := bind.Guvenli(interp.CallName(uyeDurum), func() (interp.Value, error) {
			s := interp.NewDictVal()
			adlar, tensorler := o.durumTensorleri(true)
			for i, ad := range adlar {
				s.Set(ad, Yeni(tensorler[i]))
			}
			return s, nil
		})
		return v, true, err
	case uyeDurumuYukle:
		return &interp.BuiltinVal{Name: codes.Name(uyeDurumuYukle), NoNamedArgs: true, Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			return bind.Guvenli(interp.CallName(uyeDurumuYukle), func() (interp.Value, error) { return o.durumuYukleTR(args) })
		}}, true, nil
	}
	return nil, false, nil
}

// durumuYukleTR op.durumu_yükle(sözlük) metodudur.
func (o *Iyilestirici) durumuYukleTR(args []interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(uyeDurumuYukle)
	if len(args) != 1 {
		return nil, interp.ArgCountError(uyeDurumuYukle, "1", len(args), "op."+codes.Name(uyeDurum))
	}
	s, ok := args[0].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgTypeError(uyeDurumuYukle, 1, args[0], codes.A206)
	}
	st := make(map[string]*tensor.Tensor, len(s.Order))
	for _, k := range s.Order {
		v, var_ := s.Pairs[k]
		if !var_ {
			continue
		}
		d, ok := v.(*Deger)
		if _, metin := s.KeyValue(k).(*interp.StringVal); !metin || !ok {
			return nil, fmt.Errorf("%s: sözlük %s ile alınmış olmalı (anahtarları %s, değerleri %s); %q girdisi %s", fn,
				codes.Name(uyeDurum), codes.Name(codes.A202), codes.Name(codes.A212), s.DisplayKey(k), interp.TypeAndSummary(v))
		}
		st[k] = d.t
	}
	if err := o.DurumuYukle(fn, st); err != nil {
		return nil, err
	}
	return interp.TRUE, nil
}

// IyilestiriciMi değer bir iyileştirici (AdamW, SGD) mi?
func IyilestiriciMi(v interp.Value) (*Iyilestirici, bool) {
	o, ok := v.(*Iyilestirici)
	return o, ok
}

// Ad iyileştiricinin oluşturucusunun adıdır (AdamW, SGD).
func (o *Iyilestirici) Ad() string { return o.ad }

// ParametreSayisi iyileştiricinin güncellediği parametre sayısıdır.
func (o *Iyilestirici) ParametreSayisi() int { return len(o.params) }

// DurumGorunumu iyileştiricinin durumunu adlara göre sıralı olarak verir; tensörler
// iyileştiricinin kendi depolarının görünümleridir (kopya değildir, cihazda ek bellek
// ayrılmaz). Yalnızca okunmalı, sıradaki adımdan önce kullanılmalıdır (kontrol noktası
// kaydederken konağa indirir).
func (o *Iyilestirici) DurumGorunumu() ([]string, []*tensor.Tensor) { return o.durumTensorleri(false) }

func (o *Iyilestirici) durumTensorleri(kopya bool) ([]string, []*tensor.Tensor) {
	var st map[string]*tensor.Tensor
	switch {
	case o.adamw != nil && kopya:
		st = o.adamw.State()
	case o.adamw != nil:
		st = o.adamw.StateView()
	case kopya:
		st = o.sgd.State()
	default:
		st = o.sgd.StateView()
	}
	adlar := make([]string, 0, len(st))
	for ad := range st {
		adlar = append(adlar, ad)
	}
	slices.Sort(adlar)
	tensorler := make([]*tensor.Tensor, len(adlar))
	for i, ad := range adlar {
		tensorler[i] = st[ad]
	}
	return adlar, tensorler
}

// DurumuYukle durumu (DurumGorunumu ya da op.durum biçiminde) yükler: parametreye bağlı
// her tensör ("m.k" ...) k. parametrenin cihazına taşınır. Anahtar, parametre sayısı ya da
// şekil uyuşmazlığı fn ile başlayan bir hatadır ve iyileştiriciyi değiştirmez (önce
// denetlenir). Kontrol noktasında parametreler bundan ÖNCE yüklenmelidir: ana kopyalar
// parametrenin o anki sürümüne bağlanır.
func (o *Iyilestirici) DurumuYukle(fn string, st map[string]*tensor.Tensor) error {
	for ad, t := range st {
		if err := o.durumGirdisiDenetle(fn, ad, t.Shape()); err != nil {
			return err
		}
	}
	tasinmis := make(map[string]*tensor.Tensor, len(st))
	_, err := bind.Kurtar(func() bool {
		for ad, t := range st {
			if k, ok := o.parametreSirasi(ad); ok {
				t = t.To(o.params[k].Device())
			}
			tasinmis[ad] = t
		}
		if o.adamw != nil {
			o.adamw.LoadState(tasinmis)
		} else {
			o.sgd.LoadState(tasinmis)
		}
		return true
	}, func(r any) error {
		return fmt.Errorf("%s: %s (%s, %d parametre)", fn, strings.TrimPrefix(fmt.Sprint(r), "tensor: "), o.ad, len(o.params))
	})
	return err
}

// parametreSirasi "m.3" gibi bir anahtarın parametre sırasını (3) verir.
func (o *Iyilestirici) parametreSirasi(ad string) (int, bool) {
	_, sira, ok := strings.Cut(ad, ".")
	if !ok {
		return 0, false
	}
	k, err := strconv.Atoi(sira)
	if err != nil || k < 0 || k >= len(o.params) || strconv.Itoa(k) != sira {
		return 0, false
	}
	return k, true
}

// DurumuDenetle durum girdilerinin (adlar ve şekiller) bu iyileştiriciye uyduğunu, hiçbir
// şey yüklemeden denetler; kontrol noktası bütün nesneleri yüklemeden önce bununla sınar.
func (o *Iyilestirici) DurumuDenetle(fn string, adlar []string, sekiller [][]int) error {
	for i, ad := range adlar {
		if err := o.durumGirdisiDenetle(fn, ad, sekiller[i]); err != nil {
			return err
		}
	}
	return nil
}

// durumGirdisiDenetle bir durum girdisinin anahtarını ve şeklini iyileştiriciyle
// karşılaştırır (yükleme yarıda kalmasın diye önceden).
func (o *Iyilestirici) durumGirdisiDenetle(fn, ad string, sekil []int) error {
	onek, _, _ := strings.Cut(ad, ".")
	gecerli := []string{"hiz", "ana"}
	if o.adamw != nil {
		gecerli = []string{"m", "v", "adim", "ana"}
	}
	k, sirali := o.parametreSirasi(ad)
	switch {
	case ad == "adim" && o.adamw != nil:
		k = -1
	case !sirali || !slices.Contains(gecerli, onek):
		return fmt.Errorf("%s: %q bu iyileştiricinin (%s, %d parametre) durum anahtarı değil; durum aynı tipte ve aynı parametre listesiyle kurulmuş bir iyileştiricinin %s'u olmalı",
			fn, ad, o.ad, len(o.params), codes.Name(uyeDurum))
	}
	beklenen := []int{2} // adım sayaçları
	if k >= 0 && onek != "adim" {
		beklenen = o.params[k].Shape()
	}
	if !slices.Equal(sekil, beklenen) {
		return fmt.Errorf("%s: %q girdisinin şekli %s, beklenen %s (parametre listesi kayıttakiyle aynı olmalı)", fn, ad, sekilMetni(sekil), sekilMetni(beklenen))
	}
	return nil
}

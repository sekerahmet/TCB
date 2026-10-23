package ml

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// İyileştirici durumu: op.durum (Y502) momentleri, adım sayaçlarını ve ana kopyaları ad →
// tensör sözlüğü olarak verir; op.durumu_yükle(sözlük) (Y503) aynı parametre listesiyle
// kurulmuş bir iyileştiriciye geri yükler. Anahtarlar çekirdeğin anahtarlarıdır
// (internal/compute/tensor, iyileştirici durumu): "m.k", "v.k", "hiz.k", "ana.k", "adim.k",
// "adim"; k parametrenin listedeki sırasıdır. Kontrol noktası aynı Go yöntemlerini kullanır.

const (
	memberState     = codes.Y502
	memberLoadState = codes.Y503
)

// stateMember durum ve durumu_yükle üyelerini verir; başka bir üyeyse ok yanlış.
func (o *Optimizer) stateMember(kd codes.Code) (interp.Value, bool, error) {
	switch kd {
	case memberState:
		v, err := bind.Safe(interp.CallName(memberState), func() (interp.Value, error) {
			s := interp.NewDictVal()
			names, tensors := o.stateTensors(true)
			for i, ad := range names {
				s.Set(ad, New(tensors[i]))
			}
			return s, nil
		})
		return v, true, err
	case memberLoadState:
		return bind.CodeMethod(memberLoadState, true, o.loadStateTR), true, nil
	}
	return nil, false, nil
}

// loadStateTR op.durumu_yükle(sözlük) metodudur.
func (o *Optimizer) loadStateTR(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(memberLoadState)
	s, err := bind.SingleDictArg(memberLoadState, args, messages.Text(messages.H2326, codes.DisplayName(memberState)))
	if err != nil {
		return nil, err
	}
	st := make(map[string]*tensor.Tensor, len(s.Order))
	for _, k := range s.Order {
		v, var_ := s.Pairs[k]
		if !var_ {
			continue
		}
		d, ok := v.(*TensorValue)
		if _, text := s.KeyValue(k).(*interp.StringVal); !text || !ok {
			return nil, messages.New(messages.H6501, fn, codes.DisplayName(memberState), codes.DisplayName(codes.A202), codes.DisplayName(codes.A212), s.DisplayKey(k), interp.TypeAndSummary(v))
		}
		st[k] = d.t
	}
	if err := o.LoadState(fn, st); err != nil {
		return nil, err
	}
	return interp.True, nil
}

// IsOptimizer değer bir iyileştirici (AdamW, SGD) mi?
func IsOptimizer(v interp.Value) (*Optimizer, bool) {
	o, ok := v.(*Optimizer)
	return o, ok
}

// Name iyileştiricinin oluşturucusunun adıdır (AdamW, SGD).
func (o *Optimizer) Name() string { return o.ad }

// ParamCount iyileştiricinin güncellediği parametre sayısıdır.
func (o *Optimizer) ParamCount() int { return len(o.params) }

// StateView iyileştiricinin durumunu adlara göre sıralı olarak verir; tensörler
// iyileştiricinin kendi depolarının görünümleridir (kopya değildir, cihazda ek bellek
// ayrılmaz). Yalnızca okunmalı, sıradaki adımdan önce kullanılmalıdır (kontrol noktası
// kaydederken konağa indirir).
func (o *Optimizer) StateView() ([]string, []*tensor.Tensor) { return o.stateTensors(false) }

func (o *Optimizer) stateTensors(dup bool) ([]string, []*tensor.Tensor) {
	var st map[string]*tensor.Tensor
	switch {
	case o.adamw != nil && dup:
		st = o.adamw.State()
	case o.adamw != nil:
		st = o.adamw.StateView()
	case dup:
		st = o.sgd.State()
	default:
		st = o.sgd.StateView()
	}
	names := make([]string, 0, len(st))
	for ad := range st {
		names = append(names, ad)
	}
	slices.Sort(names)
	tensors := make([]*tensor.Tensor, len(names))
	for i, ad := range names {
		tensors[i] = st[ad]
	}
	return names, tensors
}

// LoadState durumu (StateView ya da op.durum biçiminde) yükler: parametreye bağlı
// her tensör ("m.k" ...) k. parametrenin cihazına taşınır. Anahtar, parametre sayısı ya da
// şekil uyuşmazlığı fn ile başlayan bir hatadır ve iyileştiriciyi değiştirmez (önce
// denetlenir). Kontrol noktasında parametreler bundan ÖNCE yüklenmelidir: ana kopyalar
// parametrenin o anki sürümüne bağlanır.
func (o *Optimizer) LoadState(fn string, st map[string]*tensor.Tensor) error {
	if err := o.CheckLoad(fn, st); err != nil {
		return err
	}
	moved := make(map[string]*tensor.Tensor, len(st))
	return o.recoverStateOp(fn, func() {
		for ad, t := range st {
			if k, ok := o.paramOrder(ad); ok {
				t = t.To(o.params[k].Device())
			}
			moved[ad] = t
		}
		if o.adamw != nil {
			o.adamw.LoadState(moved)
		} else {
			o.sgd.LoadState(moved)
		}
	})
}

// CheckLoad durumu yüklemeden, LoadState'in bütün kurallarıyla denetler: anahtarlar ve
// şekiller (CheckState) ile adım sayaçları ve moment çiftleri. Kontrol noktası hiçbir nesneyi
// yazmadan önce bununla sınar (dil tanımı 20.7).
func (o *Optimizer) CheckLoad(fn string, st map[string]*tensor.Tensor) error {
	for ad, t := range st {
		if err := o.checkStateEntry(fn, ad, t.Shape()); err != nil {
			return err
		}
	}
	return o.recoverStateOp(fn, func() {
		if o.adamw != nil {
			o.adamw.CheckState(st)
		} else {
			o.sgd.CheckState(st)
		}
	})
}

// recoverStateOp durum işlemini çalıştırır; tensör katmanının paniğini H6507 hatasına çevirir.
func (o *Optimizer) recoverStateOp(fn string, op func()) error {
	_, err := bind.Recover(func() bool {
		op()
		return true
	}, func(r any) error {
		return messages.New(messages.H6507, fn, strings.TrimPrefix(fmt.Sprint(r), "tensor: "), o.ad, len(o.params))
	})
	return err
}

// paramOrder "m.3" gibi bir anahtarın parametre sırasını (3) verir.
func (o *Optimizer) paramOrder(ad string) (int, bool) {
	_, order, ok := strings.Cut(ad, ".")
	if !ok {
		return 0, false
	}
	k, err := strconv.Atoi(order)
	if err != nil || k < 0 || k >= len(o.params) || strconv.Itoa(k) != order {
		return 0, false
	}
	return k, true
}

// CheckState durum girdilerinin (adlar ve şekiller) bu iyileştiriciye uyduğunu, hiçbir
// şey yüklemeden denetler; kontrol noktası bütün nesneleri yüklemeden önce bununla sınar.
func (o *Optimizer) CheckState(fn string, names []string, shapes [][]int) error {
	for i, ad := range names {
		if err := o.checkStateEntry(fn, ad, shapes[i]); err != nil {
			return err
		}
	}
	return nil
}

// checkStateEntry bir durum girdisinin anahtarını ve şeklini iyileştiriciyle
// karşılaştırır (yükleme yarıda kalmasın diye önceden).
func (o *Optimizer) checkStateEntry(fn, ad string, shape []int) error {
	prefix, _, _ := strings.Cut(ad, ".")
	valid := []string{"hiz", "ana"}
	if o.adamw != nil {
		valid = []string{"m", "v", "adim", "ana"}
	}
	k, sorted := o.paramOrder(ad)
	switch {
	case ad == "adim" && o.adamw != nil:
		k = -1
	case !sorted || !slices.Contains(valid, prefix):
		return messages.New(messages.H6516, fn, ad, o.ad, len(o.params), codes.DisplayName(memberState))
	}
	expected := []int{2} // adım sayaçları
	if k >= 0 && prefix != "adim" {
		expected = o.params[k].Shape()
	}
	if !slices.Equal(shape, expected) {
		return messages.New(messages.H6518, fn, ad, tensor.ShapeText(shape), tensor.ShapeText(expected))
	}
	return nil
}

package checkpoint

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Stateful, durumu kontrol noktasına yazılıp geri yüklenebilen motor değerleridir: bugün
// Örnekleyici ve SıralıOkuyucu (internal/stdlib/dataset), ileride Üreteç (rastgele_üreteç). TR'de
// x.durum (Y502) ve x.durumu_yükle(sözlük) (Y503) üyelerinin Go yüzüdür; bir motor yeni bir
// tipi kontrol noktasına bu iki yöntemi yazarak katar (bu paketi içe aktarmasına gerek yoktur).
type Stateful interface {
	interp.Value
	// StateDict değerin durumunu yalnızca düz değerlerden (Tam, Ondalık, Metin, Mantık,
	// yok, Liste, Sözlük) oluşan bir sözlük olarak verir.
	StateDict(m *interp.VM) (*interp.DictVal, error)
	// LoadState durumu geri yükler; kayıt bu değere uymuyorsa (başka küme, tohum ...) yol
	// gösterici bir hata verir ve değeri değiştirmez.
	LoadState(m *interp.VM, s *interp.DictVal) error
}

// object kontrol_noktası sözlüğündeki bir girdidir.
type object struct {
	name  string
	kind  objectKind
	value interp.Value

	names    []string          // model: tensörlerin yol adları
	params   []*ml.TensorValue // model: tensörler (parametreler ve türev izlemeyen durum tensörleri)
	op       *ml.Optimizer
	stateful Stateful
	log      *logging.Log
}

// splitObjects kontrol_noktası'na verilen sözlüğü nesnelere ayırır ve her birinin
// kaydedilebilir olduğunu denetler.
func splitObjects(fn string, s *interp.DictVal) ([]*object, error) {
	var out []*object
	for _, k := range s.Order {
		v, ok := s.Pairs[k]
		if !ok {
			continue
		}
		if _, text := s.KeyValue(k).(*interp.StringVal); !text {
			return nil, messages.New(messages.H7434, fn, codes.Name(codes.A202), s.DisplayKey(k))
		}
		if err := checkObjectName(k); err != nil {
			return nil, messages.New(messages.H2000, fn, err)
		}
		n, err := newObject(fn, k, v)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if len(out) == 0 || len(out) > maxObjects {
		return nil, messages.New(messages.H7435,
			fn, maxObjects, codes.Name(codes.T702))
	}
	return out, nil
}

// newObject bir değerin türünü belirler.
func newObject(fn, name string, v interp.Value) (*object, error) {
	n := &object{name: name, value: v}
	if g, ok := logging.IsLog(v); ok {
		n.kind, n.log = kindLog, g
		return n, nil
	}
	if op, ok := ml.IsOptimizer(v); ok {
		n.kind, n.op = kindOptimizer, op
		return n, nil
	}
	if d, ok := v.(Stateful); ok {
		n.kind, n.stateful = kindStateful, d
		return n, nil
	}
	// Model: bütün tensörleri, türev izleyenler (parametreler) ve izlemeyenler (durum: TR ile
	// yazılmış iyileştiricilerin momentleri, sabit tablolar) birlikte saklanır.
	names, params, err := ml.CollectTensors(fn, v)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		n.kind, n.names, n.params = kindModel, names, params
		return n, nil
	}
	switch x := v.(type) {
	case *interp.DictVal, *interp.ListVal:
		if _, err := writePlain(x); err != nil {
			return nil, messages.New(messages.H7401, fn, name, err)
		}
		n.kind = kindPlain
		return n, nil
	case *interp.IntVal, *interp.FloatVal, *interp.StringVal, *interp.BoolVal:
		return nil, messages.New(messages.H7436,
			fn, name, x.Type(), name)
	case *ml.TensorValue:
		return nil, messages.New(messages.H7437, fn, name, interp.TypeAndSummary(v))
	}
	return nil, messages.New(messages.H7438,
		fn, name, interp.TypeAndSummary(v))
}

// tensorPlan kaydedilecek bir tensördür: dosyadaki adı ve kaynağı.
type tensorPlan struct {
	name string
	t    *tensor.Tensor
}

// tensors nesnenin dosyaya yazılacak tensörleridir ("<nesne>/<ad>"). İyileştirici
// durumu kopyasız görünümlerdir (cihazda ek bellek yok); yazılırken konağa indirilir.
func (n *object) tensors() []tensorPlan {
	var out []tensorPlan
	switch n.kind {
	case kindModel:
		for i, name := range n.names {
			out = append(out, tensorPlan{name: n.name + "/" + name, t: n.params[i].Tensor()})
		}
	case kindOptimizer:
		names, tensors := n.op.StateView()
		for i, name := range names {
			out = append(out, tensorPlan{name: n.name + "/" + name, t: tensors[i]})
		}
	}
	return out
}

// record nesnenin durum.json kaydıdır (tensörler hariç).
func (n *object) record(m *interp.VM) (objectRecord, error) {
	k := objectRecord{Name: n.name, Kind: n.kind.String()}
	var err error
	switch n.kind {
	case kindOptimizer:
		k.Optimizer, k.ParamCount = n.op.Name(), n.op.ParamCount()
	case kindLog:
		k.Length, err = n.log.Len()
	case kindStateful:
		var s *interp.DictVal
		if s, err = n.stateful.StateDict(m); err == nil {
			k.Value, err = writePlain(s)
		}
	case kindPlain:
		k.Value, err = writePlain(n.value)
	}
	if err != nil {
		return k, messages.New(messages.H7439, n.name, err)
	}
	return k, nil
}

// pending bir noktadan okunan, bir nesneye ait kayıtlardır.
type pending struct {
	record objectRecord
	shapes map[string][]int // tensör adı (nesne öneki olmadan) → şekil
	kinds  map[string]tensor.DType
	order  []string // tensörlerin dosyadaki sırası
	value  interp.Value
}

// checkMatch kaydın şimdiki nesneye uyduğunu, hiçbir şey yüklemeden denetler. surum
// noktanın biçim sürümüdür (modellerin kaydı sürüme bağlıdır, bkz. modelMatch).
func (n *object) checkMatch(fn string, y *pending, version int) error {
	if y.record.Kind != n.kind.String() {
		return messages.New(messages.H7440, fn, n.name, y.record.Kind, n.kind)
	}
	switch n.kind {
	case kindModel:
		return n.modelMatch(messages.Text(messages.H7441, fn, n.name), y, version)
	case kindOptimizer:
		if y.record.Optimizer != n.op.Name() || y.record.ParamCount != n.op.ParamCount() {
			return messages.New(messages.H7442,
				fn, n.name, y.record.ParamCount, y.record.Optimizer, n.op.ParamCount(), n.op.Name())
		}
		shapes := make([][]int, len(y.order))
		for i, name := range y.order {
			shapes[i] = y.shapes[name]
		}
		return n.op.CheckState(messages.Text(messages.H7441, fn, n.name), y.order, shapes)
	case kindStateful, kindPlain:
		if len(y.shapes) > 0 {
			return messages.New(messages.H7443, fn, n.name)
		}
		v, err := readPlain(y.record.Value)
		if err != nil {
			return messages.New(messages.H7444, fn, n.name, err)
		}
		y.value = v
		if n.kind == kindStateful {
			if _, ok := v.(*interp.DictVal); !ok {
				return messages.New(messages.H7445, fn, n.name)
			}
		} else if fmt.Sprint(v.Type()) != fmt.Sprint(n.value.Type()) {
			return messages.New(messages.H7446, fn, n.name, v.Type(), n.value.Type())
		}
	}
	return nil
}

// modelMatch modelin tensörlerini kayıttakilerle karşılaştırır: her tensör kayıtta aynı
// şekil ve türde bulunmalı, kayıtta modelde olmayan tensör olmamalıdır. Sürüm 1 noktalarında
// yalnızca türev izleyen tensörler kaydedilmişti: orada kayıtta olmayan türev izlemeyen
// tensörler (ör. sabit tablolar) olduğu gibi kalır, kayıttaki tensörler modelde türev izlesin
// izlemesin yerlerine yazılır.
func (n *object) modelMatch(fn string, y *pending, version int) error {
	found := 0
	for i, name := range n.names {
		t := n.params[i].Tensor()
		sk, ok := y.shapes[name]
		if !ok {
			if version == 1 && !t.RequiresGrad() {
				continue
			}
			what := messages.Text(messages.H7447)
			if !t.RequiresGrad() {
				what = messages.Text(messages.H7448)
			}
			return messages.New(messages.H7449, fn, name, what, len(n.names), len(y.shapes))
		}
		found++
		if sm := t.Shape(); !slices.Equal(sm, sk) {
			return messages.New(messages.H7450, fn, name, shapeText(sm), shapeText(sk))
		}
		if tm, tk := t.DType(), y.kinds[name]; tm != tk {
			return messages.New(messages.H7451, fn, name, tm, tk)
		}
	}
	if found == len(y.shapes) {
		return nil
	}
	for _, name := range y.order {
		if !slices.Contains(n.names, name) {
			if o := turkish.Closest(name, n.names); o != "" {
				return messages.New(messages.H7453, fn, name, o)
			}
			return messages.New(messages.H7452, fn, name)
		}
	}
	return nil
}

// shapeText bir şekli [2, 3] biçiminde yazar.
func shapeText(s []int) string { return strings.ReplaceAll(fmt.Sprint(s), " ", ", ") }

// settingsText programın komut satırı ayarlarıdır (gizliler maskeli, karşılaştırma için).
func settingsText(m *interp.VM, a *interp.DictVal) json.RawMessage {
	if a == nil {
		return nil
	}
	b, err := writePlainFiltered(a, m.Mask)
	if err != nil {
		return nil
	}
	return b
}

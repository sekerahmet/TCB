package ml

import (
	"slices"
	"strconv"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bir modelin parametrelerini toplama (parametreler) ve var olan parametrelere yükleme
// (parametreleri_yükle). Kontrol noktası (internal/stdlib/checkpoint) aynı Go fonksiyonlarını
// kullanır: CollectParams, CollectTensors, CheckParamShapes, CopyInto.

// paramsBuiltin parametreler(model, adlarıyla: yanlış) yerleşiğidir: bir yapı
// nesnesini (iç içe yapılar, listeler ve sözlüklerle birlikte) gezip türev izleyen yaprak
// tensörleri sabit sırayla (yapı alanlarının tanım sırası, liste sırası, sözlük ekleme
// sırası) toplar. Aynı tensör bir kez yer alır. adlarıyla: doğru yol adı → tensör sözlüğü
// verir ("bloklar.0.dikkat.Wq"); bu sözlük tensör_kaydet ile kaydedilebilir.
func paramsBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T530)
	if len(args) != 1 {
		return nil, messages.New(messages.H6500, fn, nameWithNames.TR, codes.DisplayName(codes.A041), len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, nameWithNames); err != nil {
		return nil, err
	}
	withNames, err := namedBool(fn, named, nameWithNames, false)
	if err != nil {
		return nil, err
	}
	names, found, err := CollectParams(fn, args[0])
	if err != nil {
		return nil, err
	}
	if !withNames {
		el := make([]interp.Value, len(found))
		for i, d := range found {
			el[i] = d
		}
		return &interp.ListVal{Elements: el}, nil
	}
	s := interp.NewDictVal()
	for i, ad := range names {
		s.Set(ad, found[i])
	}
	return s, nil
}

// CollectParams bir modelin (yapı nesnesi; iç içe yapılar, listeler ve sözlüklerle; ya
// da tek bir tensör) türev izleyen yaprak tensörlerini parametreler(model, adlarıyla: doğru)
// ile aynı sırayla ve yol adlarıyla verir; aynı tensör bir kez yer alır, model tek bir
// tensörse adı "0"dır. fn hata mesajının başıdır. Model hiç parametre içermeyebilir.
func CollectParams(fn string, model interp.Value) (names []string, found []*TensorValue, failure error) {
	return collectLeaves(fn, model, false)
}

// CollectTensors CollectParams gibidir, ama türev izlemeyen tensörleri de (durum:
// iyileştirici momentleri, sabit tablolar) aynı gezinti sırasıyla ve yol adlarıyla verir.
// Ara sonuçlar (türev izleyen işlem sonuçları) yine atlanır. Kontrol noktası nesnenin bütün
// tensörlerini bununla toplar.
func CollectTensors(fn string, model interp.Value) (names []string, found []*TensorValue, failure error) {
	return collectLeaves(fn, model, true)
}

// collectLeaves modeli gezer; hepsi yanlışsa yalnızca türev izleyen yaprakları toplar.
func collectLeaves(fn string, model interp.Value, all bool) (names []string, found []*TensorValue, failure error) {
	seen := map[*tensor.Tensor]bool{}
	walked := map[any]bool{}
	nameOwner := map[string]*tensor.Tensor{}
	var walk func(v interp.Value, fpath string)
	walk = func(v interp.Value, fpath string) {
		if failure != nil {
			return
		}
		subPath := func(ad string) string {
			if fpath == "" {
				return ad
			}
			return fpath + "." + ad
		}
		switch x := v.(type) {
		case *TensorValue:
			if x.intermediate || (!all && !x.t.RequiresGrad()) || seen[x.t] {
				return
			}
			if o, ok := nameOwner[fpath]; ok && o != x.t {
				failure = messages.New(messages.H6505, fn, fpath)
				return
			}
			seen[x.t] = true
			nameOwner[fpath] = x.t
			found = append(found, x)
			names = append(names, fpath)
		case *interp.StructInstanceVal:
			if !walked[x] {
				walked[x] = true
				x.EachField(func(ad string, d interp.Value) { walk(d, subPath(ad)) })
			}
		case *interp.ListVal:
			if !walked[x] {
				walked[x] = true
				for i, el := range x.Elements {
					walk(el, subPath(strconv.Itoa(i)))
				}
			}
		case *interp.DictVal:
			if !walked[x] {
				walked[x] = true
				for _, k := range x.Order {
					walk(x.Pairs[k], subPath(x.DisplayKey(k)))
				}
			}
		}
	}
	walk(model, "")
	if failure != nil {
		return nil, nil, failure
	}
	for i, ad := range names {
		if ad == "" {
			names[i] = "0" // model tek bir tensörse
		}
	}
	return names, found, nil
}

// CheckParamShapes modelin parametrelerini (CollectParams sonucu) yüklenecek kaynağın
// ad → şekil eşlemesiyle karşılaştırır: her parametre kaynakta aynı şekilde bulunmalı,
// kaynakta modelde olmayan bir ad olmamalıdır. srcName mesajda kaynağı anar ("sözlükte",
// "kayıtta"). İlk uyumsuzluk yol gösterici bir hatadır; hiçbir şey yazılmaz.
func CheckParamShapes(fn string, names []string, found []*TensorValue, src map[string][]int, srcName string) error {
	for i, ad := range names {
		sk, ok := src[ad]
		if !ok {
			return messages.New(messages.H6508, fn, ad, srcName, len(names), len(src))
		}
		if sm := found[i].t.Shape(); !slices.Equal(sm, sk) {
			return messages.New(messages.H6511, fn, ad, shapeText(sm), srcName, shapeText(sk))
		}
	}
	if len(src) == len(names) {
		return nil
	}
	var extra []string
	for ad := range src {
		if !slices.Contains(names, ad) {
			extra = append(extra, ad)
		}
	}
	slices.Sort(extra)
	if o := turkish.Closest(extra[0], names); o != "" {
		return messages.New(messages.H6532, fn, srcName, len(extra), extra[0], o)
	}
	return messages.New(messages.H6531, fn, srcName, len(extra), extra[0])
}

// CopyInto kaynağın değerlerini p'ye yerinde yazar: kaynak p'nin cihazına taşınır, türü
// farklıysa dönüştürülür (şekiller aynı olmalıdır). İşlem çizgeye kaydedilmez; p'nin sürüm
// sayacı artar (p'yi kullanmış eski bir çizgeden geri yayılım hata verir, 15.6). İyileştirici
// p'nin kendisini tuttuğu için güncellemeler yüklenen değerlerle sürer. Çekirdek panikleri
// (cihaz belleği) çağıranın kurtarıcısına gider.
func CopyInto(p, src *tensor.Tensor) {
	if src.RequiresGrad() {
		src = src.Detach()
	}
	p.CopyFrom(src.To(p.Device()))
}

// loadParamsBuiltin parametreleri_yükle(model, sözlük) yerleşiğidir: ad → tensör
// sözlüğünü (tensör_yükle sonucu) modelin var olan parametrelerine yerinde yazar ve yazılan
// parametre sayısını verir.
func loadParamsBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T703)
	if len(args) != 2 {
		return nil, interp.ArgCountError(codes.T703, "2", len(args), messages.Text(messages.H2327), messages.Text(messages.H2328))
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	names, found, err := CollectParams(fn, args[0])
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, messages.New(messages.H6520, fn, interp.TypeAndSummary(args[0]), gradExample("..."))
	}
	s, ok := args[1].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.T703, 2, args[1], codes.A206)
	}
	src := make(map[string]*tensor.Tensor, len(s.Order))
	shapes := make(map[string][]int, len(s.Order))
	for _, k := range s.Order {
		v, ok := s.Pairs[k]
		if !ok {
			continue
		}
		if _, text := s.KeyValue(k).(*interp.StringVal); !text {
			return nil, messages.New(messages.H6522, fn, codes.DisplayName(codes.A202), s.DisplayKey(k))
		}
		d, ok := v.(*TensorValue)
		if !ok {
			return nil, messages.New(messages.H6524, fn, k, codes.DisplayName(codes.A212), typeName(v))
		}
		src[k], shapes[k] = d.t, d.t.Shape()
	}
	if err := CheckParamShapes(fn, names, found, shapes, messages.Text(messages.H6533)); err != nil {
		return nil, err
	}
	if _, err := safe(func() bool {
		for i, ad := range names {
			CopyInto(found[i].t, src[ad])
		}
		return true
	}); err != nil {
		return nil, messages.New(messages.H2000, fn, panicReason(err))
	}
	return interp.NewInt(int64(len(names))), nil
}

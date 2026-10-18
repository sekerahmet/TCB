package yurut

import (
	"errors"
	"fmt"

	"tr_dili/internal/dil/ipucu"
	"tr_dili/internal/dil/kod"
)

// Tip değerleri (3.11): tip(x) değerin tipini bir tip değeri olarak verir; tip adları (Tam,
// Liste, Tensör ...) ifadede bu değerlerdir. Değer dilden bağımsızdır: tip kodunu (A2xx) ya da
// yapı nesnelerinde yapının tanımını taşır. Yazdırılınca adı çalıştırma dilinde görünür;
// karşılaştırma değerle yapılır (tip(x) == Tam, tip(p) == Nokta). Böylece kodlu saklanan bir
// program hangi dilde gösterilirse gösterilsin aynı sonucu verir.

// VAL_TYPE tip değerlerinin tipidir (A215 Tip).
var VAL_TYPE = TipAdi(kod.A215)

// TipVal bir tip değeridir.
type TipVal struct {
	// Kod yerleşik ve motor tiplerinin kodudur (A2xx); yapı nesnelerinin tipinde kod.Yok.
	Kod kod.Kod
	// Yapi yapı nesnelerinin tipidir (tip(p) → Nokta); diğer tiplerde nil.
	Yapi *StructDefVal
}

func (t *TipVal) Type() ValueType { return VAL_TYPE }
func (t *TipVal) IsTruthy() bool  { return true }
func (t *TipVal) Inspect() string {
	if t.Yapi != nil {
		return t.Yapi.Name
	}
	return kod.Ad(t.Kod)
}

// Esit tip değerlerinin eşitliğidir: aynı tip kodu ya da aynı yapı. Bir yapı nesnesinin tipi
// yapının kendisine de eşittir (tip(p) == Nokta).
func (t *TipVal) Esit(b Value) bool {
	switch x := b.(type) {
	case *TipVal:
		return t.Kod == x.Kod && t.Yapi == x.Yapi
	case *StructDefVal:
		return t.Yapi == x
	}
	return false
}

// tipDegerleri canlı tip kodlarının (A2xx) paylaşılan tip değerleridir (kodun sayısına göre
// dizi; çalışma anında ayırma ve ad araması yok). tipDegeriAdla motor tiplerinin değer
// tipinden (ValueType: kodun adı) tip değerine eşlemesidir.
var (
	tipDegerleri  [300]*TipVal
	tipDegeriAdla = map[ValueType]*TipVal{}
)

func init() {
	for _, k := range kod.Kodlar() {
		if k.Alan() == kod.AlanTip {
			t := &TipVal{Kod: k}
			tipDegerleri[k.Sayi()] = t
			tipDegeriAdla[TipAdi(k)] = t
		}
	}
}

// TipDegeri bir tip kodunun (A2xx) tip değeridir.
func TipDegeri(k kod.Kod) *TipVal {
	if k.Alan() != kod.AlanTip || tipDegerleri[k.Sayi()] == nil {
		panic(fmt.Sprintf("TipDegeri: %s canlı bir tip kodu (A2xx) değil", k))
	}
	return tipDegerleri[k.Sayi()]
}

// DegerinTipi tip(v)'nin sonucudur: v'nin tip değeri.
func DegerinTipi(v Value) *TipVal {
	switch x := v.(type) {
	case *IntVal:
		return tipDegerleri[kod.A200.Sayi()]
	case *FloatVal:
		return tipDegerleri[kod.A201.Sayi()]
	case *StringVal:
		return tipDegerleri[kod.A202.Sayi()]
	case *BoolVal:
		return tipDegerleri[kod.A203.Sayi()]
	case *NullVal:
		return tipDegerleri[kod.A204.Sayi()]
	case *ListVal:
		return tipDegerleri[kod.A205.Sayi()]
	case *DictVal:
		return tipDegerleri[kod.A206.Sayi()]
	case *StructInstanceVal:
		return x.Def.tipDegeri()
	}
	if t := tipDegeriAdla[v.Type()]; t != nil {
		return t
	}
	return tipDegerleri[kod.A213.Sayi()] // Herhangi: tipi kod listesinde olmayan değer
}

// tipDegeri yapının nesnelerinin tip değeridir; ilk kullanımda kurulur (yorumlayıcı kilidi
// altında).
func (s *StructDefVal) tipDegeri() *TipVal {
	if s.tip == nil {
		s.tip = &TipVal{Yapi: s}
	}
	return s.tip
}

// tipleriKaydet tip adlarını genel kapsama tip değerleri olarak bağlar (tip(x) == Tam).
func (v *VM) tipleriKaydet() {
	for _, k := range kod.Kodlar() {
		if k.Alan() == kod.AlanTip {
			for _, y := range kod.Yazimlar(k) {
				v.GlobalEnv.Define(y, tipDegerleri[k.Sayi()], true)
			}
		}
	}
}

// tipMetinHatasi tip değeri metinle karşılaştırılıyorsa yol gösteren hatayı verir (tip
// değeri hiçbir metne eşit değildir; sessizce yanlış dönmek yerine hata). Değilse nil.
func tipMetinHatasi(op string, a, b Value) error {
	t, ok := a.(*TipVal)
	m, ok2 := b.(*StringVal)
	if !ok || !ok2 {
		t, ok = b.(*TipVal)
		m, ok2 = a.(*StringVal)
		if !ok || !ok2 {
			return nil
		}
	}
	return errors.New(ipucu.TipMetinKarsilastirmasi(op, m.Val, t.Inspect()))
}

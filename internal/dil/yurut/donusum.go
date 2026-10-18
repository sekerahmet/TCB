package yurut

import (
	"errors"
	"fmt"
)

func GoToTRValue(raw interface{}) Value {
	switch v := raw.(type) {
	case nil:
		return NULL
	case bool:
		return BoolToVal(v)
	case int:
		return NewInt(int64(v))
	case int64:
		return NewInt(v)
	case float64:
		if v == float64(int64(v)) {
			return NewInt(int64(v))
		}
		return &FloatVal{Val: v}
	case string:
		return &StringVal{Val: v}
	case []interface{}:
		elems := make([]Value, len(v))
		for i, el := range v {
			elems[i] = GoToTRValue(el)
		}
		return &ListVal{Elements: elems}
	case map[string]interface{}:
		d := NewDictVal()
		for k, val := range v {
			d.Set(k, GoToTRValue(val))
		}
		return d
	default:
		return &StringVal{Val: fmt.Sprintf("%v", v)}
	}
}

// ErrDongusel kendini (doğrudan ya da dolaylı) içeren bir liste/sözlüğün dış biçime (JSON,
// Go değerleri) yazılmak istendiğini bildirir: l = [1]; l.ekle(l); json_metin(l).
var ErrDongusel = errors.New("değer kendini içeriyor (döngüsel liste ya da sözlük); JSON'a yazılamaz")

// DisDegerDerinligi dış biçime yazılabilecek en derin iç içe liste/sözlük seviyesidir;
// JSON okumadaki sınırla (maxJSONDepth) aynıdır, böylece yazılan her değer geri okunabilir.
const DisDegerDerinligi = maxJSONDepth

// DerinlikHatasi DisDegerDerinligi aşıldığında dönen hatadır.
func DerinlikHatasi() error {
	return fmt.Errorf("değer çok derin iç içe (en fazla %d seviye)", DisDegerDerinligi)
}

// TRToGo bir TR değerini Go değerine (nil, bool, int64, float64, string, []interface{},
// map[string]interface{}) çevirir. Döngüsel değerlerde ErrDongusel, DisDegerDerinligi'nden
// derin değerlerde açık bir hata döner; böylece yığın taşmasıyla süreç çökmez. Aynı alt
// değerin birden çok yerde geçmesi (döngü olmadan paylaşım) serbesttir.
func TRToGo(val Value) (interface{}, error) {
	g := DisGezgin{}
	return g.cevir(val, 0)
}

// TRToGoValue TRToGo'nun hatasız biçimidir; yalnızca döngüsüz olduğu bilinen değerler
// (testler) içindir. Döngüsel ya da çok derin değerde panikler.
func TRToGoValue(val Value) interface{} {
	g, err := TRToGo(val)
	if err != nil {
		panic(err)
	}
	return g
}

// DisGezgin bir TR değerini dış biçime yazarken döngüyü ve derinliği denetler. Değeri
// başka bir biçime (ör. sıralı JSON) yazan kod her liste/sözlüğe girerken Gir, çıkarken
// Cik çağırır; sıfır değeri kullanıma hazırdır.
type DisGezgin struct {
	// acik şu an yazılmakta olan (atası olunan) kapsayıcılardır; yalnızca bunlara geri
	// dönmek döngüdür.
	acik map[Value]bool
}

// Gir v kapsayıcısına derinlik seviyesinde girer; döngü ya da aşırı derinlikte hata verir.
func (g *DisGezgin) Gir(v Value, derinlik int) error {
	if derinlik > DisDegerDerinligi {
		return DerinlikHatasi()
	}
	if g.acik == nil {
		g.acik = map[Value]bool{}
	}
	if g.acik[v] {
		return ErrDongusel
	}
	g.acik[v] = true
	return nil
}

// Cik v kapsayıcısından çıkar.
func (g *DisGezgin) Cik(v Value) { delete(g.acik, v) }

func (g *DisGezgin) cevir(val Value, derinlik int) (interface{}, error) {
	switch v := val.(type) {
	case *NullVal:
		return nil, nil
	case *BoolVal:
		return v.Val, nil
	case *IntVal:
		return v.Val, nil
	case *FloatVal:
		return v.Val, nil
	case *StringVal:
		return v.Val, nil
	case *ListVal:
		if err := g.Gir(v, derinlik); err != nil {
			return nil, err
		}
		defer g.Cik(v)
		res := make([]interface{}, len(v.Elements))
		for i, el := range v.Elements {
			c, err := g.cevir(el, derinlik+1)
			if err != nil {
				return nil, err
			}
			res[i] = c
		}
		return res, nil
	case *DictVal:
		if err := g.Gir(v, derinlik); err != nil {
			return nil, err
		}
		defer g.Cik(v)
		res := make(map[string]interface{}, len(v.Pairs))
		for k, el := range v.Pairs {
			// JSON anahtarları metin olmak zorundadır; metin olmayan anahtarlar
			// görünen hâlleriyle yazılır (1 -> "1").
			c, err := g.cevir(el, derinlik+1)
			if err != nil {
				return nil, err
			}
			res[v.DisplayKey(k)] = c
		}
		return res, nil
	case DuzDegerli:
		// Dış değerler (ör. tensör → iç içe sayı listesi)
		return g.cevir(v.DuzDeger(), derinlik+1)
	default:
		return v.Inspect(), nil
	}
}

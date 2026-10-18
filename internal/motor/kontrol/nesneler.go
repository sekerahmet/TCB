package kontrol

import (
	"encoding/json"
	"fmt"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/gunluk"
	"tr_dili/internal/motor/ogrenme"
)

// Durumlu, durumu kontrol noktasına yazılıp geri yüklenebilen motor değerleridir: bugün
// Örnekleyici ve SıralıOkuyucu (internal/motor/veri), ileride Üreteç (rastgele_üreteç). TR'de
// x.durum (Y502) ve x.durumu_yükle(sözlük) (Y503) üyelerinin Go yüzüdür; bir motor yeni bir
// tipi kontrol noktasına bu iki yöntemi yazarak katar (bu paketi içe aktarmasına gerek yoktur).
type Durumlu interface {
	yurut.Value
	// DurumSozlugu değerin durumunu yalnızca düz değerlerden (Tam, Ondalık, Metin, Mantık,
	// yok, Liste, Sözlük) oluşan bir sözlük olarak verir.
	DurumSozlugu(m *yurut.VM) (*yurut.DictVal, error)
	// DurumuYukle durumu geri yükler; kayıt bu değere uymuyorsa (başka küme, tohum ...) yol
	// gösterici bir hata verir ve değeri değiştirmez.
	DurumuYukle(m *yurut.VM, s *yurut.DictVal) error
}

// nesne kontrol_noktası sözlüğündeki bir girdidir.
type nesne struct {
	ad    string
	tur   nesneTuru
	deger yurut.Value

	adlar   []string         // model: parametre yol adları
	params  []*ogrenme.Deger // model: parametreler
	op      *ogrenme.Iyilestirici
	durumlu Durumlu
	gunluk  *gunluk.Gunluk
}

// nesneleriAyir kontrol_noktası'na verilen sözlüğü nesnelere ayırır ve her birinin
// kaydedilebilir olduğunu denetler.
func nesneleriAyir(fn string, s *yurut.DictVal) ([]*nesne, error) {
	var out []*nesne
	for _, k := range s.Order {
		v, ok := s.Pairs[k]
		if !ok {
			continue
		}
		if _, metin := s.KeyValue(k).(*yurut.StringVal); !metin {
			return nil, fmt.Errorf("%s: sözlük anahtarları nesne adları (%s) olmalıdır, %s verildi", fn, kod.Ad(kod.A202), s.DisplayKey(k))
		}
		if err := nesneAdiDenetle(k); err != nil {
			return nil, fmt.Errorf("%s: %v", fn, err)
		}
		n, err := nesneKur(fn, k, v)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if len(out) == 0 || len(out) > enFazlaNesne {
		return nil, fmt.Errorf("%s: kaydedilecek nesneler sözlüğü 1 ile %d arası nesne içermeli, ör. %s(\"koşular/deneme1\", {model: model, op: op})",
			fn, enFazlaNesne, kod.Ad(kod.T702))
	}
	return out, nil
}

// nesneKur bir değerin türünü belirler.
func nesneKur(fn, ad string, v yurut.Value) (*nesne, error) {
	n := &nesne{ad: ad, deger: v}
	if g, ok := gunluk.GunlukMu(v); ok {
		n.tur, n.gunluk = turGunluk, g
		return n, nil
	}
	if op, ok := ogrenme.IyilestiriciMi(v); ok {
		n.tur, n.op = turIyilestirici, op
		return n, nil
	}
	if d, ok := v.(Durumlu); ok {
		n.tur, n.durumlu = turDurumlu, d
		return n, nil
	}
	adlar, params, err := ogrenme.ParametreleriTopla(fn, v)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		n.tur, n.adlar, n.params = turModel, adlar, params
		return n, nil
	}
	switch x := v.(type) {
	case *yurut.DictVal, *yurut.ListVal:
		if _, err := duzYaz(x); err != nil {
			return nil, fmt.Errorf("%s: %q: %v", fn, ad, err)
		}
		n.tur = turDuz
		return n, nil
	case *yurut.IntVal, *yurut.FloatVal, *yurut.StringVal, *yurut.BoolVal:
		return nil, fmt.Errorf("%s: %q bir %s; düz bir değer doğrudan verilemez (geri yüklenirken yerine yazılamaz). Bir sözlüğe koyun: {%s: {değer: ...}}",
			fn, ad, x.Type(), ad)
	case *ogrenme.Deger:
		return nil, fmt.Errorf("%s: %q türev izlemeyen bir tensör; parametre değil. Model parametreleri %s ile oluşturulur", fn, ad, yurut.TipVeOzet(v))
	}
	return nil, fmt.Errorf("%s: %q kaydedilemez (%s); modeller, iyileştiriciler, okuyucular, günlükler ve düz değerli sözlük ya da listeler verilebilir",
		fn, ad, yurut.TipVeOzet(v))
}

// tensorPlani kaydedilecek bir tensördür: dosyadaki adı ve kaynağı.
type tensorPlani struct {
	ad string
	t  *tensor.Tensor
}

// tensorleri nesnenin dosyaya yazılacak tensörleridir ("<nesne>/<ad>"). İyileştirici
// durumu kopyasız görünümlerdir (cihazda ek bellek yok); yazılırken konağa indirilir.
func (n *nesne) tensorleri() []tensorPlani {
	var out []tensorPlani
	switch n.tur {
	case turModel:
		for i, ad := range n.adlar {
			out = append(out, tensorPlani{ad: n.ad + "/" + ad, t: n.params[i].Tensor()})
		}
	case turIyilestirici:
		adlar, tensorler := n.op.DurumGorunumu()
		for i, ad := range adlar {
			out = append(out, tensorPlani{ad: n.ad + "/" + ad, t: tensorler[i]})
		}
	}
	return out
}

// kaydi nesnenin durum.json kaydıdır (tensörler hariç).
func (n *nesne) kaydi(m *yurut.VM) (nesneKaydi, error) {
	k := nesneKaydi{Ad: n.ad, Tur: n.tur.String()}
	var err error
	switch n.tur {
	case turIyilestirici:
		k.Iyilestirici, k.ParametreSayisi = n.op.Ad(), n.op.ParametreSayisi()
	case turGunluk:
		k.Uzunluk, err = n.gunluk.Uzunluk()
	case turDurumlu:
		var s *yurut.DictVal
		if s, err = n.durumlu.DurumSozlugu(m); err == nil {
			k.Deger, err = duzYaz(s)
		}
	case turDuz:
		k.Deger, err = duzYaz(n.deger)
	}
	if err != nil {
		return k, fmt.Errorf("%q: %v", n.ad, err)
	}
	return k, nil
}

// yuklenecek bir noktadan okunan, bir nesneye ait kayıtlardır.
type yuklenecek struct {
	kayit    nesneKaydi
	sekiller map[string][]int // tensör adı (nesne öneki olmadan) → şekil
	turler   map[string]tensor.DType
	sira     []string // tensörlerin dosyadaki sırası
	deger    yurut.Value
}

// uyumDenetle kaydın şimdiki nesneye uyduğunu, hiçbir şey yüklemeden denetler.
func (n *nesne) uyumDenetle(fn string, y *yuklenecek) error {
	if y.kayit.Tur != n.tur.String() {
		return fmt.Errorf("%s: %q kayıtta bir %s, şimdi bir %s; aynı nesnelerle sürdürün ya da yeni bir koşu klasörü verin", fn, n.ad, y.kayit.Tur, n.tur)
	}
	switch n.tur {
	case turModel:
		if err := ogrenme.ParametreUyumu(fmt.Sprintf("%s: %q", fn, n.ad), n.adlar, n.params, y.sekiller, "kayıtta"); err != nil {
			return err
		}
		for i, ad := range n.adlar {
			if t, s := n.params[i].Tensor().DType(), y.turler[ad]; t != s {
				return fmt.Errorf("%s: %q: %q parametresinin türü modelde %s, kayıtta %s; aynı veri türüyle sürdürün", fn, n.ad, ad, t, s)
			}
		}
	case turIyilestirici:
		if y.kayit.Iyilestirici != n.op.Ad() || y.kayit.ParametreSayisi != n.op.ParametreSayisi() {
			return fmt.Errorf("%s: %q kayıtta %d parametreli bir %s, şimdi %d parametreli bir %s; aynı iyileştiriciyle sürdürün",
				fn, n.ad, y.kayit.ParametreSayisi, y.kayit.Iyilestirici, n.op.ParametreSayisi(), n.op.Ad())
		}
		sekiller := make([][]int, len(y.sira))
		for i, ad := range y.sira {
			sekiller[i] = y.sekiller[ad]
		}
		return n.op.DurumuDenetle(fmt.Sprintf("%s: %q", fn, n.ad), y.sira, sekiller)
	case turDurumlu, turDuz:
		if len(y.sekiller) > 0 {
			return fmt.Errorf("%s: %q için kayıtta beklenmeyen tensörler var", fn, n.ad)
		}
		v, err := duzOku(y.kayit.Deger)
		if err != nil {
			return fmt.Errorf("%s: %q nesnesinin kaydı bozuk: %v", fn, n.ad, err)
		}
		y.deger = v
		if n.tur == turDurumlu {
			if _, ok := v.(*yurut.DictVal); !ok {
				return fmt.Errorf("%s: %q nesnesinin durumu bir sözlük değil", fn, n.ad)
			}
		} else if fmt.Sprint(v.Type()) != fmt.Sprint(n.deger.Type()) {
			return fmt.Errorf("%s: %q kayıtta %s, şimdi %s", fn, n.ad, v.Type(), n.deger.Type())
		}
	}
	return nil
}

// ayarlarYazimi programın komut satırı ayarlarıdır (gizliler maskeli, karşılaştırma için).
func ayarlarYazimi(m *yurut.VM, a *yurut.DictVal) json.RawMessage {
	if a == nil {
		return nil
	}
	b, err := duzYazSuzgecli(a, m.Maskele)
	if err != nil {
		return nil
	}
	return b
}

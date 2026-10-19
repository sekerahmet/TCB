package checkpoint

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Durumlu, durumu kontrol noktasına yazılıp geri yüklenebilen motor değerleridir: bugün
// Örnekleyici ve SıralıOkuyucu (internal/stdlib/dataset), ileride Üreteç (rastgele_üreteç). TR'de
// x.durum (Y502) ve x.durumu_yükle(sözlük) (Y503) üyelerinin Go yüzüdür; bir motor yeni bir
// tipi kontrol noktasına bu iki yöntemi yazarak katar (bu paketi içe aktarmasına gerek yoktur).
type Durumlu interface {
	interp.Value
	// DurumSozlugu değerin durumunu yalnızca düz değerlerden (Tam, Ondalık, Metin, Mantık,
	// yok, Liste, Sözlük) oluşan bir sözlük olarak verir.
	DurumSozlugu(m *interp.VM) (*interp.DictVal, error)
	// DurumuYukle durumu geri yükler; kayıt bu değere uymuyorsa (başka küme, tohum ...) yol
	// gösterici bir hata verir ve değeri değiştirmez.
	DurumuYukle(m *interp.VM, s *interp.DictVal) error
}

// nesne kontrol_noktası sözlüğündeki bir girdidir.
type nesne struct {
	ad    string
	tur   nesneTuru
	deger interp.Value

	adlar   []string    // model: tensörlerin yol adları
	params  []*ml.Deger // model: tensörler (parametreler ve türev izlemeyen durum tensörleri)
	op      *ml.Iyilestirici
	durumlu Durumlu
	gunluk  *logging.Gunluk
}

// nesneleriAyir kontrol_noktası'na verilen sözlüğü nesnelere ayırır ve her birinin
// kaydedilebilir olduğunu denetler.
func nesneleriAyir(fn string, s *interp.DictVal) ([]*nesne, error) {
	var out []*nesne
	for _, k := range s.Order {
		v, ok := s.Pairs[k]
		if !ok {
			continue
		}
		if _, metin := s.KeyValue(k).(*interp.StringVal); !metin {
			return nil, fmt.Errorf("%s: sözlük anahtarları nesne adları (%s) olmalıdır, %s verildi", fn, codes.Ad(codes.A202), s.DisplayKey(k))
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
		return nil, fmt.Errorf("%s: kaydedilecek nesneler sözlüğü 1 ile %d arası nesne içermeli, ör. %s(\"runs/deneme1\", {model: model, op: op})",
			fn, enFazlaNesne, codes.Ad(codes.T702))
	}
	return out, nil
}

// nesneKur bir değerin türünü belirler.
func nesneKur(fn, ad string, v interp.Value) (*nesne, error) {
	n := &nesne{ad: ad, deger: v}
	if g, ok := logging.GunlukMu(v); ok {
		n.tur, n.gunluk = turGunluk, g
		return n, nil
	}
	if op, ok := ml.IyilestiriciMi(v); ok {
		n.tur, n.op = turIyilestirici, op
		return n, nil
	}
	if d, ok := v.(Durumlu); ok {
		n.tur, n.durumlu = turDurumlu, d
		return n, nil
	}
	// Model: bütün tensörleri, türev izleyenler (parametreler) ve izlemeyenler (durum: TR ile
	// yazılmış iyileştiricilerin momentleri, sabit tablolar) birlikte saklanır.
	adlar, params, err := ml.TensorleriTopla(fn, v)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		n.tur, n.adlar, n.params = turModel, adlar, params
		return n, nil
	}
	switch x := v.(type) {
	case *interp.DictVal, *interp.ListVal:
		if _, err := duzYaz(x); err != nil {
			return nil, fmt.Errorf("%s: %q: %v", fn, ad, err)
		}
		n.tur = turDuz
		return n, nil
	case *interp.IntVal, *interp.FloatVal, *interp.StringVal, *interp.BoolVal:
		return nil, fmt.Errorf("%s: %q bir %s; düz bir değer doğrudan verilemez (geri yüklenirken yerine yazılamaz). Bir sözlüğe koyun: {%s: {değer: ...}}",
			fn, ad, x.Type(), ad)
	case *ml.Deger:
		return nil, fmt.Errorf("%s: %q bir işlemin sonucu (ara tensör, %s); kaydedilecek tensörler yaprak olmalıdır: parametreler ve türevsiz güncellenen durum tensörleri", fn, ad, interp.TipVeOzet(v))
	}
	return nil, fmt.Errorf("%s: %q kaydedilemez (%s); modeller, iyileştiriciler, okuyucular, günlükler ve düz değerli sözlük ya da listeler verilebilir",
		fn, ad, interp.TipVeOzet(v))
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
func (n *nesne) kaydi(m *interp.VM) (nesneKaydi, error) {
	k := nesneKaydi{Ad: n.ad, Tur: n.tur.String()}
	var err error
	switch n.tur {
	case turIyilestirici:
		k.Iyilestirici, k.ParametreSayisi = n.op.Ad(), n.op.ParametreSayisi()
	case turGunluk:
		k.Uzunluk, err = n.gunluk.Uzunluk()
	case turDurumlu:
		var s *interp.DictVal
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
	deger    interp.Value
}

// uyumDenetle kaydın şimdiki nesneye uyduğunu, hiçbir şey yüklemeden denetler. surum
// noktanın biçim sürümüdür (modellerin kaydı sürüme bağlıdır, bkz. modelUyumu).
func (n *nesne) uyumDenetle(fn string, y *yuklenecek, surum int) error {
	if y.kayit.Tur != n.tur.String() {
		return fmt.Errorf("%s: %q kayıtta bir %s, şimdi bir %s; aynı nesnelerle sürdürün ya da yeni bir koşu klasörü verin", fn, n.ad, y.kayit.Tur, n.tur)
	}
	switch n.tur {
	case turModel:
		return n.modelUyumu(fmt.Sprintf("%s: %q", fn, n.ad), y, surum)
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
			if _, ok := v.(*interp.DictVal); !ok {
				return fmt.Errorf("%s: %q nesnesinin durumu bir sözlük değil", fn, n.ad)
			}
		} else if fmt.Sprint(v.Type()) != fmt.Sprint(n.deger.Type()) {
			return fmt.Errorf("%s: %q kayıtta %s, şimdi %s", fn, n.ad, v.Type(), n.deger.Type())
		}
	}
	return nil
}

// modelUyumu modelin tensörlerini kayıttakilerle karşılaştırır: her tensör kayıtta aynı
// şekil ve türde bulunmalı, kayıtta modelde olmayan tensör olmamalıdır. Sürüm 1 noktalarında
// yalnızca türev izleyen tensörler kaydedilmişti: orada kayıtta olmayan türev izlemeyen
// tensörler (ör. sabit tablolar) olduğu gibi kalır, kayıttaki tensörler modelde türev izlesin
// izlemesin yerlerine yazılır.
func (n *nesne) modelUyumu(fn string, y *yuklenecek, surum int) error {
	bulunan := 0
	for i, ad := range n.adlar {
		t := n.params[i].Tensor()
		sk, ok := y.sekiller[ad]
		if !ok {
			if surum == 1 && !t.RequiresGrad() {
				continue
			}
			ne := "parametresi"
			if !t.RequiresGrad() {
				ne = "durum tensörü (türev izlemeyen)"
			}
			return fmt.Errorf("%s: modelin %q %s kayıtta yok (model %d, kayıt %d tensör); model ile kayıt aynı yapıda olmalı", fn, ad, ne, len(n.adlar), len(y.sekiller))
		}
		bulunan++
		if sm := t.Shape(); !slices.Equal(sm, sk) {
			return fmt.Errorf("%s: %q tensörünün şekli modelde %s, kayıtta %s", fn, ad, sekilYazisi(sm), sekilYazisi(sk))
		}
		if tm, tk := t.DType(), y.turler[ad]; tm != tk {
			return fmt.Errorf("%s: %q tensörünün türü modelde %s, kayıtta %s; aynı veri türüyle sürdürün", fn, ad, tm, tk)
		}
	}
	if bulunan == len(y.sekiller) {
		return nil
	}
	for _, ad := range y.sira {
		if !slices.Contains(n.adlar, ad) {
			oneri := ""
			if o := turkish.EnYakin(ad, n.adlar); o != "" {
				oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
			}
			return fmt.Errorf("%s: kayıttaki %q tensörü modelde yok%s", fn, ad, oneri)
		}
	}
	return nil
}

// sekilYazisi bir şekli [2, 3] biçiminde yazar.
func sekilYazisi(s []int) string { return strings.ReplaceAll(fmt.Sprint(s), " ", ", ") }

// ayarlarYazimi programın komut satırı ayarlarıdır (gizliler maskeli, karşılaştırma için).
func ayarlarYazimi(m *interp.VM, a *interp.DictVal) json.RawMessage {
	if a == nil {
		return nil
	}
	b, err := duzYazSuzgecli(a, m.Maskele)
	if err != nil {
		return nil
	}
	return b
}

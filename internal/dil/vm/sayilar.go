package vm

import (
	"fmt"
	"math"
)

// Yerleşiklerin sayı kuralları işleçlerle aynıdır: Tam sayılar tam (int64) olarak
// hesaplanır ve karşılaştırılır, sınırı aşan sonuç sessizce sarmak yerine "Tam sayı
// taşması" hatası verir (3.1); Ondalık'a çevrim yalnızca bir Ondalık işlenen varsa olur.

// tamSiniri 2^63'tür: int64'e çevrilebilecek Ondalık değerler [-2^63, 2^63) aralığındadır.
const tamSiniri = 9223372036854775808.0

// ondaliktanTam bir Ondalık değeri (kesirli kısmı atılmış) Tam'a çevirir; NaN, sonsuz ya
// da int64 aralığı dışındaki değerde hata verir.
func ondaliktanTam(ad string, f float64) (int64, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f >= tamSiniri || f < -tamSiniri {
		return 0, fmt.Errorf("%s: %s Tam sayıya çevrilemez (Tam sayı taşması; Tam en fazla 64 bitlik bir tam sayıdır)", ad, (&FloatVal{Val: f}).Inspect())
	}
	return int64(f), nil
}

// tamOndalikKarsilastir bir Tam ile (NaN olmayan) bir Ondalığı tam olarak karşılaştırır
// (Ondalığa çevirmek 2^53 üstünde eşitlik uydururdu): a < f ise -1, eşitse 0, büyükse 1.
func tamOndalikKarsilastir(a int64, f float64) int {
	if f >= tamSiniri {
		return -1
	}
	if f < -tamSiniri {
		return 1
	}
	t := math.Trunc(f)
	ti := int64(t)
	switch {
	case a < ti:
		return -1
	case a > ti:
		return 1
	case f > t:
		return -1
	case f < t:
		return 1
	}
	return 0
}

// sayiKarsilastir iki sayıyı (Tam ya da Ondalık) tam olarak karşılaştırır. ok yanlışsa
// değerlerden biri sayı değildir ya da NaN'dır (karşılaştırılamaz).
func sayiKarsilastir(a, b Value) (int, bool) {
	switch x := a.(type) {
	case *IntVal:
		switch y := b.(type) {
		case *IntVal:
			switch {
			case x.Val < y.Val:
				return -1, true
			case x.Val > y.Val:
				return 1, true
			}
			return 0, true
		case *FloatVal:
			if math.IsNaN(y.Val) {
				return 0, false
			}
			return tamOndalikKarsilastir(x.Val, y.Val), true
		}
	case *FloatVal:
		if math.IsNaN(x.Val) {
			return 0, false
		}
		switch y := b.(type) {
		case *IntVal:
			return -tamOndalikKarsilastir(y.Val, x.Val), true
		case *FloatVal:
			switch {
			case math.IsNaN(y.Val):
				return 0, false
			case x.Val < y.Val:
				return -1, true
			case x.Val > y.Val:
				return 1, true
			}
			return 0, true
		}
	}
	return 0, false
}

// sayiMi değerin Tam ya da Ondalık olup olmadığını söyler.
func sayiMi(v Value) bool {
	switch v.(type) {
	case *IntVal, *FloatVal:
		return true
	}
	return false
}

// sayiTopla bir listenin elemanlarını işleçlerle aynı kuralla toplar: hepsi Tam ise sonuç
// Tam'dır ve taşma hata verir; bir Ondalık varsa sonuç Ondalık'tır. Sayı olmayan eleman
// hatadır. ad hata mesajındaki yerleşiğin adıdır.
func sayiTopla(ad string, elems []Value) (Value, error) {
	var tam int64
	var ondalik float64
	ondalikVar := false
	for i, el := range elems {
		switch x := el.(type) {
		case *IntVal:
			if ondalikVar {
				ondalik += float64(x.Val)
				continue
			}
			s := tam + x.Val
			if (s > tam) != (x.Val > 0) {
				return nil, fmt.Errorf("%s: Tam sayı taşması (%d + %d, listenin %d. elemanında); ondalık toplam için elemanları ondalık(...) ile çevirin", ad, tam, x.Val, i+1)
			}
			tam = s
		case *FloatVal:
			if !ondalikVar {
				ondalikVar = true
				ondalik = float64(tam)
			}
			ondalik += x.Val
		default:
			return nil, ListeElemaniHatasi(ad, i, sayiTipi, el)
		}
	}
	if ondalikVar {
		return &FloatVal{Val: ondalik}, nil
	}
	return NewInt(tam), nil
}

// ListeElemaniHatasi bir listenin i. (0'dan başlayan) elemanının yanlış tipte olduğunu
// bildirir.
func ListeElemaniHatasi(ad string, i int, beklenen string, verilen Value) error {
	return fmt.Errorf("%s: listenin %d. elemanı %s olmalıdır, %s verildi", ad, i+1, beklenen, TipVeOzet(verilen))
}

// enUc min/maks'ın ortak gövdesidir: değerlerin en küçüğünü (buyuk yanlışsa) ya da en
// büyüğünü, değerin kendisini (tipi korunarak) döndürür. Tam'lar tam karşılaştırılır; sayı
// olmayan değer hatadır; NaN varsa sonuç NaN'dır. Boş girdide yok döner.
func enUc(ad string, elems []Value, buyuk bool, liste bool) (Value, error) {
	if len(elems) == 0 {
		return NULL, nil
	}
	var secilen Value
	for i, el := range elems {
		if !sayiMi(el) {
			if liste {
				return nil, ListeElemaniHatasi(ad, i, sayiTipi, el)
			}
			return nil, ArgumanTipHatasi(ad, i+1, sayiTipi, el)
		}
		if f, ok := el.(*FloatVal); ok && math.IsNaN(f.Val) {
			return f, nil
		}
		if secilen == nil {
			secilen = el
			continue
		}
		c, _ := sayiKarsilastir(el, secilen)
		if (buyuk && c > 0) || (!buyuk && c < 0) {
			secilen = el
		}
	}
	return secilen, nil
}

// aralikAdimlari bir aralığın gezilecek ilk ve son değerini ve adımını verir; bos doğruysa
// aralıkta hiç değer yoktur. Son değer dahil hesaplandığı için int64 sınırındaki aralıklar
// (0..=9223372036854775807) taşmadan gezilir.
func aralikAdimlari(r *RangeVal) (ilk, son, adim int64, bos bool) {
	ilk, son, adim = r.Start, r.End, 1
	switch {
	case r.Start < r.End:
		if !r.Inclusive {
			son = r.End - 1
		}
	case r.Start > r.End:
		adim = -1
		if !r.Inclusive {
			son = r.End + 1
		}
	default: // Start == End
		bos = !r.Inclusive
	}
	return ilk, son, adim, bos
}

// dilimUclari a..b (ya da a..=b) dilimini n uzunluklu bir kabın [bas, son) aralığına
// çevirir: negatif uç, indekste olduğu gibi sondan sayar (-1 son eleman); uçlar kabın
// sınırlarına kırpılır, bas > son ise dilim boştur.
func dilimUclari(r *RangeVal, n int) (bas, son int) {
	b, s := r.Start, r.End
	if b < 0 {
		b += int64(n)
	}
	if s < 0 {
		s += int64(n)
	}
	if r.Inclusive && s < math.MaxInt64 {
		s++
	}
	kirp := func(x int64) int {
		if x < 0 {
			return 0
		}
		if x > int64(n) {
			return n
		}
		return int(x)
	}
	bas, son = kirp(b), kirp(s)
	if bas > son {
		bas = son
	}
	return bas, son
}

package logging

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// olcuDegeri bir ölçü değerini günlüğe yazılacak düz değere çevirir: sayı, metin, mantık,
// yok, liste ve sözlük olduğu gibi kalır; tek elemanlı tensör (kayıp) sayı, daha büyük tensör
// iç içe liste olur. Değer hemen çevrilir: tensörün türev çizgesi sonraki adıma taşınmaz.
func olcuDegeri(fn, ad string, v interp.Value) (interp.Value, error) {
	switch x := v.(type) {
	case *interp.IntVal, *interp.FloatVal, *interp.StringVal, *interp.BoolVal, *interp.NullVal, *interp.ListVal, *interp.DictVal:
		return v, nil
	case *ml.Deger:
		if t := x.Tensor(); t.Numel() == 1 {
			return &interp.FloatVal{Val: kisaOndalik(t.Item())}, nil
		}
		return x.PlainValue(), nil
	}
	return nil, fmt.Errorf("%s: '%s' ölçüsünün değeri %s ya da %s olmalı; %s verildi",
		fn, ad, interp.TypeNames(codes.A200, codes.A201, codes.A202, codes.A203), codes.Name(codes.A212), interp.TypeAndSummary(v))
}

// kisaOndalik float32'yi en kısa yazımıyla float64'e çevirir (0.1 → 0.1, 0.10000000149 değil).
func kisaOndalik(x float32) float64 {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(float64(x), 'g', -1, 32), 64)
	return f
}

// olcuYazimi bir ölçünün çubuktaki kısa yazımıdır: sayılar 3 anlamlı basamakla (Türkçe
// ondalık virgülüyle), metin kısaltılarak.
func olcuYazimi(v interp.Value) string {
	switch x := v.(type) {
	case *interp.FloatVal:
		return sayiYazimi(x.Val)
	case *interp.IntVal:
		return strconv.FormatInt(x.Val, 10)
	case *interp.StringVal:
		if r := []rune(x.Val); len(r) > 20 {
			return string(r[:20]) + "…"
		}
		return x.Val
	case *interp.ListVal:
		if len(x.Elements) == 1 {
			return olcuYazimi(x.Elements[0])
		}
		return fmt.Sprintf("[%d]", len(x.Elements))
	case *interp.DictVal:
		return fmt.Sprintf("{%d}", len(x.Pairs))
	}
	return v.Inspect()
}

// sayiYazimi bir ondalığı 3 anlamlı basamakla yazar: 2.31 → "2,31", 85.24 → "85,2",
// 0.001 → "0,001", 0.000312 → "3,12e-4", 1234567 → "1,23e6". Çok küçük ve çok büyük
// sayılar bilimsel yazımla gösterilir.
func sayiYazimi(f float64) string {
	switch {
	case math.IsNaN(f) || math.IsInf(f, 0):
		return (&interp.FloatVal{Val: f}).Inspect()
	case f == 0:
		return "0"
	}
	a := math.Abs(f)
	if a >= 1e-3 && a < 1e5 {
		basamak := max(0, 2-int(math.Floor(math.Log10(a))))
		s := strconv.FormatFloat(f, 'f', basamak, 64)
		if strings.Contains(s, ".") {
			s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		}
		return strings.Replace(s, ".", ",", 1)
	}
	s := strconv.FormatFloat(f, 'e', 2, 64) // 3.12e-04
	govde, us, _ := strings.Cut(s, "e")
	if strings.Contains(govde, ".") {
		govde = strings.TrimRight(strings.TrimRight(govde, "0"), ".")
	}
	n, _ := strconv.Atoi(us)
	return strings.Replace(govde, ".", ",", 1) + "e" + strconv.Itoa(n)
}

// sureYazimi geçen süreyi yazar: "45 sn", "3 dk 2 sn", "61 dk 3 sn", "2 sa 5 dk".
func sureYazimi(sn float64) string {
	t := int64(math.Round(sn))
	switch {
	case t < 60:
		return fmt.Sprintf("%d sn", t)
	case t < 2*3600:
		if t%60 == 0 {
			return fmt.Sprintf("%d dk", t/60)
		}
		return fmt.Sprintf("%d dk %d sn", t/60, t%60)
	}
	return fmt.Sprintf("%d sa %d dk", t/3600, t%3600/60)
}

// kalanYazimi kalan süre tahminini kaba yazar: "45 sn", "14 dk", "2 sa 5 dk".
func kalanYazimi(sn float64) string {
	t := int64(math.Round(sn))
	switch {
	case t < 60:
		return fmt.Sprintf("%d sn", max(t, 1))
	case t < 3600:
		return fmt.Sprintf("%d dk", (t+30)/60)
	}
	return fmt.Sprintf("%d sa %d dk", t/3600, t%3600/60)
}

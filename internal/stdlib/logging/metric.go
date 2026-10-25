package logging

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// metricValue bir ölçü değerini günlüğe yazılacak düz değere çevirir: sayı, metin, mantık,
// yok, liste ve sözlük olduğu gibi kalır; tek elemanlı tensör (kayıp) sayı, daha büyük tensör
// iç içe liste olur. Değer hemen çevrilir: tensörün türev çizgesi sonraki adıma taşınmaz.
func metricValue(fn, name string, v interp.Value) (interp.Value, error) {
	switch x := v.(type) {
	case *interp.IntVal, *interp.FloatVal, *interp.StringVal, *interp.BoolVal, *interp.NullVal, *interp.ListVal, *interp.DictVal:
		return v, nil
	case *ml.TensorValue:
		if t := x.Tensor(); t.Numel() == 1 {
			return &interp.FloatVal{Val: shortDecimal(t.Item())}, nil
		}
		return x.PlainValue(), nil
	}
	return nil, messages.New(messages.H7300,
		fn, name, interp.TypeNames(codes.A200, codes.A201, codes.A202, codes.A203), codes.DisplayName(codes.A212), interp.TypeAndSummary(v))
}

// shortDecimal float32'yi en kısa yazımıyla float64'e çevirir (0.1 → 0.1, 0.10000000149 değil).
func shortDecimal(x float32) float64 {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(float64(x), 'g', -1, 32), 64)
	return f
}

// metricText bir ölçünün çubuktaki kısa yazımıdır: sayılar 3 anlamlı basamakla (Türkçe
// ondalık virgülüyle), metin kısaltılarak.
func metricText(v interp.Value) string {
	switch x := v.(type) {
	case *interp.FloatVal:
		return formatNumber(x.Val)
	case *interp.IntVal:
		return strconv.FormatInt(x.Val, 10)
	case *interp.StringVal:
		if r := []rune(x.Val); len(r) > 20 {
			return string(r[:20]) + "…"
		}
		return x.Val
	case *interp.ListVal:
		if len(x.Elements) == 1 {
			return metricText(x.Elements[0])
		}
		return fmt.Sprintf("[%d]", len(x.Elements))
	case *interp.DictVal:
		return fmt.Sprintf("{%d}", len(x.Pairs))
	}
	return v.Inspect()
}

// formatNumber bir ondalığı 3 anlamlı basamakla yazar: 2.31 → "2,31", 85.24 → "85,2",
// 0.001 → "0,001", 0.000312 → "3,12e-4", 1234567 → "1,23e6". Çok küçük ve çok büyük
// sayılar bilimsel yazımla gösterilir.
func formatNumber(f float64) string {
	switch {
	case math.IsNaN(f) || math.IsInf(f, 0):
		return (&interp.FloatVal{Val: f}).Text(codes.Display()) // çubuğun satırı görüntüdür (TR_DIL)
	case f == 0:
		return "0"
	}
	a := math.Abs(f)
	if a >= 1e-3 && a < 1e5 {
		digits := max(0, 2-int(math.Floor(math.Log10(a))))
		s := strconv.FormatFloat(f, 'f', digits, 64)
		if strings.Contains(s, ".") {
			s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		}
		return strings.Replace(s, ".", ",", 1)
	}
	s := strconv.FormatFloat(f, 'e', 2, 64) // 3.12e-04
	body, exp, _ := strings.Cut(s, "e")
	if strings.Contains(body, ".") {
		body = strings.TrimRight(strings.TrimRight(body, "0"), ".")
	}
	n, _ := strconv.Atoi(exp)
	return strings.Replace(body, ".", ",", 1) + "e" + strconv.Itoa(n)
}

// formatDuration geçen süreyi yazar: "45 sn", "3 dk 2 sn", "61 dk 3 sn", "2 sa 5 dk".
func formatDuration(sec float64) string {
	t := int64(math.Round(sec))
	switch {
	case t < 60:
		return messages.Text(messages.H7301, t)
	case t < 2*3600:
		if t%60 == 0 {
			return messages.Text(messages.H7302, t/60)
		}
		return messages.Text(messages.H7303, t/60, t%60)
	}
	return messages.Text(messages.H7304, t/3600, t%3600/60)
}

// remainingText kalan süre tahminini kaba yazar: "45 sn", "14 dk", "2 sa 5 dk".
func remainingText(sec float64) string {
	t := int64(math.Round(sec))
	switch {
	case t < 60:
		return messages.Text(messages.H7301, max(t, 1))
	case t < 3600:
		return messages.Text(messages.H7302, (t+30)/60)
	}
	return messages.Text(messages.H7304, t/3600, t%3600/60)
}

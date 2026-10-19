package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"math"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------------
// Yazdırma
// ----------------------------------------------------------------------------

// yazdirmaSiniri bu sayıdan çok elemanı olan tensörlerde her eksenin yalnızca
// başı ve sonu yazılır.
const yazdirmaSiniri = 1000

// kenarEleman kısaltılmış yazdırmada eksen başına baştan ve sondan gösterilen eleman sayısı.
const kenarEleman = 3

func (d *Deger) Inspect() string {
	t := d.t
	sh := t.Shape()
	var ek []string
	// Tür yalnızca float32 dışındaki türlerde gösterilir: "Tensör[2×3] bf16".
	turEki := ""
	if t.DType() != tensor.Float32 {
		turEki = " " + turAdi(t.DType())
	}
	if t.Device() != tensor.CPU {
		ek = append(ek, t.Device().String())
	}
	if t.RequiresGrad() {
		ek = append(ek, "türev izleniyor")
	}
	if len(sh) == 0 {
		x := t.Item()
		s := sayiMetni(x, bilimselMi([]float32{x}))
		if d.tamSayili {
			s = strconv.FormatInt(int64(x), 10)
		}
		if len(ek) > 0 {
			s += ", " + strings.Join(ek, ", ")
		}
		return string(interp.VAL_TENSOR) + "(" + s + ")" + turEki
	}
	baslik := string(interp.VAL_TENSOR) + "[" + sekilCarpimi(sh) + "]" + turEki
	if len(ek) > 0 {
		baslik += " (" + strings.Join(ek, ", ") + ")"
	}
	kisalt := t.Numel() > yazdirmaSiniri
	// Gösterilecek hücreler: kısaltılmamış tensörde hepsi (tek kopya), kısaltılmışta
	// yalnızca eksen başı/sonu hücreleri tek tek okunur; tüm veri kopyalanıp
	// biçimlenmez. Hücreler gezinme sırasıyla toplanır, sonra aynı sırayla yazılır.
	var veri []float32
	if !kisalt {
		veri = t.Data()
	}
	adimlar := make([]int, len(sh))
	s := 1
	for i := len(sh) - 1; i >= 0; i-- {
		adimlar[i] = s
		s *= sh[i]
	}
	idx := make([]int, len(sh))
	var hucreler []float32
	var topla func(eksen, ofset int)
	topla = func(eksen, ofset int) {
		for _, i := range gosterilecek(sh[eksen], kisalt) {
			if i < 0 {
				continue
			}
			idx[eksen] = i
			if eksen < len(sh)-1 {
				topla(eksen+1, ofset+i*adimlar[eksen])
			} else if kisalt {
				hucreler = append(hucreler, t.At(idx...))
			} else {
				hucreler = append(hucreler, veri[ofset+i])
			}
		}
	}
	topla(0, 0)
	// Tüm hücreler aynı biçimde ve genişlikte yazılır.
	bilimsel := bilimselMi(hucreler)
	metinler := make([]string, len(hucreler))
	genislik := 0
	for i, x := range hucreler {
		if d.tamSayili {
			metinler[i] = strconv.FormatInt(int64(x), 10)
		} else {
			metinler[i] = sayiMetni(x, bilimsel)
		}
		genislik = max(genislik, len([]rune(metinler[i])))
	}
	sira := 0
	var sb strings.Builder
	sb.WriteString(baslik + " ")
	var yaz func(eksen int, girinti string)
	yaz = func(eksen int, girinti string) {
		n := sh[eksen]
		indeksler := gosterilecek(n, kisalt)
		if eksen == len(sh)-1 {
			sb.WriteString("[")
			for k, i := range indeksler {
				if k > 0 {
					sb.WriteString(", ")
				}
				if i < 0 {
					sb.WriteString("…")
					continue
				}
				m := metinler[sira]
				sira++
				for p := len([]rune(m)); p < genislik; p++ {
					sb.WriteByte(' ')
				}
				sb.WriteString(m)
			}
			sb.WriteString("]")
			return
		}
		ic := girinti + "  "
		sb.WriteString("[\n")
		for k, i := range indeksler {
			if k > 0 {
				sb.WriteString(",\n")
			}
			sb.WriteString(ic)
			if i < 0 {
				sb.WriteString("…")
				continue
			}
			yaz(eksen+1, ic)
		}
		sb.WriteString("\n" + girinti + "]")
	}
	yaz(0, "")
	return sb.String()
}

// bilimselMi sabit noktalı (%.4f) gösterimin bilgi kaybettireceği durumda bilimsel
// gösterime geçilip geçilmeyeceğini söyler: sıfır olmayan bir değer 0.0001'den küçükse
// (0.0000 görünürdü) ya da bir değer 1e8 ve üzerindeyse.
func bilimselMi(x []float32) bool {
	for _, v := range x {
		a := math.Abs(float64(v))
		if math.IsNaN(a) || math.IsInf(a, 0) {
			continue
		}
		if (a != 0 && a < 1e-4) || a >= 1e8 {
			return true
		}
	}
	return false
}

// gosterilecek bir eksende yazılacak indeksleri verir; -1 "…" yerini gösterir.
func gosterilecek(n int, kisalt bool) []int {
	if !kisalt || n <= 2*kenarEleman {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, 0, 2*kenarEleman+1)
	for i := 0; i < kenarEleman; i++ {
		out = append(out, i)
	}
	out = append(out, -1)
	for i := n - kenarEleman; i < n; i++ {
		out = append(out, i)
	}
	return out
}

// sayiMetni bir hücreyi yazar. Sayı olmayan değerler Türkçe ve tutarlıdır:
// "Tanımsız" (NaN), "Sonsuz", "-Sonsuz".
func sayiMetni(x float32, bilimsel bool) string {
	f := float64(x)
	switch {
	case math.IsNaN(f):
		return "Tanımsız"
	case math.IsInf(f, 1):
		return "Sonsuz"
	case math.IsInf(f, -1):
		return "-Sonsuz"
	}
	if bilimsel {
		return strconv.FormatFloat(f, 'e', 4, 64)
	}
	return strconv.FormatFloat(f, 'f', 4, 64)
}

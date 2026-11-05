package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"math"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------------
// Yazdırma
// ----------------------------------------------------------------------------

// printLimit bu sayıdan çok elemanı olan tensörlerde her eksenin yalnızca
// başı ve sonu yazılır.
const printLimit = 1000

// edgeItems kısaltılmış yazdırmada eksen başına baştan ve sondan gösterilen eleman sayısı.
const edgeItems = 3

var _ interp.LanguageText = (*TensorValue)(nil)

// Inspect is the value's text (3.14): language-independent, the same as its program text.
func (d *TensorValue) Inspect() string { return d.Text(codes.ValueNames()) }

// Text is the tensor's text in lang (docs/spec/03_tipler.md): the type name, the non-finite
// numbers (the same names as Ondalık's, TR_val_inf and TR_val_nan) and the notes are lang's; str(t) and
// yazdır(t) give it in the language of the file they are written in.
func (d *TensorValue) Text(lang *codes.Language) string {
	t := d.t
	sh := t.Shape()
	var ek []string
	// Tür yalnız cinsinin varsayılanı dışındaki hassasiyette gösterilir (15.5): "Tensör[2×3] bf16",
	// "Tensör[3] uint8"; int64 ve bool yazılmaz (cins öğelerden okunur).
	dtypeSuffix := ""
	if !t.DType().IsDefault() {
		dtypeSuffix = " " + t.DType().Name()
	}
	if t.Device() != tensor.CPU {
		ek = append(ek, t.Device().String())
	}
	if t.RequiresGrad() {
		ek = append(ek, messages.TextIn(lang.Code, messages.H6814))
	}
	if len(sh) == 0 {
		s := d.cellTexts(nil, false, lang)[0]
		if len(ek) > 0 {
			s += ", " + strings.Join(ek, ", ")
		}
		return lang.Name(codes.TypeTensor) + "(" + s + ")" + dtypeSuffix
	}
	header := lang.Name(codes.TypeTensor) + "[" + shapeProduct(sh) + "]" + dtypeSuffix
	if len(ek) > 0 {
		header += " (" + strings.Join(ek, ", ") + ")"
	}
	shorten := t.Numel() > printLimit
	texts := d.cellTexts(sh, shorten, lang)
	width := 0
	for _, m := range texts {
		width = max(width, len([]rune(m)))
	}
	order := 0
	var sb strings.Builder
	sb.WriteString(header + " ")
	var write func(axis int, indent string)
	write = func(axis int, indent string) {
		n := sh[axis]
		indices := toShow(n, shorten)
		if axis == len(sh)-1 {
			sb.WriteString("[")
			for k, i := range indices {
				if k > 0 {
					sb.WriteString(", ")
				}
				if i < 0 {
					sb.WriteString("…")
					continue
				}
				m := texts[order]
				order++
				for p := len([]rune(m)); p < width; p++ {
					sb.WriteByte(' ')
				}
				sb.WriteString(m)
			}
			sb.WriteString("]")
			return
		}
		ic := indent + "  "
		sb.WriteString("[\n")
		for k, i := range indices {
			if k > 0 {
				sb.WriteString(",\n")
			}
			sb.WriteString(ic)
			if i < 0 {
				sb.WriteString("…")
				continue
			}
			write(axis+1, ic)
		}
		sb.WriteString("\n" + indent + "]")
	}
	write(0, "")
	return sb.String()
}

// cellTexts gösterilecek hücrelerin metinleri, gezinme sırasıyla: kısaltılmamış tensörde hepsi,
// kısaltılmışta yalnız eksen başı ve sonu (toShow). Ondalık hücreler aynı biçimde (sabit ya da
// bilimsel) yazılır; tam sayılar ondalıksız, mantık değerleri lang'ın adıyla.
func (d *TensorValue) cellTexts(sh []int, shorten bool, lang *codes.Language) []string {
	t := d.t
	var flat []int // gösterilecek hücrelerin düz konumları
	steps := tensor.ContiguousStrides(sh)
	var walk func(axis, offset int)
	walk = func(axis, offset int) {
		if axis == len(sh) {
			flat = append(flat, offset)
			return
		}
		for _, i := range toShow(sh[axis], shorten) {
			if i >= 0 {
				walk(axis+1, offset+i*steps[axis])
			}
		}
	}
	walk(0, 0)
	texts := make([]string, len(flat))
	switch t.Elem() {
	case tensor.ElemInt:
		vals := t.Int64s()
		for i, o := range flat {
			texts[i] = strconv.FormatInt(vals[o], 10)
		}
	case tensor.ElemBool:
		vals := t.Int64s()
		for i, o := range flat {
			texts[i] = (&interp.BoolVal{Val: vals[o] != 0}).Text(lang)
		}
	default:
		cells := make([]float64, len(flat))
		if quantized := t.DType().IsQuantized(); quantized || shorten && t.DType().IsFloat() {
			// Büyük tensörün bütün verisi kopyalanmaz: yalnız gösterilecek hücreler okunur;
			// nicemlenmiş tensörde o hücrelerin blokları açılır (15.53).
			idx := make([]int, len(sh))
			for i, o := range flat {
				for a := range sh {
					idx[a] = o / steps[a] % sh[a]
				}
				if quantized {
					cells[i] = float64(t.OpenAt(idx...))
				} else {
					cells[i] = float64(t.At(idx...))
				}
			}
		} else {
			vals := t.Float64s()
			for i, o := range flat {
				cells[i] = vals[o]
			}
		}
		scientific := isScientific64(cells)
		for i, x := range cells {
			texts[i] = numberText64(x, scientific, lang)
		}
	}
	return texts
}

// isScientific64 sabit noktalı (%.4f) gösterimin bilgi kaybettireceği durumda bilimsel gösterime
// geçilir: sıfır olmayan bir değer 0.0001'den küçükse (0.0000 görünürdü) ya da bir değer 1e8 ve
// üzerindeyse.
func isScientific64(x []float64) bool {
	for _, a := range x {
		a = math.Abs(a)
		if math.IsNaN(a) || math.IsInf(a, 0) {
			continue
		}
		if (a != 0 && a < 1e-4) || a >= 1e8 {
			return true
		}
	}
	return false
}

// toShow bir eksende yazılacak indeksleri verir; -1 "…" yerini gösterir.
func toShow(n int, shorten bool) []int {
	if !shorten || n <= 2*edgeItems {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, 0, 2*edgeItems+1)
	for i := 0; i < edgeItems; i++ {
		out = append(out, i)
	}
	out = append(out, -1)
	for i := n - edgeItems; i < n; i++ {
		out = append(out, i)
	}
	return out
}

// numberText64 bir hücreyi yazar. Sonsuz ve tanımsız değerler Ondalık'la aynı adlarla, lang
// dilinde yazılır (interp.NonFiniteText: sonsuz, -sonsuz, NaN).
func numberText64(f float64, scientific bool, lang *codes.Language) string {
	if s, ok := interp.NonFiniteText(f, lang); ok {
		return s
	}
	if scientific {
		return strconv.FormatFloat(f, 'e', 4, 64)
	}
	return strconv.FormatFloat(f, 'f', 4, 64)
}

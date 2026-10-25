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

// Inspect is the tensor's display text, in the person's writing language (TR_DIL): the
// debugger and messages show it.
func (d *TensorValue) Inspect() string { return d.Text(codes.Display()) }

// Text is the tensor's text in lang (docs/spec/03_tipler.md): the type name, the non-finite
// numbers (the same names as Ondalık's, A110 and A111) and the notes are lang's; str(t) and
// yazdır(t) give it in the language of the file they are written in.
func (d *TensorValue) Text(lang *codes.Language) string {
	t := d.t
	sh := t.Shape()
	var ek []string
	// Tür yalnızca float32 dışındaki türlerde gösterilir: "Tensör[2×3] bf16".
	dtypeSuffix := ""
	if t.DType() != tensor.Float32 {
		dtypeSuffix = " " + t.DType().Name()
	}
	if t.Device() != tensor.CPU {
		ek = append(ek, t.Device().String())
	}
	if t.RequiresGrad() {
		ek = append(ek, messages.TextIn(lang.Code, messages.H6814))
	}
	if len(sh) == 0 {
		x := t.Item()
		s := numberText(x, isScientific([]float32{x}), lang)
		if d.hasInts {
			s = strconv.FormatInt(int64(x), 10)
		}
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
	// Gösterilecek hücreler: kısaltılmamış tensörde hepsi (tek kopya), kısaltılmışta
	// yalnızca eksen başı/sonu hücreleri tek tek okunur; tüm veri kopyalanıp
	// biçimlenmez. Hücreler gezinme sırasıyla toplanır, sonra aynı sırayla yazılır.
	var data []float32
	if !shorten {
		data = t.Data()
	}
	steps := make([]int, len(sh))
	s := 1
	for i := len(sh) - 1; i >= 0; i-- {
		steps[i] = s
		s *= sh[i]
	}
	idx := make([]int, len(sh))
	var cells []float32
	var sum func(axis, offset int)
	sum = func(axis, offset int) {
		for _, i := range toShow(sh[axis], shorten) {
			if i < 0 {
				continue
			}
			idx[axis] = i
			if axis < len(sh)-1 {
				sum(axis+1, offset+i*steps[axis])
			} else if shorten {
				cells = append(cells, t.At(idx...))
			} else {
				cells = append(cells, data[offset+i])
			}
		}
	}
	sum(0, 0)
	// Tüm hücreler aynı biçimde ve genişlikte yazılır.
	scientific := isScientific(cells)
	texts := make([]string, len(cells))
	width := 0
	for i, x := range cells {
		if d.hasInts {
			texts[i] = strconv.FormatInt(int64(x), 10)
		} else {
			texts[i] = numberText(x, scientific, lang)
		}
		width = max(width, len([]rune(texts[i])))
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

// isScientific sabit noktalı (%.4f) gösterimin bilgi kaybettireceği durumda bilimsel
// gösterime geçilip geçilmeyeceğini söyler: sıfır olmayan bir değer 0.0001'den küçükse
// (0.0000 görünürdü) ya da bir değer 1e8 ve üzerindeyse.
func isScientific(x []float32) bool {
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

// numberText bir hücreyi yazar. Sonsuz ve tanımsız değerler Ondalık'la aynı adlarla, lang
// dilinde yazılır (interp.NonFiniteText: sonsuz, -sonsuz, NaN).
func numberText(x float32, scientific bool, lang *codes.Language) string {
	f := float64(x)
	if s, ok := interp.NonFiniteText(f, lang); ok {
		return s
	}
	if scientific {
		return strconv.FormatFloat(f, 'e', 4, 64)
	}
	return strconv.FormatFloat(f, 'f', 4, 64)
}

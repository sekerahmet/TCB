package ml

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ----------------------------------------------------------------------------
// Hata ayıklayıcı (İzleme Masası) desteği
// ----------------------------------------------------------------------------

// WatchKind İzleme Masası'nda ikon seçimi için değerin türüdür.
func (d *TensorValue) WatchKind() string { return "tensor" }

// WatchLabel tip sütununda gösterilen özettir (ör. "Tensör [2×3]"; float32 dışındaki
// türlerde tür de eklenir: "Tensör [2×3] bf16").
func (d *TensorValue) WatchLabel() string {
	sh := d.t.Shape()
	dtypeSuffix := ""
	if dt := d.t.DType(); dt != tensor.Float32 {
		dtypeSuffix = " " + dtypeName(dt)
	}
	if len(sh) == 0 {
		return string(interp.VAL_TENSOR) + " (skaler)" + dtypeSuffix
	}
	return string(interp.VAL_TENSOR) + " [" + shapeProduct(sh) + "]" + dtypeSuffix
}

// TableData 1-2 boyutlu tensörü tabloya (sütunlar + satırlar) çevirir.
func (d *TensorValue) TableData() ([]string, [][]string, error) {
	const limit = 10000
	sh := d.t.Shape()
	data := d.t.Data()
	format := func(x float32) string { return fmt.Sprintf("%.6g", x) }
	switch len(sh) {
	case 1:
		var rows [][]string
		for i, x := range data {
			if i >= limit {
				break
			}
			rows = append(rows, []string{format(x)})
		}
		return []string{"Değer"}, rows, nil
	case 2:
		cols := make([]string, sh[1])
		for j := range cols {
			cols[j] = fmt.Sprintf("Sütun %d", j+1)
		}
		var rows [][]string
		for i := 0; i < sh[0] && i < limit; i++ {
			row := make([]string, sh[1])
			for j := range row {
				row[j] = format(data[i*sh[1]+j])
			}
			rows = append(rows, row)
		}
		return cols, rows, nil
	}
	return nil, nil, messages.New(messages.H6701, shapeText(sh))
}

// watchInlineLimit, değerleri İzleme Masası'nda ve Değişkenler panelinde satır içinde
// gösterilen tensörlerin en fazla eleman sayısıdır. Daha büyük tensörler yalnızca şekilleriyle
// özetlenir ve tablo olarak açılır.
const watchInlineLimit = 8

// WatchSummary hata ayıklayıcının değer sütununda gösterilen kısa değerdir: skaler tensörde
// sayının kendisi ("0.6074 (Tensör, skaler)"), 1-8 elemanlı tensörde iç içe değerler
// ("[[1.4298]] (Tensör, 1×1)"). float32 dışındaki türlerde tür de yazılır ("..., bf16").
// Sayılar yazdır() ile aynı biçimdedir (4 basamak; çok küçük/büyükte bilimsel). Daha büyük
// ya da boş tensörde boş metin döner: değer sütununda yalnızca etiket ve tablo bağlantısı
// görünür.
func (d *TensorValue) WatchSummary() string {
	t := d.t
	n := t.Numel()
	if n == 0 || n > watchInlineLimit {
		return ""
	}
	sh := t.Shape()
	data := t.Data()
	scientific := isScientific(data)
	number := func(x float32) string {
		if d.hasInts {
			return strconv.FormatInt(int64(x), 10)
		}
		return numberText(x, scientific)
	}
	ek := []string{string(interp.VAL_TENSOR)}
	var sb strings.Builder
	if len(sh) == 0 {
		sb.WriteString(number(data[0]))
		ek = append(ek, "skaler")
	} else {
		order := 0
		var write func(axis int)
		write = func(axis int) {
			sb.WriteByte('[')
			for i := 0; i < sh[axis]; i++ {
				if i > 0 {
					sb.WriteString(", ")
				}
				if axis == len(sh)-1 {
					sb.WriteString(number(data[order]))
					order++
				} else {
					write(axis + 1)
				}
			}
			sb.WriteByte(']')
		}
		write(0)
		ek = append(ek, shapeProduct(sh))
	}
	if dt := t.DType(); dt != tensor.Float32 {
		ek = append(ek, dtypeName(dt))
	}
	return sb.String() + " (" + strings.Join(ek, ", ") + ")"
}

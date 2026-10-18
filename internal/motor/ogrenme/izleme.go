package ogrenme

import (
	"fmt"
	"strconv"
	"strings"

	"tr_dili/internal/hesap/tensor"
)

// ----------------------------------------------------------------------------
// Hata ayıklayıcı (İzleme Masası) desteği
// ----------------------------------------------------------------------------

// IzlemeTuru İzleme Masası'nda ikon seçimi için değerin türüdür.
func (d *Deger) IzlemeTuru() string { return "tensor" }

// IzlemeEtiketi tip sütununda gösterilen özettir (ör. "Tensör [2×3]"; float32 dışındaki
// türlerde tür de eklenir: "Tensör [2×3] bf16").
func (d *Deger) IzlemeEtiketi() string {
	sh := d.t.Shape()
	turEki := ""
	if dt := d.t.DType(); dt != tensor.Float32 {
		turEki = " " + turAdi(dt)
	}
	if len(sh) == 0 {
		return "Tensör (skaler)" + turEki
	}
	return "Tensör [" + sekilCarpimi(sh) + "]" + turEki
}

// TabloVerisi 1-2 boyutlu tensörü tabloya (sütunlar + satırlar) çevirir.
func (d *Deger) TabloVerisi() ([]string, [][]string, error) {
	const sinir = 10000
	sh := d.t.Shape()
	veri := d.t.Data()
	bicim := func(x float32) string { return fmt.Sprintf("%.6g", x) }
	switch len(sh) {
	case 1:
		var rows [][]string
		for i, x := range veri {
			if i >= sinir {
				break
			}
			rows = append(rows, []string{bicim(x)})
		}
		return []string{"Değer"}, rows, nil
	case 2:
		cols := make([]string, sh[1])
		for j := range cols {
			cols[j] = fmt.Sprintf("Sütun %d", j+1)
		}
		var rows [][]string
		for i := 0; i < sh[0] && i < sinir; i++ {
			row := make([]string, sh[1])
			for j := range row {
				row[j] = bicim(veri[i*sh[1]+j])
			}
			rows = append(rows, row)
		}
		return cols, rows, nil
	}
	return nil, nil, fmt.Errorf("tablo görünümü yalnızca 1 ve 2 boyutlu tensörler için kullanılabilir (bu tensör: %s)", sekilMetni(sh))
}

// izlemeSatirIciSiniri, değerleri İzleme Masası'nda ve Değişkenler panelinde satır içinde
// gösterilen tensörlerin en fazla eleman sayısıdır. Daha büyük tensörler yalnızca şekilleriyle
// özetlenir ve tablo olarak açılır.
const izlemeSatirIciSiniri = 8

// IzlemeOzeti hata ayıklayıcının değer sütununda gösterilen kısa değerdir: skaler tensörde
// sayının kendisi ("0.6074 (Tensör, skaler)"), 1-8 elemanlı tensörde iç içe değerler
// ("[[1.4298]] (Tensör, 1×1)"). float32 dışındaki türlerde tür de yazılır ("..., bf16").
// Sayılar yazdır() ile aynı biçimdedir (4 basamak; çok küçük/büyükte bilimsel). Daha büyük
// ya da boş tensörde boş metin döner: değer sütununda yalnızca etiket ve tablo bağlantısı
// görünür.
func (d *Deger) IzlemeOzeti() string {
	t := d.t
	n := t.Numel()
	if n == 0 || n > izlemeSatirIciSiniri {
		return ""
	}
	sh := t.Shape()
	veri := t.Data()
	bilimsel := bilimselMi(veri)
	sayi := func(x float32) string {
		if d.tamSayili {
			return strconv.FormatInt(int64(x), 10)
		}
		return sayiMetni(x, bilimsel)
	}
	ek := []string{"Tensör"}
	var sb strings.Builder
	if len(sh) == 0 {
		sb.WriteString(sayi(veri[0]))
		ek = append(ek, "skaler")
	} else {
		sira := 0
		var yaz func(eksen int)
		yaz = func(eksen int) {
			sb.WriteByte('[')
			for i := 0; i < sh[eksen]; i++ {
				if i > 0 {
					sb.WriteString(", ")
				}
				if eksen == len(sh)-1 {
					sb.WriteString(sayi(veri[sira]))
					sira++
				} else {
					yaz(eksen + 1)
				}
			}
			sb.WriteByte(']')
		}
		yaz(0)
		ek = append(ek, sekilCarpimi(sh))
	}
	if dt := t.DType(); dt != tensor.Float32 {
		ek = append(ek, turAdi(dt))
	}
	return sb.String() + " (" + strings.Join(ek, ", ") + ")"
}

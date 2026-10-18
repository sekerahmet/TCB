package ogrenme

import (
	"fmt"
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

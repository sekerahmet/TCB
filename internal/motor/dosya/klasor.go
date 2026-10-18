package dosya

import (
	"fmt"
	"os"
	"path/filepath"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
)

// klasorOlusturYerlesik klasör_oluştur(yol) yerleşiğidir: klasörü üst klasörleriyle oluşturur
// (varsa sorun değildir) ve Yol olarak döndürür.
func klasorOlusturYerlesik(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := yurut.Cagri(kod.G122)
	if len(args) != 1 {
		return nil, yurut.ArgumanSayisiHatasi(kod.G122, "1", len(args), "klasör")
	}
	ham, err := yolOku(kod.G122, 1, args[0])
	if err != nil {
		return nil, err
	}
	cozulen, _ := m.YolCoz(filepath.FromSlash(ham))
	if b, err := os.Stat(cozulen); err == nil && !b.IsDir() {
		return nil, fmt.Errorf("%s: %q oluşturulamadı: aynı adda bir dosya var", fn, ham)
	}
	if err := os.MkdirAll(cozulen, 0o755); err != nil {
		return nil, turkce.DosyaHatasi(fn, ham, "oluşturulamadı", err)
	}
	return YeniYol(ham), nil
}

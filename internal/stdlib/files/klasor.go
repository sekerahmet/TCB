package files

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// klasorOlusturYerlesik klasör_oluştur(yol) yerleşiğidir: klasörü üst klasörleriyle oluşturur
// (varsa sorun değildir) ve Yol olarak döndürür.
func klasorOlusturYerlesik(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.Cagri(codes.G122)
	if len(args) != 1 {
		return nil, interp.ArgumanSayisiHatasi(codes.G122, "1", len(args), "klasör")
	}
	ham, err := yolOku(codes.G122, 1, args[0])
	if err != nil {
		return nil, err
	}
	cozulen, _ := m.YolCoz(filepath.FromSlash(ham))
	if b, err := os.Stat(cozulen); err == nil && !b.IsDir() {
		return nil, fmt.Errorf("%s: %q oluşturulamadı: aynı adda bir dosya var", fn, ham)
	}
	if err := os.MkdirAll(cozulen, 0o755); err != nil {
		return nil, turkish.DosyaHatasi(fn, ham, "oluşturulamadı", err)
	}
	return YeniYol(ham), nil
}

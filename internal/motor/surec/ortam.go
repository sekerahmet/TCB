package surec

import (
	"fmt"
	"os"
	"strings"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
)

// ortamDegiskeni ortam_değişkeni(ad) yerleşiğidir: değişkenin değeri (Metin) ya da
// tanımsızsa yok. Bütün ortamı listeleyen ya da ortamı değiştiren bir ad bilerek yoktur
// (yanlışlıkla döküm olmasın; TR alt süreç açmaz). Adı gizli bir sözcük içeren
// değişkenin değeri gizli değer süzgecine kaydedilir.
func ortamDegiskeni(m *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
	if len(args) != 1 {
		return nil, yurut.ArgumanSayisiHatasi(kod.G800, "1", len(args), "ad")
	}
	s, ok := args[0].(*yurut.StringVal)
	if !ok {
		return nil, yurut.ArgumanTipHatasi(kod.G800, 1, args[0], kod.A202)
	}
	ad := s.Val
	if ad == "" || strings.ContainsAny(ad, "=\x00") {
		return nil, fmt.Errorf("%s: %q geçerli bir ortam değişkeni adı değil (boş olamaz, '=' içeremez)", yurut.Cagri(kod.G800), ad)
	}
	deger, bulundu := os.LookupEnv(ad)
	if !bulundu {
		return yurut.NULL, nil
	}
	if GizliAdMi(ad) && m != nil {
		gizliEkle(m, deger)
	}
	return &yurut.StringVal{Val: deger}, nil
}

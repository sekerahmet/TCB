package process

import (
	"fmt"
	"os"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// ortamDegiskeni ortam_değişkeni(ad) yerleşiğidir: değişkenin değeri (Metin) ya da
// tanımsızsa yok. Bütün ortamı listeleyen ya da ortamı değiştiren bir ad bilerek yoktur
// (yanlışlıkla döküm olmasın; TR alt süreç açmaz). Adı gizli bir sözcük içeren
// değişkenin değeri gizli değer süzgecine kaydedilir.
func ortamDegiskeni(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.G800, "1", len(args), "ad")
	}
	s, ok := args[0].(*interp.StringVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.G800, 1, args[0], codes.A202)
	}
	ad := s.Val
	if ad == "" || strings.ContainsAny(ad, "=\x00") {
		return nil, fmt.Errorf("%s: %q geçerli bir ortam değişkeni adı değil (boş olamaz, '=' içeremez)", interp.CallPrefix(codes.G800), ad)
	}
	deger, bulundu := os.LookupEnv(ad)
	if !bulundu {
		return interp.NULL, nil
	}
	if GizliAdMi(ad) && m != nil {
		gizliEkle(m, deger)
	}
	return &interp.StringVal{Val: deger}, nil
}

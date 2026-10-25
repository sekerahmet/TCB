package process

import (
	"os"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// envVar ortam_değişkeni(ad) yerleşiğidir: değişkenin değeri (Metin) ya da
// tanımsızsa yok. Bütün ortamı listeleyen ya da ortamı değiştiren bir ad bilerek yoktur
// (yanlışlıkla döküm olmasın; TR alt süreç açmaz). Adı gizli bir sözcük içeren
// değişkenin değeri gizli değer süzgecine kaydedilir.
func envVar(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.FnEnv, 1, len(args), messages.Text(messages.H2330))
	}
	s, ok := args[0].(*interp.StringVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.FnEnv, 1, args[0], codes.TypeString)
	}
	name := s.Val
	if name == "" || strings.ContainsAny(name, "=\x00") {
		return nil, messages.New(messages.H7231, interp.CallPrefix(codes.FnEnv), name)
	}
	value, found := os.LookupEnv(name)
	if !found {
		return interp.Null, nil
	}
	if IsSecretName(name) && m != nil {
		addSecret(m, value)
	}
	return &interp.StringVal{Val: value}, nil
}

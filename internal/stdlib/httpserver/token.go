package httpserver

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// tokenBytes TR_secret_token'ın rastgele bayt sayısıdır (28.2a).
const tokenBytes = 32

// secretToken TR_secret_token()'dır (28.2a): işletim sisteminin güvenli rastgele kaynağından 32 bayt,
// URL'de güvenli taban 64 ile (dolgusuz, 43 karakter, [0-9A-Za-z_-]). Dilin tohumlu üreteci
// kullanılmaz. Değer gizlidir (19.4): yorumlayıcının süzgecine verilir.
func secretToken(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.FnSecretToken, 0, len(args))
	}
	b := make([]byte, tokenBytes)
	_, _ = rand.Read(b) // crypto/rand.Read hata vermez (Go 1.24)
	t := base64.RawURLEncoding.EncodeToString(b)
	m.AddSecret(t)
	return &interp.StringVal{Val: t}, nil
}

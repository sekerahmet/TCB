package interp

import (
	"strings"
	"testing"
	"time"
)

func TestUnknownNameErrorHasColumnAndSuggestion(t *testing.T) {
	for _, tc := range []struct{ code, want string }{
		{"değişken toplam = 1\nyazdır(toplm + 1)", "Satır 2, Sütun 8: 'toplm' adında bir değişken ya da fonksiyon yok. Bunu mu demek istediniz: 'toplam'?"},
		{"değişken değişken_adı = 1\ndegisken_adi = 2", "Bunu mu demek istediniz: 'değişken_adı'?"},
		{"değişken sayaç = 1\nsayac += 1", "Satır 2, Sütun 1: 'sayac' adında bir değişken yok. Bunu mu demek istediniz: 'sayaç'?"},
		{"yazdırr(1)", "Bunu mu demek istediniz: 'yazdır'?"},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, tc.code)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q içeren hata bekleniyordu, alınan: %v", tc.want, err)
		}
	}
}

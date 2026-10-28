package interp

import (
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestUnknownNameErrorHasColumnAndSuggestion(t *testing.T) {
	for _, tc := range []struct {
		code      string
		line, col int    // 0: konum denetlenmez
		want      string // öneri ipucunun metni (biçimi)
	}{
		{"TR_var adet = 1\nyazdır(adt + 1)", 2, 8, "Bunu mu demek istediniz: 'adet'?"},
		{"TR_var değişken_adı = 1\ndegisken_adi = 2", 0, 0, "Bunu mu demek istediniz: 'değişken_adı'?"},
		{"TR_var sayaç = 1\nsayac += 1", 2, 1, "Bunu mu demek istediniz: 'sayaç'?"},
		{"yazdırr(1)", 0, 0, "Bunu mu demek istediniz: 'TR_print'?"},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, tc.code)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q içeren hata bekleniyordu, alınan: %v", tc.want, err)
			continue
		}
		if line, col, ok := messages.Pos(err); tc.line != 0 && (!ok || line != tc.line || col != tc.col) {
			t.Errorf("konum %d:%d bekleniyordu, alınan %d:%d (%v)", tc.line, tc.col, line, col, ok)
		}
	}
}

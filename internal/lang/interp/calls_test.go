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
		{"TR_var adet = 1\nTR_print(adt + 1)", 2, len("TR_print(") + 1, "Bunu mu demek istediniz: 'adet'?"},
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

// Kural (6.7, geçiş): sondaki blok argümansız da çağrılabilir; o zaman TR_it değer almaz ve okunursa
// çalışma hatasıdır (sessiz yokluk değil). Argümanla çağrılan blok TR_it'i okur.
func TestTrailingBlockCalledWithoutArgument(t *testing.T) {
	if _, val, err := runTRWithin(t, 5*time.Second, "TR_func yap(f) { TR_return f() }\nyap { 5 }"); err != nil || val.Inspect() != "5" {
		t.Errorf("argümansız çağrı: %v %v", val, err)
	}
	if _, _, err := runTRWithin(t, 5*time.Second, "TR_func yap(f) { TR_return f() }\nyap { TR_it }"); !codeAt(err, messages.H2707, 2) {
		t.Errorf("H2707 bekleniyordu: %v", err)
	}
	if _, val, err := runTRWithin(t, 5*time.Second, "[1, 2].TR_map { TR_it * 2 }"); err != nil || val.Inspect() != "[2, 4]" {
		t.Errorf("argümanla çağrı: %v %v", val, err)
	}
}

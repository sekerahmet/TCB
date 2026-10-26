package ml

import (
	"bytes"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Hesap çekirdeğinin kodlu paniği (ör. GPU belleği yetmedi, H4014) her ml yerleşiğinde
// yakalanabilir bir hataya dönüşür ve kodunu korur. Eskiden sıfırlar/birler/rastgele_normal
// tensörü safe dışında kurduğu için GPU'nun bellek paniği yorumlayıcının en üst kurtarmasına
// çıkıyor, iç hata (H9000) sayılıyor ve dene/yakala onu göremiyordu.
func TestWrappedBuiltinPanicIsCatchable(t *testing.T) {
	runWith := func(code string) (string, error) {
		m := interp.New()
		wrap(m, codes.FnZeros, newBuiltin(func(*interp.VM, []interp.Value, map[string]interp.Value) (interp.Value, error) {
			panic(messages.New(messages.H4014, "1 MB", "0 bayt", "1 GB", "CUDA_ERROR_OUT_OF_MEMORY"))
		}))
		var out bytes.Buffer
		m.Out = &out
		p := parser.New(source.TextLexer(code))
		_, err := m.Run(p.ParseProgram())
		return out.String(), err
	}
	_, err := runWith(`sıfırlar([2])`)
	if !messages.Has(err, messages.H4014) || messages.Has(err, messages.H9000) {
		t.Fatalf("kodlu bellek hatası (H4014) bekleniyordu, iç hata değil: %v", err)
	}
	out, err := runWith(`dene { sıfırlar([2]) } yakala h { yazdır("yakalandı") }`)
	if err != nil || out != "yakalandı\n" {
		t.Fatalf("dene/yakala hatayı yakalamalı: %q, %v", out, err)
	}
}

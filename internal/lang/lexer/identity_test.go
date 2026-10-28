package lexer

import "testing"

// Kural (2.3a): yapı gövdesinde bir işleçle biten satırdan sonraki satır değerin devamıdır; oradaki
// kimlik öbür yerdedir, üye değil. Bit işleçleri (4.12) de bu işleçlerdendir: TR_parameters hem
// fonksiyon hem üye olduğu için yanlış yer yanlış kodu verirdi.
func TestALineAfterABitOperatorContinuesTheValue(t *testing.T) {
	for _, op := range []string{"&", "|", "^", "<<", ">>"} {
		src := "TR_struct A {\n    x: TR_Int = 1 " + op + "\n        TR_parameters(m)\n}\n"
		found := false
		for _, tok := range allOf(NewBody(src, true)) {
			if tok.Literal == "TR_fn_parameters" {
				found = true
			}
			if tok.Literal == "TR_mem_parameters" {
				t.Errorf("%q sonrasındaki satırda TR_parameters üye okundu, fonksiyon olmalıydı:\n%s", op, src)
			}
		}
		if !found {
			t.Errorf("%q sonrasındaki satırda TR_fn_parameters bulunamadı:\n%s", op, src)
		}
	}
}

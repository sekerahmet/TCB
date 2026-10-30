package lexer

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

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

// Kural (3.5, 7.1): satır sonundaki isteğe bağlı alan tipinin '?'si işleç değildir; sonraki satırın
// adı yine yapının üyesidir (TR_mem_parameters), devam eden bir değerin adı değil.
func TestALineAfterAnOptionalFieldTypeIsAMember(t *testing.T) {
	src := "TR_struct A {\n    x: TR_Int?\n    TR_parameters: TR_Int?\n}\n"
	found := false
	for _, tok := range allOf(NewBody(src, true)) {
		if tok.Literal == "TR_mem_parameters" {
			found = true
		}
		if tok.Literal == "TR_fn_parameters" {
			t.Errorf("'?' ile biten satırdan sonra TR_parameters fonksiyon okundu, üye olmalıydı:\n%s", src)
		}
	}
	if !found {
		t.Errorf("'?' ile biten satırdan sonra TR_mem_parameters bulunamadı:\n%s", src)
	}
}

// Kural (3.12, 17.6): eski çevirme fonksiyonları tip çağrısı oldu; TR_ ile yazılmış eski ya da başka
// dilin adı, aynı yazımlı bir üye kodu ya da yazımca yakın bir kimlik yerine tipi önerir; aralık için
// ileti 0..n'yi ve TR_List(0..n)'yi söyler. Öneri kopyalanınca çalışan yazımdır.
func TestAnOldConversionNameSuggestsTheType(t *testing.T) {
	for lit, want := range map[string]string{
		"TR_str": "TR_String", "TR_int": "TR_Int", "TR_type": "TR_Type", "TR_checkpoint": "TR_Checkpoint",
		"TR_tensor": "TR_Tensor", "TR_embedding": "TR_embedding_lookup", "TR_None": "TR_none", "TR_def": "TR_func",
	} {
		err := identityError(codes.PlaceOther, lit)
		if !messages.Has(err, messages.H0026) || !strings.Contains(err.Error(), "'"+want+"'") {
			t.Errorf("%s: %v; want H0026 suggesting %s", lit, err, want)
		}
	}
	if err := identityError(codes.PlaceOther, "TR_range"); !messages.Has(err, messages.H0242) {
		t.Errorf("TR_range: %v; want H0242", err)
	}
	// Tipe giden tanıdık ad aynı yazımlı üyenin önüne geçmez: map TR_Dict'in tanıdık adıdır, TR_map üyedir.
	if err := identityError(codes.PlaceOther, "TR_map"); !messages.Has(err, messages.H0027) {
		t.Errorf("TR_map: %v; want H0027 (x.TR_map)", err)
	}
	// Yazım hatasına en yakın kimlik önerilir.
	if err := identityError(codes.PlaceOther, "TR_prnt"); !strings.Contains(err.Error(), "'TR_print'") {
		t.Errorf("TR_prnt: %v", err)
	}
}

package parser

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Kural (13.9): yerleşiklerin imzasındaki tip metni programdaki yazımla, dilin kendi tip
// ayrıştırıcısıyla okunur; tipten sonra kalan metin ve tip olmayan metin hatadır.
func TestParseTypeText(t *testing.T) {
	cases := []struct{ text, name string }{
		{"TR_Int", "TR_type_Int"}, // ayrıştırıcı kimliği kodu olarak taşır (programdaki gibi)
		{"TR_List[T]", "TR_type_List"},
		{"TR_Dict[TR_String, T]", "TR_type_Dict"},
		{"T?", "T"},
	}
	for _, c := range cases {
		ty, err := ParseTypeText(c.text)
		if err != nil || ty.Name != c.name {
			t.Errorf("%q: %+v %v", c.text, ty, err)
		}
	}
	if ty, err := ParseTypeText("(T) -> U"); err != nil || ty.Return == nil || len(ty.Params) != 1 {
		t.Errorf("fonksiyon tipi okunmalı: %+v %v", ty, err)
	}
	if ty, err := ParseTypeText("T?"); err != nil || !ty.Optional {
		t.Errorf("T? isteğe bağlı olmalı: %+v %v", ty, err)
	}
	for _, bad := range []string{"", "TR_Int TR_Float", "TR_List[", "()", "TR_Int|TR_Float"} {
		if _, err := ParseTypeText(bad); err == nil {
			t.Errorf("%q hata vermeli", bad)
		}
	}
}

// Kural (13.9): tanım dosyasındaki her tip metni (konumlu parametre, adlı argüman, dönüş) dilin tip
// ayrıştırıcısıyla okunur ve asıl adı tanımın tip koduyla aynıdır: codes paketi tipin yapısını okumaz.
func TestDefinitionTypesParse(t *testing.T) {
	for _, k := range codes.DefinitionCodes() {
		def, _ := codes.DefinitionOf(k)
		sigs := []codes.Signature{def.Signature}
		for _, m := range def.Members {
			sigs = append(sigs, m.Signature)
		}
		for _, sig := range sigs {
			exprs := append([]codes.TypeExpr{sig.Return}, sig.Params...)
			for _, a := range sig.Named {
				exprs = append(exprs, a.Type)
			}
			for _, e := range exprs {
				for i, text := range e.Text {
					ty, err := ParseTypeText(text)
					if err != nil {
						t.Errorf("%s: %q okunmadı: %v", k, text, err)
						continue
					}
					head := ty.Name
					switch {
					case ty.Return != nil:
						head = codes.TypeFunction.String()
					case codes.TypeParameter(ty.Name):
						head = codes.TypeAny.String()
					}
					if head != e.Types[i].String() {
						t.Errorf("%s: %q'nın asıl adı %s, tanımın kodu %s", k, text, ty.Name, e.Types[i])
					}
				}
			}
		}
	}
}

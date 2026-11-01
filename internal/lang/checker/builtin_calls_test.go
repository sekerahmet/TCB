package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (13.11): tipi bilinen alıcıda (Metin, Liste, Sözlük, Aralık) yerleşik metodun konumlu
// argüman sayısı tanım dosyasının imzasına uymazsa denetim hatasıdır; kod ve ileti çalışma
// anındakidir (H2011): ad ve kullanım kimlikle, görünüm adı ve <nil> olmadan.
func TestMemberArgCount(t *testing.T) {
	cases := []struct{ source, want string }{
		{"TR_var l = [3, 1, 2]\nTR_print(l.TR_reverse(1))", "`.TR_reverse()`"},
		{`TR_print("abc".TR_replace("a"))`, "`.TR_replace(eski, yeni)`"},
		{`TR_print({"a": 1}.TR_get())`, "`.TR_get(anahtar)`"},
		{"TR_print((0..3).TR_reverse(1))", "`.TR_reverse()`"},
		{"[1].TR_append()", "`.TR_append(eleman, ...)`"},
	}
	for _, tc := range cases {
		_, errs := checkErrs(t, tc.source)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H2011) {
			t.Errorf("tek H2011 beklenirdi, gelen %v\nkaynak:\n%s", errs, tc.source)
			continue
		}
		text := messages.ErrorIn(errs[0], "tr")
		if !strings.Contains(text, tc.want) || strings.Contains(text, "<nil>") || strings.Contains(text, "liste.") {
			t.Errorf("ileti %s kullanımını kimlikle vermeli: %s", tc.want, text)
		}
	}
	// Doğru sayı, tipi bilinmeyen alıcı (unknownValue), isteğe bağlı alıcı
	// ve henüz sayısı sınanmayan alıcı tipi (tensör) hatasızdır.
	for _, source := range []string{
		"TR_var l = [3, 1, 2]\nTR_print(l.TR_sort(), l.TR_join(), [\"a\"].TR_join(\",\"))",
		`TR_print(` + unknownValue + `.TR_sort(1, 2))`,
		"TR_var l: TR_List[TR_Int]? = TR_none\nTR_print(l?.TR_sort(1, 2))",
	} {
		noErrors(t, source)
	}
}

// Kural (13.10, 13.11): tipi bilinen alıcının tipinde olmayan yerleşik metot denetim hatasıdır;
// kod çalışma anındakidir (H2474), ipucu metodun hangi tiplerde olduğunu söyler. Metot tek
// argümanın tipinde varsa ipucu yer değiştirmeyi önerir (" ".TR_join(l) → l.TR_join(" ")).
func TestMissingMember(t *testing.T) {
	cases := []struct{ source, want string }{
		{"TR_var l = [\"a\", \"b\"]\nTR_print(\" \".TR_join(l))", "`l.TR_join(\" \")`"},
		{"TR_var l = [\"a\"]\nTR_print(l.TR_strip())", "`.TR_strip` TR_String tipinin metodudur"},
		{"TR_print((0..3).TR_sort())", "`.TR_sort` TR_List tipinin metodudur"},
		{"TR_var t: TR_Tensor = TR_Tensor([1.0])\nTR_print(t.TR_sort())", "`.TR_sort` TR_List tipinin metodudur"},
		{"TR_var l = [1]\nTR_print(\"a\".TR_join(1))", "`.TR_join` TR_List tipinin metodudur"}, // argüman liste değil: yer değiştirme yok
	}
	for _, tc := range cases {
		errs := checkFileErrs(t, tc.source)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H2474) {
			t.Errorf("tek H2474 beklenirdi, gelen %v\nkaynak:\n%s", errs, tc.source)
			continue
		}
		text := messages.ErrorIn(errs[0], "tr")
		if !strings.Contains(text, tc.want) || strings.Contains(tc.want, "metodudur") && strings.Contains(text, "yeri değişmiş") {
			t.Errorf("ileti %s içermeli (yer değiştirme yalnız argümanın tipinde): %s", tc.want, text)
		}
	}
	// Metot alıcının tipinde, alıcının tipi bilinmiyor (TR_JSON) ya da isteğe bağlı: hatasız.
	for _, source := range []string{
		"TR_var l = [\"a\"]\nTR_print(l.TR_join(\" \"), \" a \".TR_strip(), {\"a\": 1}.TR_keys())",
		`TR_print(` + unknownValue + `.TR_strip())`,
		"TR_var m: TR_String? = TR_none\nTR_print(m?.TR_sort())",
	} {
		noErrors(t, source)
	}
}

// Kural (13.11): tipi bilinen alıcının özelliği çağrılmaz; değeri fonksiyon olmayan bir tipse
// çalışma anındaki hatanın kendisi denetimde verilir (H2421). Tipi bilinmeyen alıcı, değeri
// fonksiyon olabilen özellik ve parantezsiz özellik hatasızdır.
func TestPropertyCall(t *testing.T) {
	for _, source := range []string{
		"TR_print([1].TR_length(1))",
		"TR_var l = [1, 2]\nTR_print(l.TR_length())",
		`TR_print({"a": 1}.TR_length())`,
	} {
		_, errs := checkErrs(t, source)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H2421) {
			t.Errorf("tek H2421 beklenirdi, gelen %v\nkaynak:\n%s", errs, source)
		}
	}
	for _, source := range []string{
		"TR_print([1].TR_length, \"ab\".TR_length)",
		`TR_print(` + unknownValue + `.TR_length())`,
		"TR_func f() -> TR_Int {\n    TR_return 1\n}\nTR_print([f].TR_first())",
	} {
		noErrors(t, source)
	}
}

// Kural (13.11): tipi bilinen çekirdek alıcıda metodun tek tipli parametresine uymayan, tipi bilinen
// argüman çalışma anının koduyla denetim hatasıdır (H2423); ileti metodu kimlikle yazar. Tipi bilinmeyen
// argüman, TR_JSON, motorun denetlemediği parametre (ParamsNotVerified) ve doğru tip hatasızdır.
func TestMemberArgType(t *testing.T) {
	for _, source := range []string{
		`TR_print("a,b".TR_split(1))`,
		`TR_print([1, 2].TR_join(1))`,
		`TR_print("ab".TR_starts_with(TR_none))`,
		`TR_print([1].TR_filter(3))`,
	} {
		_, errs := checkErrs(t, source)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H2423) {
			t.Errorf("tek H2423 beklenirdi, gelen %v\nkaynak:\n%s", errs, source)
			continue
		}
		if text := messages.ErrorIn(errs[0], "en"); !strings.Contains(text, ": .TR_") || strings.Contains(text, "string.") || strings.Contains(text, "list.") {
			t.Errorf("ileti metodu kimlikle yazmalı: %s", text)
		}
	}
	for _, source := range []string{
		`TR_print("a,b".TR_split(","), [1].TR_filter((x) => x > 0), [1].TR_reduce(0, (a, x) => a + x))`,
		`TR_print("a,b".TR_split(` + unknownValue + `))`,
		`TR_print([1].TR_reduce((a, x) => a + x))`,
	} {
		noErrors(t, source)
	}
}

// Kural (3.6): liste tek tiplidir; öğe tipi bilinen listeye .TR_append ile başka tipte değer eklemek
// denetim hatasıdır (H1123; çalışma anı öğe tipini taşımaz). Tam sayı yazımı ondalık listeye eklenir
// (3.13); öğe tipi bilinmeyen listeye her değer eklenir.
func TestAppendElementType(t *testing.T) {
	singleError(t, "TR_var l = [1]\nl.TR_append(\"a\")", messages.H1123, 2, "TR_List[TR_Int]", "TR_String")
	singleError(t, "TR_var l: TR_List[TR_String] = []\nl.TR_append(\"a\", 2)", messages.H1123, 2, "2.", "TR_Int")
	noErrors(t, "TR_var f: TR_List[TR_Float] = []\nf.TR_append(1, 2.5)\nTR_var l = [[1]]\nl.TR_append([2])\nTR_var j = "+unknownValue+"\nj.TR_append(\"a\")\nTR_print(f, l, j)")
}

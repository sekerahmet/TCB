package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (1.2, 13.9): yerleşik çağrısının tipi tanım dosyasındaki tek tipli dönüşüdür (fonksiyon,
// metot, tip çağrısı); tipi bilinen çekirdek alıcıda metodun dönüşü alıcıdaki imzasıdır, öğe tipi
// alıcıdan kurulur (memberValueType). Birleşim dönüş, motorla doğrulanmamış dönüş (ReturnsNotVerified)
// ve denetleyicinin tanımadığı tip TR_Any kalır.
func TestBuiltinReturnTypes(t *testing.T) {
	expectTypes(t, map[string]string{
		"TR_var v = [3, 1].TR_sort()":        listType(typeInt),
		"TR_var v = \"a\".TR_upper()":        typeString,
		"TR_var v = \"a,b\".TR_split(\",\")": listType(typeString),
		"TR_var v = {\"a\": 1}.TR_keys()":    listType(typeString),
		"TR_var v = TR_Tensor([1.0])":        typeTensor,
		"TR_var v = TR_time_ms()":            typeInt,
		"TR_var v = TR_monotonic_ns()":       typeInt,
		// 13.8: dönüşte yazılı öğe tipi tipe girer
		"TR_var v = TR_parse_csv(\"1,2\")":       listType(listType(typeString)),
		"TR_var v = TR_read_csv(\"a.csv\")":      listType(listType(typeString)),
		"TR_var v = TR_parse_csv(\"1,2\")[0]":    listType(typeString),
		"TR_var v = TR_parse_csv(\"1,2\")[0][0]": typeString,
		"TR_var v = TR_floor(1.5)":               typeAny,  // birleşim: TR_Int|TR_Float|TR_Tensor
		"TR_var v = TR_json_parse(\"1\")":        typeJSON, // 3.7a; motorla doğrulaması TestJSONReturnsAreJSONValues (stdlib)
		"TR_var v = TR_Path(\"a\")":              typeAny,  // tanınmayan tip
		"TR_var v = [\"a\"].TR_join(\",\")":      typeString,
		"TR_var v = (0..3).TR_reverse()":         listType(typeInt), // alıcıdaki imza: aralıkta TR_List[TR_Int]
		"TR_var v = [1].TR_contains(1)":          typeBool,
		// 13.1: TR_max ve TR_min'in tipi işlecin tipidir; karışıkta TR_Float
		"TR_var v = TR_max(3, 7)":                        typeInt,
		"TR_var v = TR_max(2, 1.5)":                      typeFloat,
		"TR_var v = TR_min([3, 1])":                      typeInt,
		"TR_var v = TR_min([2.0, 1.5])":                  typeFloat,
		"TR_var v = TR_max(TR_Tensor([1.0]), TR_dim: 0)": typeAny,
		"TR_var v = TR_max(\"a\", \"b\")":                typeAny,
		// 13.1: anahtarlı biçim listenin öğesini verir
		"TR_var v = TR_max([1, 2], (x) => -x)":           typeInt,
		"TR_var v = TR_min([\"a\"], (s) => s.TR_length)": typeString,
		"TR_var v = TR_max([[1]], (l) => l.TR_length)":   listType(typeInt),
		// 13.1: TR_input TR_String? verir (girdi sonunda TR_none)
		"TR_var v = TR_input()":                      typeString + "?",
		"TR_var v = TR_max(" + unknownValue + ", 1)": typeAny,
		// 13.1: TR_round(x) TR_Int, TR_round(x, basamak) girdinin tipi
		"TR_var v = TR_round(3.7)":                     typeInt,
		"TR_var v = TR_round(5, 2)":                    typeInt,
		"TR_var v = TR_round(3.7, 0)":                  typeFloat,
		"TR_var v = TR_round(1234.0, -2)":              typeFloat,
		"TR_var v = TR_round(TR_Tensor([1.5]))":        typeAny,
		"TR_var v = TR_round(" + unknownValue + ", 1)": typeAny,
	})
	_, errs := checkErrs(t, "TR_var m = TR_max(2, 3)\nm = 1.5")
	if len(errs) != 1 || !messages.Has(errs[0], messages.H1005) {
		t.Errorf("TR_max(2, 3) TR_Int'tir, ondalık atanamaz (H1005) olmalı: %v", errs)
	}
	_, errs = checkErrs(t, "TR_var s = \"a\".TR_upper()\ns = 1")
	if len(errs) != 1 || !messages.Has(errs[0], messages.H1005) {
		t.Errorf("metin değişkenine tam sayı H1005 olmalı: %v", errs)
	}
}

// Kural (yanlış alarm vermez): birleşik dönüşlü yerleşiğin (TR_abs: TR_Int, TR_Float ya da TR_Tensor)
// çağrısının tipi bilinmez; testlerin "tipi bilinmeyen değer"i (unknownValue) budur.
func TestUnknownValueHasNoType(t *testing.T) {
	expectTypes(t, map[string]string{"TR_var v = " + unknownValue: typeAny})
}

// Kural (D1 Gemma 3, Teslim D): yerleşik tipte olmayan bir yapının metodu (kitaplık tipi; aktarılan
// modülün yapısı) yerleşik üye tablosundan okunmaz: aynı adlı yerleşik üyenin imzası ona geçmez, dönüşü
// TR_Any'dir (yapının imzası denetleyiciye gelene kadar). Kitaplık tipinin kurucusu o tipi verir; açık
// tipli ve tipsiz alıcı aynıdır. Yerleşik tipteki alıcı değişmez: TR_MaskStream'in TR_finish'i TR_String.
func TestLibraryStructMethodIsNotTheBuiltinMember(t *testing.T) {
	for _, alıcı := range []string{"TR_var d: TR_ChatTokens = TR_ChatTokens(TR_tokenizer: p)", "TR_var d = TR_ChatTokens(TR_tokenizer: p)"} {
		noErrors(t, "TR_import \"tr/chat\"\nTR_func f(p: TR_Tokenizer) -> TR_List[TR_Int] {\n    "+alıcı+"\n    TR_return d.TR_finish()\n}\n")
	}
	singleError(t, "TR_import \"tr/chat\"\nTR_var s: TR_String = TR_ChatTokens(TR_tokenizer: TR_none)\n", messages.H1001, 2, `"TR_String"`, `"TR_ChatTokens"`)
	singleError(t, "TR_var m = TR_mask_stream()\nTR_var x: TR_Int = m.TR_finish()\n", messages.H1001, 2, `"TR_Int"`, `"TR_String"`)
}

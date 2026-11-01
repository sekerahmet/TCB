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
		"TR_var v = TR_floor(1.5)":           typeAny, // birleşim: TR_Int|TR_Float|TR_Tensor
		"TR_var v = TR_json_parse(\"1\")":    typeAny, // dönüşü motorla uyuşmuyor (13.8)
		"TR_var v = TR_Path(\"a\")":          typeAny, // tanınmayan tip
		"TR_var v = [\"a\"].TR_join(\",\")":  typeString,
		"TR_var v = (0..3).TR_reverse()":     listType(typeInt), // alıcıdaki imza: aralıkta TR_List[TR_Int]
		"TR_var v = [1].TR_contains(1)":      typeBool,
	})
	_, errs := checkErrs(t, "TR_var s = \"a\".TR_upper()\ns = 1")
	if len(errs) != 1 || !messages.Has(errs[0], messages.H1005) {
		t.Errorf("metin değişkenine tam sayı H1005 olmalı: %v", errs)
	}
}

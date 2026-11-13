package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (1.2, 13.9): tipi bilinen çekirdek alıcıda yerleşik özelliğin değeri ve metodun sonucu
// tanımdaki dönüş yazımıdır; tip parametreleri alıcının öğe tipleridir (TR_List[T]'de T, TR_Dict[K, V]'de
// K ve V). Öğe tipi bilinmeyen alıcıda T TR_Any'dir: TR_List[T] öğesiz TR_List, T? TR_Any. Çağrısız
// metot bir fonksiyon değeridir (6.9), özelliğin tipini almaz.
func TestMemberValueTypes(t *testing.T) {
	expectTypes(t, map[string]string{
		"TR_var v = [1, 2].TR_length":           typeInt,
		"TR_var v = \"çay\".TR_length":          typeInt,
		"TR_var v = [1, 2].TR_first":            optionalType(typeInt),
		"TR_var v = [[1]].TR_last":              optionalType(listType(typeInt)),
		"TR_var v = {\"a\": 1.5}.TR_get(\"a\")": optionalType(typeFloat),
		"TR_var v = {\"a\": 1}.TR_values()":     listType(typeInt),
		"TR_var l: TR_List[TR_String] = []\nTR_var v = l.TR_filter((s) => s != \"\")": listType(typeString),
		"TR_var v = [1, 2].TR_get(0)":              optionalType(typeInt),
		"TR_var v = [\"b\", \"a\"].TR_unique()":    listType(typeString),
		"TR_var v = " + unknownValue + ".TR_first": typeAny, // alıcının tipi bilinmiyor
		"TR_var v = [3, 1].TR_sort":                typeAny, // çağrısız metot: fonksiyon değeri
	})
}

// Kural (1.2, 3.13): üyenin tipi bilinince tipli değişkene uyar. İsteğe bağlı değerin T beklenen
// yere verilmesi 3.5'in işidir (4b), burada sınanmaz.
func TestMemberValueTypeMismatch(t *testing.T) {
	singleError(t, "TR_var n: TR_String = [1, 2].TR_length", messages.H1001, 1, `"TR_String"`, `"TR_Int"`)
	singleError(t, "TR_var x: TR_String = [1, 2].TR_first ?? 0", messages.H1001, 1, `"TR_String"`, `"TR_Int"`)
	noErrors(t, "TR_var x: TR_Int = [1, 2].TR_first ?? 0\nTR_var k: TR_List[TR_String] = {\"a\": 1}.TR_keys()\nTR_print(x, k)")
}

package ml

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kendini içeren liste tensöre çevrilirken sonsuz şekil üretmez (eskiden bellek bitene dek
// eksen ekleniyordu); açık bir hata verir.
func TestTensorFromCyclicList(t *testing.T) {
	_, err := run(t, "TR_var a = []\na.TR_append(a)\nTR_Tensor(a)")
	if err == nil || !messages.Has(err, messages.H6018) {
		t.Fatalf("döngüsel liste hatası beklenirdi: %v", err)
	}
	deep := strings.Repeat("[", 22) + "1.0" + strings.Repeat("]", 22)
	if out := mustRun(t, "TR_print(TR_Tensor("+deep+").TR_shape.TR_length)"); strings.TrimSpace(out) != "22" {
		t.Fatalf("derin ama düzgün liste: %q", out)
	}
}

// Kural (15.2): öğe cinsi veriden çıkar. Tam sayı verisi (aralık da) float32'den geçmeden
// "int64" tensör kurar (2²⁴'ün ve 2⁵³'ün üstü bozulmaz); en az bir ondalık sayılı liste ondalıktır,
// yuvarlar; mantık değeri "bool"dur ve sayıyla karışmaz; boş liste ondalıktır. TR_dtype: cinsin
// içinde seçer, başka cinsin hassasiyeti hatadır; tam sayı hassasiyetine yazılan değer sığmalıdır.
func TestTensorKindFromData(t *testing.T) {
	out := mustRun(t, `TR_print(TR_Tensor([16777217, -9223372036854775807, 9007199254740993]).TR_tolist, TR_Tensor(16777217).TR_dtype)
TR_print(TR_Tensor([16777217.0]).TR_tolist, TR_Tensor([2.5, 16777217]).TR_tolist, TR_Tensor([]).TR_dtype)
TR_print(TR_Tensor(0..3).TR_tolist, TR_Tensor(1..=2).TR_dtype, TR_Tensor([TR_true, TR_false]).TR_dtype)
TR_print(TR_Tensor([1, 2], TR_dtype: "uint8").TR_dtype, TR_Tensor([1.5], TR_dtype: "bf16").TR_dtype)`)
	want := "[16777217, -9223372036854775807, 9007199254740993] int64\n" +
		"[16777216.0] [2.5, 16777216.0] float32\n" +
		"[0, 1, 2] int64 bool\n" +
		"uint8 bf16\n"
	if out != want {
		t.Fatalf("kurulan tensörler %q, beklenen %q", out, want)
	}
	expectCodes(t, []codeCase{
		{"TR_Tensor([1, TR_true])", messages.H6975},
		{"TR_Tensor([1.5], TR_dtype: \"int64\")", messages.H6976},
		{"TR_zeros([3], TR_dtype: \"int64\")", messages.H6976},
		{"TR_Tensor([TR_true], TR_dtype: \"float32\")", messages.H6976},
		{"TR_Tensor([300], TR_dtype: \"uint8\")", messages.H6964},
		{"TR_Tensor([1, 2], TR_requires_grad: TR_true)", messages.H6973},
		{"TR_Tensor(\"a\")", messages.H6032},
	})
	_, err := run(t, "TR_Tensor([[1, 2], [3, TR_false]])")
	if e, ok := messages.As(err); !ok || fmt.Sprint(e.Args[1]) != "[1][1]" {
		t.Fatalf("ilk karışan öğenin konumu: %v", err)
	}
}

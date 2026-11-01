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

// Kural (15.1'in ara kuralı): bütün öğeleri tam sayı olan listede float32'de tam yazılamayan değer
// (|n| > 2²⁴) tensör kurmayı çalışma hatası yapar (H6954); ileti ilk öğenin konumunu ve değerini
// söyler. 2²⁴, 2²⁴ + 2 ve tam yazılan büyük sayılar kurulur; karışık liste ondalıktır, yuvarlar.
func TestIntListInexactInFloat32(t *testing.T) {
	expectCodes(t, []codeCase{
		{"TR_Tensor([16777217])", messages.H6954},
		{"TR_Tensor([1, -16777217])", messages.H6954},
		{"TR_Tensor([[1, 2], [3, 33554433]])", messages.H6954},
		{"TR_Tensor([9223372036854775807])", messages.H6954},
		{"TR_Tensor(16777217)", messages.H6954},
		{"TR_Tensor(-16777217)", messages.H6954},
		{"TR_Tensor(9223372036854775807)", messages.H6954},
	})
	_, err0 := run(t, "TR_Tensor(16777219)")
	if e, ok := messages.As(err0); !ok || len(e.Args) != 4 || fmt.Sprint(e.Args[1:]) != "[ 16777219 16777220]" {
		t.Fatalf("tek tam sayının iletisi: %v", err0)
	}
	if got := mustRun(t, `TR_print(TR_Tensor(16777216).TR_tolist, TR_Tensor(-16777216).TR_tolist, TR_Tensor(16777218).TR_tolist, TR_Tensor(16777217.0).TR_tolist)`); got != "16777216.0 -16777216.0 16777218.0 16777216.0\n" {
		t.Fatalf("tek tam sayının sınırları %q", got)
	}
	_, err := run(t, "TR_Tensor([[1, 2], [16777219, 33554433]])")
	if e, ok := messages.As(err); !ok || len(e.Args) != 4 || fmt.Sprint(e.Args[1:]) != "[[1][0] 16777219 16777220]" {
		t.Fatalf("ilk yanlış öğe ve konumu: %v", err)
	}
	out := mustRun(t, `TR_print(TR_Tensor([16777216, 16777218, -16777216, 4611686018427387904]).TR_tolist)
TR_print(TR_Tensor([16777217.0]).TR_tolist, TR_Tensor([2.5, 16777217]).TR_tolist, TR_Tensor([]).TR_shape)`)
	if want := "[16777216.0, 16777218.0, -16777216.0, 4.611686018427388e+18]\n[16777216.0] [2.5, 16777216.0] [0]\n"; out != want {
		t.Fatalf("kurulan tensörler %q, beklenen %q", out, want)
	}
	for _, c := range []struct {
		n    int64
		want bool
	}{{0, true}, {1 << 24, true}, {1<<24 + 1, false}, {1<<24 + 2, true}, {-(1<<24 + 1), false}, {1 << 62, true},
		{1<<63 - 1, false}, {-1 << 63, true}, {-1<<63 + 1, false}} {
		if got := exactInFloat32(c.n); got != c.want {
			t.Errorf("exactInFloat32(%d) = %v", c.n, got)
		}
	}
}

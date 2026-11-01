package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (15.6d): bileşik atama tensörü yerinde değiştirir; tensörü paylaşan her ad değişikliği
// görür (alan, liste öğesi ve parametre de). İndeksli hedef görünüm üstünde yerinde değişir.
func TestCompoundAssignInPlace(t *testing.T) {
	out := mustRun(t, `TR_var p = TR_Tensor([1.0, 2.0])
TR_var q = p
p -= 0.5
TR_var l = [p]
p *= 2.0
TR_func artır(x: TR_Tensor) { x += 10.0 }
artır(p)
TR_print(q.TR_tolist, l[0].TR_tolist, p.TR_tolist)
TR_var m = TR_Tensor([[1.0, 2.0], [3.0, 4.0]])
TR_var satır = m[1]
m[1] += TR_Tensor([10.0, 20.0])
m[0..1] *= 3.0
satır /= 2.0
satır **= 2.0
TR_print(m.TR_tolist)`)
	want := "[11.0, 13.0] [11.0, 13.0] [11.0, 13.0]\n[[3.0, 6.0], [42.25, 144.0]]\n"
	if out != want {
		t.Fatalf("yerinde bileşik atama %q, beklenen %q", out, want)
	}
}

// Kural (15.6d): sonucun hassasiyeti hedefinkidir ("bf16" += "float32" "bf16" kalır); yayın
// yalnız sağ taraftadır, sonucun şekli hedefin şekli değilse H6958; örtüşen kaynak önce kopyalanır.
func TestCompoundAssignDTypeShapeOverlap(t *testing.T) {
	out := mustRun(t, `TR_var b = TR_Tensor([1.0, 2.0], TR_dtype: "bf16")
b += TR_Tensor([0.5, 0.5])
TR_var h = TR_Tensor([1.0, 2.0], TR_dtype: "fp16")
h *= 3
TR_var m = TR_Tensor([[1.0, 2.0], [3.0, 4.0]])
m += TR_Tensor([10.0, 20.0])
TR_var o = TR_Tensor([1.0, 2.0, 3.0, 4.0])
o[1..4] += o[0..3]
TR_print(b.TR_dtype, b.TR_tolist, h.TR_dtype, h.TR_tolist, m.TR_tolist, o.TR_tolist)`)
	want := "bf16 [1.5, 2.5] fp16 [3.0, 6.0] [[11.0, 22.0], [13.0, 24.0]] [1.0, 3.0, 5.0, 7.0]\n"
	if out != want {
		t.Fatalf("hassasiyet, yayın ve örtüşme %q, beklenen %q", out, want)
	}
	expectCodes(t, []codeCase{
		{"TR_var t = TR_zeros([1])\nt += TR_ones([3])", messages.H6958},
		{"TR_var t = TR_zeros([2, 1])\nt *= TR_ones([2, 3])", messages.H6958},
		{"TR_var t = TR_zeros([2])\nt[0] += TR_ones([2])", messages.H6958},
	})
	if err := checkInPlaceDType(tensor.Int8); !messages.Has(err, messages.H6957) {
		t.Fatalf("nicemlenmiş hedef: %v", err)
	}
	if err := checkInPlaceDType(tensor.Int4); !messages.Has(err, messages.H6957) {
		t.Fatalf("4 bitlik hedef: %v", err)
	}
}

// Kural (15.6d, 15.3): bileşik atama deponun sürümünü artırır; geri yayılım için saklanmış
// tensörün değişmesi TR_backward'da yakalanır. Türevin üç yazma kuralı bileşik atamada da geçerli.
func TestCompoundAssignGradRules(t *testing.T) {
	setup := `TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var h = w * 2.0
TR_var havuz = TR_zeros([2])
`
	expectCodes(t, []codeCase{
		{setup + "w -= 0.1", messages.H6716},
		{setup + "w[0] -= 0.1", messages.H6716},
		{setup + "TR_no_grad { h += 1.0 }", messages.H6955},
		{setup + "h[0] += 1.0", messages.H6955},
		{setup + "havuz += w * 2.0", messages.H6956},
		{setup + "havuz[0] += (w * 2.0)[0]", messages.H6956},
		{setup + "TR_var c = TR_Tensor([2.0])\nTR_var y = TR_sum(w * c)\nc += 1.0\nTR_backward(y)", messages.H3012},
	})
	out := mustRun(t, setup+`TR_no_grad {
    w -= 0.5
    w[1] *= 2.0
    havuz += w * 2.0
}
TR_print(w.TR_tolist, havuz.TR_tolist)`)
	if want := "[0.5, 3.0] [1.0, 6.0]\n"; out != want {
		t.Fatalf("türevsiz kipte bileşik atama %q, beklenen %q", out, want)
	}
}

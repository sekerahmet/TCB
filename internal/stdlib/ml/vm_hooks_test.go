package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// VM kancaları: eşitlik, sağ işlenen, '**', karşılaştırma, tekli eksi ve indeksleme.
func TestVMHooks(t *testing.T) {
	out := mustRun(t, `
		TR_var t = TR_Tensor([[1.0, 2.0], [3.0, 4.0]])
		TR_print(TR_equal(TR_Tensor([1.00001]), TR_Tensor([1.0])))
		TR_print(TR_equal(TR_Tensor([1.0, 2.0]), TR_Tensor([1.0, 2.0])))
		TR_print(TR_not TR_equal(TR_Tensor([1.0, 2.0]), TR_Tensor([[1.0, 2.0]])))
		TR_print((10 - t).TR_tolist)
		TR_print((12 / t).TR_tolist)
		TR_print((-t).TR_tolist)
		TR_print((t ** 2).TR_tolist)
		TR_print((t > 2).TR_tolist)
		TR_print((2 >= t).TR_tolist)
		TR_print(t[1].TR_tolist)
		TR_print(t[-1][0].TR_tolist)
		TR_print(t[0..1].TR_shape)
		TR_print([1, 2] @ TR_Tensor([[1.0], [1.0]]))
	`)
	want := []string{
		"false", "true", "true",
		"[[9.0, 8.0], [7.0, 6.0]]",
		"[[12.0, 6.0], [4.0, 3.0]]",
		"[[-1.0, -2.0], [-3.0, -4.0]]",
		"[[1.0, 4.0], [9.0, 16.0]]",
		"[[0.0, 0.0], [1.0, 1.0]]",
		"[[1.0, 1.0], [0.0, 0.0]]",
		"[3.0, 4.0]",
		"3.0",
		"[1, 2]",
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i, w := range want {
		if i >= len(lines) || lines[i] != w {
			t.Fatalf("%d. satır: beklenen %q\ntüm çıktı:\n%s", i+1, w, out)
		}
	}
}

// Kanca hataları satır bilgisi taşımalı ve yol göstermeli; türev sağ işlenen yolunda da akmalı.
func TestVMHookErrorsAndGrad(t *testing.T) {
	_, err := run(t, "TR_var t = TR_Tensor([1.0])\n\nTR_var y = t % 2\n")
	if err == nil || !strings.HasPrefix(err.Error(), "Satır 3:") || !messages.Has(err, messages.H6703) {
		t.Fatalf("satır bilgili, yol gösteren hata bekleniyordu: %v", err)
	}
	_, err = run(t, "TR_var t = TR_Tensor([1.0, 2.0])\nTR_print(t[5])\n")
	if err == nil || !strings.HasPrefix(err.Error(), "Satır 2:") || !messages.Has(err, messages.H6709) {
		t.Fatalf("indeks hatası bekleniyordu: %v", err)
	}
	_, err = run(t, "TR_var t = TR_Tensor([[1.0, 2.0]])\nTR_print(t @ TR_Tensor([[1.0, 2.0]]))\n")
	if err == nil || !strings.HasPrefix(err.Error(), "Satır 2:") {
		t.Fatalf("'@' hatası satır bilgisi taşımalı: %v", err)
	}
	out := mustRun(t, `
		TR_var w = TR_Tensor([2.0, 4.0], TR_requires_grad: TR_true)
		TR_var kayip = TR_sum(1 / w - w[0] * 3)
		TR_backward(kayip)
		TR_print(w.TR_grad.TR_tolist)
	`)
	if strings.TrimSpace(out) != "[-6.25, -0.0625]" {
		t.Fatalf("türev: %q", out)
	}
}

// Tensör üzerinde döngü ilk eksen boyunca gezer: 1 boyutluda sayılar (tam sayılı tensörde
// Tam), N boyutluda veriyi paylaşan, türev izlenen (N-1) boyutlu görünümler.
func TestLoopOverTensor(t *testing.T) {
	out := mustRun(t, `
		TR_var t = TR_Tensor([[1.0, 2.0], [3.0, 4.0], [5.0, 6.0]])
		TR_for i, s TR_in t { TR_print(i, s.TR_shape, s.TR_tolist) }
		TR_print([x * 2 TR_for x TR_in TR_Tensor([1.5, 2.5])])
		TR_print([x TR_for x TR_in TR_Tensor([0.1])][0] == 0.1)
		TR_print([TR_Type(x) TR_for x TR_in TR_argmax(TR_Tensor([[1.0, 5.0], [7.0, 0.0]]), TR_dim: -1)])
		TR_var k = TR_argmax(TR_Tensor([[[0.0, 9.0], [5.0, 0.0]]]), TR_dim: -1)
		TR_for s TR_in k { TR_for x TR_in s { TR_print(TR_Type(x), x) } }
		TR_var z = TR_zeros([2, 2])
		TR_for satır TR_in z { satır[0] = 9 }
		TR_print(z.TR_tolist)
		TR_for x TR_in TR_zeros([0]) { TR_print("hiç") }
		TR_var w = TR_Tensor([[1.0, 2.0], [3.0, 4.0]], TR_requires_grad: TR_true)
		TR_var kayip = TR_sum(TR_Tensor(0.0))
		TR_for s TR_in w { kayip = kayip + TR_sum(s * s) }
		TR_backward(kayip)
		TR_print(w.TR_grad.TR_tolist)
	`)
	want := strings.Join([]string{
		"0 [2] [1.0, 2.0]",
		"1 [2] [3.0, 4.0]",
		"2 [2] [5.0, 6.0]",
		"[3.0, 5.0]",
		"true",
		`[Int, Int]`,
		"Int 1",
		"Int 0",
		"[[9.0, 0.0], [9.0, 0.0]]",
		"[[2.0, 4.0], [6.0, 8.0]]",
	}, "\n")
	if strings.TrimSpace(out) != want {
		t.Fatalf("tensör döngüsü:\n%s\nbeklenen:\n%s", out, want)
	}
	for _, code := range []string{
		"TR_var a = TR_Tensor(3.0)\n\nTR_for x TR_in a { }\n",
		"TR_var a = TR_Tensor(3.0)\n\nTR_print([x TR_for x TR_in a])\n",
	} {
		_, err := run(t, code)
		if err == nil || !strings.HasPrefix(err.Error(), "Satır 3:") || !messages.Has(err, messages.H6711) {
			t.Errorf("0 boyutlu tensörde satır bilgili döngü hatası bekleniyordu: %v", err)
		}
	}
}

// Bulgu 7: indeksle atama tek kopya ve tek sürüm artışıdır (eleman eleman yazılmaz);
// bitişik olmayan hedefte de.
func TestIndexAssignSingleCopy(t *testing.T) {
	v := value(t, `TR_var cikti = TR_zeros([300, 200])
cikti[0..300] = TR_ones([300, 200])
cikti.TR_transpose()[5] = 2
cikti[1][2] = 3`)
	d := v.(*TensorValue)
	if s := d.t.Version(); s != 3 {
		t.Fatalf("3 atama için sürüm %d; her atama bir kez artırmalı", s)
	}
	data := d.t.Data()
	if data[0] != 1 || data[5] != 2 || data[200+5] != 2 || data[299*200+5] != 2 || data[200+2] != 3 || data[6] != 1 {
		t.Fatalf("atama sonucu yanlış: %v", data[:8])
	}
}

// Tensör aralık indekslemesi listelerinkiyle aynıdır: negatif uç sondan sayılır, dahil uç
// (..=) bundan sonra eklenir, sınır dışı uçlar kırpılır.
func TestRangeIndexMatchesLists(t *testing.T) {
	out := mustRun(t, `TR_var l = [1, 2, 3]
TR_var t = TR_Tensor([1.0, 2.0, 3.0])
TR_for a TR_in [l[0..=-1], l[-5..2], l[2..1], l[1..=10], l[-2..=-2], l[-1..3]] { TR_print(a) }
TR_for b TR_in [t[0..=-1], t[-5..2], t[2..1], t[1..=10], t[-2..=-2], t[-1..3]] { TR_print(b.TR_tolist) }`)
	want := "[1, 2, 3]\n[1, 2]\n[]\n[2, 3]\n[2]\n[3]\n" +
		"[1.0, 2.0, 3.0]\n[1.0, 2.0]\n[]\n[2.0, 3.0]\n[2.0]\n[3.0]\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Kural: tensör dilimi ile liste dilimi sınırları tek yerde (interp.SliceBounds) hesaplanır;
// en büyük tam sayı üst ucu ikisinde de bütün kalanı verir.
// Neden: tensörün eski kopyası 'son++' ile taşıp dilimi boş yapıyordu, liste ise tam veriyordu.
func TestRangeIndexMaxInclusiveEndMatchesLists(t *testing.T) {
	out := mustRun(t, `TR_var l = [1, 2, 3]
TR_var t = TR_Tensor([1.0, 2.0, 3.0])
TR_print(l[0..=9223372036854775807])
TR_print(t[0..=9223372036854775807].TR_tolist)`)
	want := "[1, 2, 3]\n[1.0, 2.0, 3.0]\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Kural (15.6d "Türev"): yerinde yazmanın üç kuralı kipe bağlıdır, sözcüksel değil.
//   - türev izleyen yaprağa ve görünümüne yalnız türevsiz kipte yazılır (H6716);
//   - türev izleyen ara sonuca yerinde yazmak türevsiz kipte de hatadır (H6955);
//   - türev izlemeyen hedefe kipin dışında türev izleyen değer yazmak hatadır (H6956).
//
// Aynı kurallar TR_write_slice'ta da geçerlidir.
func TestWriteRulesForGradients(t *testing.T) {
	setup := `TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var h = w * 2.0
TR_var havuz = TR_zeros([2])
TR_var kv = TR_zeros([2, 2])
`
	expectCodes(t, []codeCase{
		{setup + "w[0] = 5.0", messages.H6716},
		{setup + "w[0..1][0] = 5.0", messages.H6716},
		{setup + "TR_var v = w[0..2]\nv[0] = 1.0", messages.H6716},
		{setup + "TR_write_slice(w, TR_Tensor([3.0]), TR_dim: 0, TR_start: 0)", messages.H6716},
		{setup + "TR_func yaz(t: TR_Tensor) { t[0] = 9.0 }\nyaz(w)", messages.H6716},
		{setup + "h[1] = 0.0", messages.H6955},
		{setup + "TR_no_grad { h[1] = 0.0 }", messages.H6955},
		{setup + "TR_no_grad { h[0..1][0] = 0.0 }", messages.H6955},
		{setup + "TR_no_grad { TR_write_slice(h, TR_Tensor([3.0]), TR_dim: 0, TR_start: 0) }", messages.H6955},
		{setup + "havuz[0] = (w * 2.0)[0]", messages.H6956},
		{setup + "havuz[0..2] = h", messages.H6956},
		{setup + "TR_write_slice(kv, TR_Tensor([[1.0, 1.0]]) * w, TR_dim: 0, TR_start: 0)", messages.H6956},
	})
	// Türevsiz kipte (bloktan çağrılan fonksiyonda da) yaprağa ve görünümüne yazılır; türev
	// izlemeyen hedefe türev izleyen değer yazılır, bağ bilerek kopar.
	out := mustRun(t, setup+`TR_func yaz(t: TR_Tensor) { t[0] = 9.0 }
TR_var v = w[0..2]
TR_no_grad {
    yaz(w)
    v[1] = 7.0
    havuz[0..2] = h
    TR_write_slice(kv, TR_Tensor([[1.0, 1.0]]) * w, TR_dim: 0, TR_start: 1)
}
havuz[0] = 3.0
TR_print(w.TR_tolist, havuz.TR_tolist, kv.TR_tolist)`)
	if want := "[9.0, 7.0] [3.0, 4.0] [[0.0, 0.0], [9.0, 7.0]]\n"; out != want {
		t.Fatalf("türevsiz kipte yazmalar %q, beklenen %q", out, want)
	}
}

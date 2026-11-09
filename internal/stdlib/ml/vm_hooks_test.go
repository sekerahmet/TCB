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
		"[[false, false], [true, true]]",
		"[[true, true], [false, false]]",
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

// suggestedSpelling iletinin son ters tırnaklı yazımıdır: kullanıcının kopyalayacağı öneri.
func suggestedSpelling(t *testing.T, text string) string {
	t.Helper()
	parts := strings.Split(text, "`")
	if len(parts) < 3 {
		t.Fatalf("iletide ters tırnaklı öneri yok: %q", text)
	}
	return parts[len(parts)-2]
}

// 15.6d: t[idx] kopya verir; ona yazmak sessizce hiçbir şey yapmazdı. Yazma H7042'dir, t olduğu gibi
// kalır ve iletinin önerdiği döngü, kopyalanınca, kuralın istediği sonucu verir.
func TestTensorIndexWriteSuggestionRuns(t *testing.T) {
	const rows = `TR_Tensor([[1.0, 2.0], [3.0, 4.0], [5.0, 6.0]])`
	const flat = `TR_Tensor([1.0, 2.0, 3.0, 4.0])`
	cases := []struct{ t, idx, d, op, want string }{
		{flat, `TR_Tensor([1, 3])`, `5.0`, "=", "[1.0, 5.0, 3.0, 5.0]"},
		{flat, `TR_Tensor([1, 3])`, `TR_Tensor([5.0, 6.0])`, "=", "[1.0, 5.0, 3.0, 6.0]"},
		{flat, `TR_Tensor([1, 3])`, `TR_Tensor(5.0)`, "=", "[1.0, 5.0, 3.0, 5.0]"},
		{flat, `TR_Tensor([1, 3])`, `[5.0, 6.0]`, "=", "[1.0, 5.0, 3.0, 6.0]"},
		{flat, `TR_Tensor(1)`, `5.0`, "=", "[1.0, 5.0, 3.0, 4.0]"},
		{flat, `TR_Tensor(1)`, `TR_Tensor(5.0)`, "=", "[1.0, 5.0, 3.0, 4.0]"},
		{flat, `TR_Tensor[TR_Int]([])`, `5.0`, "=", "[1.0, 2.0, 3.0, 4.0]"},
		{rows, `TR_Tensor([0, 2])`, `TR_Tensor([7.0, 8.0])`, "=", "[[7.0, 8.0], [3.0, 4.0], [7.0, 8.0]]"},
		{rows, `TR_Tensor([0, 2])`, `[7.0, 8.0]`, "=", "[[7.0, 8.0], [3.0, 4.0], [7.0, 8.0]]"},
		{rows, `TR_Tensor([2, 0])`, `TR_Tensor([[7.0, 8.0], [9.0, 0.0]])`, "=", "[[9.0, 0.0], [3.0, 4.0], [7.0, 8.0]]"},
		{rows, `TR_Tensor([2, 0])`, `[[7.0, 8.0], [9.0, 0.0]]`, "=", "[[9.0, 0.0], [3.0, 4.0], [7.0, 8.0]]"},
		{rows, `TR_Tensor([[0, 2]])`, `0.0`, "=", "[[0.0, 0.0], [3.0, 4.0], [0.0, 0.0]]"},
		{rows, `TR_Tensor([[0, 2]])`, `TR_Tensor([[[7.0, 8.0], [9.0, 0.0]]])`, "=", "[[7.0, 8.0], [3.0, 4.0], [9.0, 0.0]]"},
		{`TR_Tensor([0.0, 0.0, 0.0])`, `TR_Tensor([0, 0, 0])`, `1.0`, "+=", "[3.0, 0.0, 0.0]"},
		{`TR_Tensor([0.0, 0.0, 0.0])`, `TR_Tensor([0, 2, 0])`, `TR_Tensor([1.0, 2.0, 3.0])`, "+=", "[4.0, 0.0, 2.0]"},
		{flat, `TR_Tensor([1, 3])`, `1.0`, "-=", "[1.0, 1.0, 3.0, 3.0]"},
		{rows, `TR_Tensor([0, 2])`, `TR_Tensor([1.0, 2.0])`, "*=", "[[1.0, 4.0], [3.0, 4.0], [5.0, 12.0]]"},
		{`TR_Tensor[TR_Int]([1, 2, 3])`, `TR_Tensor([0, 2])`, `5`, "=", "[5, 2, 5]"},
		{`TR_Tensor[TR_Int]([1, 2, 3])`, `TR_Tensor([0, 2])`, `TR_Tensor[TR_Int]([5, 6])`, "*=", "[5, 2, 18]"},
		{`TR_Tensor([1.0, 2.0], TR_dtype: "bf16")`, `TR_Tensor([1])`, `5.0`, "=", "[1.0, 5.0]"},
	}
	for _, c := range cases {
		prelude := "TR_var t = " + c.t + "\nTR_var idx = " + c.idx + "\nTR_var d = " + c.d + "\n"
		wrong := prelude + "t[idx] " + c.op + " d\n"
		_, err := run(t, wrong)
		if err == nil || !messages.Has(err, messages.H7042) {
			t.Errorf("%s:\n  hata: %v, H7042 bekleniyordu", wrong, err)
			continue
		}
		spelling := suggestedSpelling(t, err.Error())
		if strings.Contains(spelling, "=") && !strings.Contains(spelling, " "+c.op+" ") {
			t.Errorf("%s: öneri işleci taşımıyor: %q", wrong, spelling)
		}
		kept := mustRun(t, prelude+"TR_var kod = TR_error_code { t[idx] "+c.op+" d }\nTR_print(kod, t.TR_tolist == "+c.t+".TR_tolist)")
		if kept != "H7042 true\n" {
			t.Errorf("%s: t değişti ya da hata başka: %q", wrong, kept)
		}
		got := mustRun(t, prelude+spelling+"\nTR_print(t.TR_tolist)")
		if got != c.want+"\n" {
			t.Errorf("%s:\n  öneri: %s\n  çıktı: %q\n  beklenen: %s", wrong, spelling, got, c.want)
		}
	}
}

// Önerilen döngü yinelenen indekste ve örtüşen veride kuraldan ayrılır (DENETİM 172): ileti şartını söyler.
func TestTensorIndexWriteNamesTheConditionOfTheLoop(t *testing.T) {
	text := messages.TextIn("en", messages.H7042, "+", "whole")
	for _, word := range []string{"no repeated item", "neither `d` nor `idx` shares data with `t`"} {
		if !strings.Contains(text, word) {
			t.Errorf("iletide %q yok: %s", word, text)
		}
	}
}

// Öbür hatalar H7042'den önce gelir: eksi indeks tensörle indekste sınır dışıdır (öneri döngüsü
// sondan sayardı), türev kuralı ve indeksin türü olduğu gibi kalır. Maske yolu H6707'dir, sessiz değildir.
func TestTensorIndexWriteKeepsTheOtherErrors(t *testing.T) {
	const flat = `TR_var t = TR_Tensor([1.0, 2.0, 3.0])
`
	const leaf = `TR_var t = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
`
	var cases []codeCase
	for _, op := range []string{"=", "+="} {
		cases = append(cases,
			codeCase{flat + "t[TR_Tensor([-1])] " + op + " 1.0", messages.H6694},
			codeCase{flat + "t[TR_Tensor([3])] " + op + " 1.0", messages.H6694},
			codeCase{flat + "t[TR_Tensor([9223372036854775807])] " + op + " 1.0", messages.H6694},
			codeCase{flat + "t[TR_Tensor([0.0])] " + op + " 1.0", messages.H6707},
			codeCase{flat + "t[t > 1.0] " + op + " 1.0", messages.H6707},
			codeCase{"TR_var t = TR_Tensor(1.0)\nt[TR_Tensor([0])] " + op + " 1.0", messages.H6704},
			codeCase{leaf + "t[TR_Tensor([0])] " + op + " 1.0", messages.H6716},
			codeCase{leaf + "TR_var h = t * 2.0\nh[TR_Tensor([0])] " + op + " 1.0", messages.H6955},
			codeCase{leaf + "TR_no_grad { t[TR_Tensor([0])] " + op + " 1.0 }", messages.H7042},
			codeCase{flat + "TR_var v = t[0..2]\nv[TR_Tensor([0])] " + op + " 1.0", messages.H7042},
		)
	}
	expectCodes(t, append(cases, codeCase{flat + "TR_print(t[t > 1.0])", messages.H6707}))
}

// Kural (7.7, 5.6; TCB'nin kararı, 2026-11-09): durum deseni karşılaştırmaz, yalnız durumu sınar; durumun
// alanı tensör tutsa da geçer: durum açan desen (Cevap.Tamam(x) =>) ve parantezsiz alansız durum
// (Cevap.Yok =>). Parantezli (Cevap.Yok) => değer desenidir, '==' ister: tensör alanlı konuda hatadır.
// Gerileme (birleşik 63, 0a8073a4): eşitliğin hatası değere bağlanınca (4.5) iki durum deseni de konunun
// tensör alanında H2770 veriyordu; 6311e11a'da "tamam" ve "yok" veriyordu.
func TestAStatePatternDoesNotCompareTheFields(t *testing.T) {
	const enum = "TR_enum Cevap {\n    Tamam(t: TR_Tensor)\n    Yok\n}\n" +
		"TR_func ad(c: Cevap) -> TR_String {\n    TR_return TR_match c {\n        Cevap.Yok => \"yok\"\n        Cevap.Tamam(x) => \"tamam\"\n    }\n}\n"
	out, err := run(t, enum+"TR_print(ad(Cevap.Tamam(TR_Tensor([1.0]))), ad(Cevap.Yok))")
	if err != nil || out != "tamam yok\n" {
		t.Fatalf("durum deseni: %q %v", out, err)
	}
	expectCodes(t, []codeCase{{enum + "TR_var z = TR_match Cevap.Tamam(TR_Tensor([1.0])) {\n    (Cevap.Yok) => 1\n    _ => 2\n}", messages.H2770}})
}

// Kural (15.5, 4.5, 12.4): tipi bilinmeyen değerde (denetimin göremediği yerde) motor denetimin kuralını
// izler: tensör tutan değerin '=='si, .TR_contains, .TR_index_of, .TR_unique ve değer deseni çalışma
// hatasıdır (H2777; yapı alanında H2770), aranan değer boş listede de. TR_assert_equal ise içteki her
// tensör çiftini TR_equal'ın anlamıyla (şekil ve bütün öğeler) karşılaştırır: [t] ile [u] eşitse geçer,
// değilse ikisini göstererek kalır. Bu 12.4'ün TCB'deki okumasıdır (2026-11-09); metnin açıklaması
// pending'in 12.4 satırında turdadır, tur başka bir okuma seçerse bu test onunla değişir. Fonksiyon
// TR_assert_equal'da da hatadır. Aynı nesne de öğe öğe: NaN tutan tensör kendisine eşit değildir.
func TestTensorInsideAValueIsAnErrorExceptInAssertEqual(t *testing.T) {
	const x = "TR_var t = TR_Tensor([1.0])\n"
	expectCodes(t, []codeCase{
		{x + "TR_var z = [t] == [t]", messages.H2777},
		{x + "TR_var z = [[t]] != [[t]]", messages.H2777},
		{x + "TR_var z = {\"k\": t} == {\"k\": t}", messages.H2777},
		{x + "TR_var z = (t, 1) == (t, 1)", messages.H2777},
		{x + "TR_var z = ([t], 1) == ([t], 2)", messages.H2777},
		{x + "TR_var z = [t].TR_contains(t)", messages.H2777},
		{x + "TR_var z = [1].TR_contains(t)", messages.H2777},
		{x + "TR_var z = [].TR_index_of([t])", messages.H2777},
		{x + "TR_var z = [t].TR_unique()", messages.H2777},
		{x + "TR_var z = TR_match [t] {\n    ([t]) => 1\n    _ => 2\n}", messages.H2777},
		{x + "TR_var z = TR_match t {\n    (t) => 1\n    _ => 2\n}", messages.H2777},
		{x + "TR_struct K {\n    t: TR_Tensor\n}\nTR_var z = K(t) == K(t)", messages.H2770},
		{x + "TR_assert_equal([t], [TR_Tensor([2.0])])", messages.H2458},
		{x + "TR_assert_equal([t], [TR_Tensor([1.0, 1.0])])", messages.H2458},
		{"TR_var n = TR_Tensor([TR_nan])\nTR_assert_equal([n], [n])", messages.H2458},
		{"TR_var n = TR_Tensor([TR_nan])\nTR_assert_equal(n, n)", messages.H2458},
		{"TR_var f = (x) => x\nTR_assert_equal([f], [f])", messages.H2706},
	})
	for _, code := range []string{
		x + "TR_var u = TR_Tensor([1.0])\nTR_assert_equal([t], [u])\nTR_assert_equal(t, u)",
		x + "TR_var u = TR_Tensor([1.0])\nTR_assert_equal({\"k\": (t, [t])}, {\"k\": (u, [u])})",
		x + "TR_struct K {\n    t: TR_Tensor\n}\nTR_assert_equal(K(t), K(TR_Tensor([1.0])))",
		x + "TR_var l = [t]\nl.TR_append(l)\nTR_var m = [TR_Tensor([1.0])]\nm.TR_append(m)\nTR_assert_equal(l, m)",
		x + "TR_var g = [t, 0][0]\nTR_assert(g != TR_none TR_and [1].TR_contains(TR_none) == TR_false)",
	} {
		if out, err := run(t, code); err != nil {
			t.Errorf("%s:\n  hata beklenmiyordu: %v %s", code, err, out)
		}
	}
}

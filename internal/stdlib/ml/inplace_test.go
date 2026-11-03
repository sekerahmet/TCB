package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
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
	if err := checkInPlaceDType(tensor.Q8); !messages.Has(err, messages.H6957) {
		t.Fatalf("nicemlenmiş hedef: %v", err)
	}
	if err := checkInPlaceDType(tensor.Q4); !messages.Has(err, messages.H6957) {
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

// Kural (15.6d, 15.6a, 15.2): cinsli hedefe (TR_Int, TR_Bool, float64) atama ve bileşik atama
// hedefin cinsinde ve hassasiyetinde yapılır: sayı hedefin cinsinden olmalı ve hassasiyetine
// sığmalı; aynı cinsin başka hassasiyetinden tensör sığma denetimiyle çevrilir; başka cinsten
// değer H6966. Bileşik atama hedefin cinsini değiştirmez (TR_Int'te `/=` H6966); H6957 yalnız
// nicemlenmiş hedeftedir. 27979f33'te çalışan `t[1] = 5` (t tam sayı değişmezli) çalışır.
func TestTypedAssign(t *testing.T) {
	out := mustRun(t, `TR_var t = TR_Tensor([0, 0, 0])
t[1] = 5
t += 1
TR_var u8 = TR_Tensor([1, 2, 3], TR_dtype: "uint8")
u8[0..2] = TR_Tensor([7, 8])
u8[2] = TR_Tensor([9])[0]
u8 *= 2
TR_var i32 = TR_Tensor([1, 2], TR_dtype: "int32")
i32 += TR_Tensor([10, 20])
i32[1] -= 2
TR_var d = TR_Tensor([1.0, 2.0], TR_dtype: "float64")
d[0] = 5
d[1] = TR_Tensor(0.25)
d -= 0.5
d /= 2
TR_var m = TR_Tensor([TR_true, TR_false])
m[1] = TR_true
TR_var f = TR_Tensor([1.0, 2.0])
f[0] = TR_Tensor(3.0, TR_dtype: "float64")
TR_print(t.TR_tolist, t.TR_dtype, u8.TR_tolist, u8.TR_dtype, i32.TR_tolist, i32.TR_dtype)
TR_print(d.TR_tolist, d.TR_dtype, m.TR_tolist, f.TR_tolist, f.TR_dtype)`)
	want := "[1, 6, 1] int64 [14, 16, 18] uint8 [11, 20] int32\n[2.25, -0.125] float64 [true, true] [3.0, 2.0] float32\n"
	if out != want {
		t.Fatalf("cinsli atama %q, beklenen %q", out, want)
	}
	setup := "TR_var t = TR_Tensor([1, 2])\nTR_var u8 = TR_Tensor([1, 2], TR_dtype: \"uint8\")\nTR_var m = TR_Tensor([TR_true, TR_false])\n"
	expectCodes(t, []codeCase{
		{setup + "u8[0] = 300", messages.H6964},
		{setup + "u8[0..2] = TR_Tensor([7, 300])", messages.H6964},
		{setup + "u8 += 300", messages.H6964},
		{setup + "u8 *= TR_Tensor([1, 200], TR_dtype: \"uint8\")", messages.H6961},
		{setup + "t[0] = 2.5", messages.H6968},
		{setup + "t[0] = TR_true", messages.H6968},
		{setup + "m[0] = 1", messages.H6968},
		{setup + "t[0..2] = m", messages.H6966},
		{setup + "TR_var f = TR_Tensor([1.0, 2.0])\nf[0] = t[0]", messages.H6966},
		{setup + "t /= 2", messages.H6966},
		{setup + "m += m", messages.H6967},
	})
}

// Kural (15.6d): bileşik atamaların hepsi (//=, %= ve bit işleçleri dahil) tensörü yerinde
// değiştirir; tensörü paylaşan ad (p = t) değişikliği her cinste görür. İndeksli hedefte de.
func TestTypedCompoundAssignAliases(t *testing.T) {
	cases := []struct{ setup, op, rhs, want string }{
		{"TR_Tensor([5, 7])", "+", "1", "[6, 8]"},
		{"TR_Tensor([5, 7])", "-", "1", "[4, 6]"},
		{"TR_Tensor([5, 7])", "*", "2", "[10, 14]"},
		{"TR_Tensor([5, 7])", "**", "2", "[25, 49]"},
		{"TR_Tensor([5, 7])", "//", "2", "[2, 3]"},
		{"TR_Tensor([5, 7])", "%", "3", "[2, 1]"},
		{"TR_Tensor([5, 7])", "&", "6", "[4, 6]"},
		{"TR_Tensor([5, 7])", "|", "8", "[13, 15]"},
		{"TR_Tensor([5, 7])", "^", "1", "[4, 6]"},
		{"TR_Tensor([5, 7])", "<<", "1", "[10, 14]"},
		{"TR_Tensor([5, 7])", ">>", "1", "[2, 3]"},
		{"TR_Tensor([5, 7], TR_dtype: \"int8\")", "//", "2", "[2, 3]"},
		{"TR_Tensor([5, 7], TR_dtype: \"int8\")", "<<", "1", "[10, 14]"},
		{"TR_Tensor([TR_true, TR_false])", "&", "TR_Tensor([TR_true, TR_true])", "[true, false]"},
		{"TR_Tensor([TR_true, TR_false])", "|", "TR_Tensor([TR_false, TR_true])", "[true, true]"},
		{"TR_Tensor([TR_true, TR_false])", "^", "TR_Tensor([TR_true, TR_true])", "[false, true]"},
		{"TR_Tensor([5.5, 7.0], TR_dtype: \"float64\")", "+", "1", "[6.5, 8.0]"},
		{"TR_Tensor([5.5, 7.0], TR_dtype: \"float64\")", "/", "2", "[2.75, 3.5]"},
		{"TR_Tensor([5.5, 7.0], TR_dtype: \"float64\")", "%", "2", "[1.5, 1.0]"},
	}
	for _, c := range cases {
		program := "TR_var t = " + c.setup + "\nTR_var p = t\nt " + c.op + "= " + c.rhs +
			"\nTR_var u = " + c.setup + "\nTR_var q = u\nu[0..2] " + c.op + "= " + c.rhs +
			"\nTR_print(p.TR_tolist, q.TR_tolist, p.TR_dtype == " + c.setup + ".TR_dtype)"
		if out := mustRun(t, program); out != c.want+" "+c.want+" true\n" {
			t.Errorf("%s %s= %s: %q, beklenen p ve q %s", c.setup, c.op, c.rhs, out, c.want)
		}
	}
	// Cinsin tanımlamadığı işleç yerinde yolda da işlecin hatasını verir.
	expectCodes(t, []codeCase{
		{"TR_var f = TR_Tensor([5.0, 7.0])\nf //= 2.0", messages.H6703},
		{"TR_var f = TR_Tensor([5.0, 7.0])\nf[0..1] %= 2.0", messages.H6703},
		{"TR_var f = TR_Tensor([5.0, 7.0])\nf &= 1.0", messages.H6703},
		{"TR_var m = TR_Tensor([TR_true])\nm <<= 1", messages.H6968},
	})
}

// Kural (dil tanımı 15.34): türevsiz kipte 16 bitlik bir parametreye bileşik atama programın hata
// akışına aynı yerde bir kez uyarı yazar: yerel ad, alan, liste öğesi ve indeksli yazım. Döngüde
// iki kez çalışan satır tek uyarı verir; güncelleme yutulur (parametre değişmez).
func TestHalfParamWarningOncePerPlace(t *testing.T) {
	o := trrun.MustExecChecked(t, `TR_var p = TR_Tensor([1.0, 2.0], TR_dtype: "bf16", TR_requires_grad: TR_true)
TR_struct K { w: TR_Tensor }
TR_var k = K(p)
TR_var l = [p]
TR_no_grad {
    TR_for i TR_in 0..2 {
        p -= 0.001
        k.w -= 0.001
        l[0] -= 0.001
        p[0..2] -= 0.001
    }
}
TR_print(p.TR_tolist)`)
	if o.Out != "[1.0, 2.0]\n" {
		t.Fatalf("çıktı: %q", o.Out)
	}
	for _, line := range []string{"satır 7)", "satır 8)", "satır 9)", "satır 10)"} {
		if n := strings.Count(o.ErrOut, "Uyarı ("+line); n != 1 {
			t.Errorf("%s: %d uyarı, 1 bekleniyordu:\n%s", line, n, o.ErrOut)
		}
	}
	if n := strings.Count(o.ErrOut, "Uyarı"); n != 4 {
		t.Fatalf("%d uyarı, 4 bekleniyordu:\n%s", n, o.ErrOut)
	}
}

// Kural: uyarı yalnız 16 bitlik (bf16, fp16) parametrededir, donmuş parametre dahil (15.33);
// parametre olmayan 16 bitlik tensörde, float32 parametrede ve türevli kipte (donmuş parametreye
// yazım orada da serbesttir) yoktur. Uyarılar programa bağlıdır.
func TestHalfParamWarningOnlyHalfParameters(t *testing.T) {
	const code = `TR_var q = TR_Tensor([1.0], TR_dtype: "bf16")
TR_var r = TR_Tensor([1.0], TR_requires_grad: TR_true)
TR_var f = TR_Tensor([1.0], TR_dtype: "fp16", TR_requires_grad: TR_true)
TR_var d = TR_Tensor([1.0], TR_dtype: "bf16", TR_requires_grad: TR_true)
d.TR_requires_grad = TR_false
TR_no_grad {
    q -= 0.5
    r -= 0.5
    f -= 0.5
    d -= 0.5
}
d += 0.25
TR_print(q.TR_tolist, r.TR_tolist, f.TR_tolist, d.TR_tolist)`
	for i := 0; i < 2; i++ {
		o := trrun.MustExecChecked(t, code)
		if o.Out != "[0.5] [0.5] [0.5] [0.75]\n" {
			t.Fatalf("çıktı: %q", o.Out)
		}
		if strings.Count(o.ErrOut, "Uyarı") != 2 || !strings.Contains(o.ErrOut, "Uyarı (satır 9)") ||
			!strings.Contains(o.ErrOut, "(fp16)") || !strings.Contains(o.ErrOut, "Uyarı (satır 10)") {
			t.Fatalf("%d. program: fp16 ve donmuş bf16 parametre uyarmalı, öbürleri değil:\n%s", i+1, o.ErrOut)
		}
	}
}

// Kural: iletinin önerdiği yazım (float32 ana kopya, güncelleme onda, parametreye indeksle yazım)
// uyarısız çalışır ve küçük güncellemeleri biriktirir.
func TestHalfParamWarningSuggestionRuns(t *testing.T) {
	for lang, want := range map[string]string{"tr": "`p[0..p.TR_shape[0]] = ana`", "en": "`p[0..p.TR_shape[0]] = master`"} {
		if text := messages.TextIn(lang, messages.H6994, "main", 3, "bf16", "-"); !strings.Contains(text, want) {
			t.Errorf("%s: ileti %s önerisini içermiyor: %q", lang, want, text)
		}
	}
	o := trrun.MustExecChecked(t, `TR_var p = TR_Tensor([1.0, 2.0], TR_dtype: "bf16", TR_requires_grad: TR_true)
TR_no_grad {
    TR_var ana = p.TR_to_dtype("float32")
    TR_for i TR_in 0..10 { ana -= 0.001 }
    p[0..p.TR_shape[0]] = ana
}
TR_print(p.TR_tolist, p.TR_dtype)`)
	if o.ErrOut != "" || o.Out != "[0.98828125, 1.9921875] bf16\n" {
		t.Fatalf("önerilen yazım: çıktı %q, hata akışı %q", o.Out, o.ErrOut)
	}
}

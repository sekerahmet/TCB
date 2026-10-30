package ml

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestSinCos(t *testing.T) {
	mustRun(t, `
TR_assert_equal(0.0, TR_sin(0))
TR_assert_equal(1.0, TR_cos(0))
TR_assert_equal(TR_Float, TR_Type(TR_sin(1)))
TR_var x = TR_Tensor([0.0, 1.0, -2.0], TR_requires_grad: TR_true)
TR_var y = TR_sin(x)
TR_assert(TR_abs(y.TR_tolist[1] - 0.84147096) < 0.0000001)
TR_backward(TR_sum(y * 1))
TR_assert(TR_abs(x.TR_grad.TR_tolist[2] - TR_cos(-2.0)) < 0.000001)
TR_var z = TR_Tensor([0.5], TR_requires_grad: TR_true)
TR_backward(TR_sum(TR_cos(z)))
TR_assert(TR_abs(z.TR_grad.TR_tolist[0] + TR_sin(0.5)) < 0.000001)
`)
}

func TestConcatAndSplit(t *testing.T) {
	mustRun(t, `
TR_var a = TR_Tensor([[1, 2], [3, 4]], TR_requires_grad: TR_true)
TR_var b = TR_Tensor([[5], [6]], TR_requires_grad: TR_true)
TR_var c = TR_concat([a, b])
TR_assert_equal([[1.0, 2.0, 5.0], [3.0, 4.0, 6.0]], c.TR_tolist)
TR_assert_equal([4, 2], TR_concat([a, a], TR_dim: 0).TR_shape)
TR_backward(TR_sum(c * TR_Tensor([1, 2, 3])))
TR_assert_equal([[1.0, 2.0], [1.0, 2.0]], a.TR_grad.TR_tolist)
TR_assert_equal([[3.0], [3.0]], b.TR_grad.TR_tolist)
TR_var p = TR_Tensor([[1, 2, 3, 4, 5, 6]], TR_requires_grad: TR_true).TR_split(3)
TR_assert_equal(3, p.TR_length)
TR_assert_equal([[3.0, 4.0]], p[1].TR_tolist)
TR_var s = TR_Tensor([[1, 2], [3, 4], [5, 6], [7, 8]]).TR_split(2, TR_dim: 0)
TR_assert_equal([[5.0, 6.0], [7.0, 8.0]], s[1].TR_tolist)
# Metin'in .böl'ü değişmedi.
TR_assert_equal(["a", "b"], "a,b".TR_split(","))
`)
	errs := []struct {
		code string
		want messages.Code
	}{
		{"TR_concat([])", messages.H6307},
		{"TR_concat(TR_Tensor([1]))", messages.H6306},
		{"TR_concat([TR_Tensor([[1, 2]]), TR_Tensor([[1, 2, 3]])], TR_dim: 0)", messages.H6311},
		{"TR_concat([TR_Tensor([[1, 2]]), TR_Tensor([1, 2])])", messages.H6309},
		{"TR_concat([TR_Tensor([1]), TR_Tensor([2])], TR_dim: 3)", messages.H6016},
		{"TR_concat([TR_Tensor([1])], axis: 0)", messages.H7900},
		{"TR_Tensor([1, 2, 3]).TR_split(2)", messages.H6318},
		{"TR_Tensor([1, 2]).TR_split(0)", messages.H6317},
	}
	for _, h := range errs {
		if _, err := run(t, h.code); err == nil || !messages.Has(err, h.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", h.code, err, h.want)
		}
	}
}

func TestKeepDim(t *testing.T) {
	mustRun(t, `
TR_var x = TR_Tensor([[[1, 2], [3, 4]], [[5, 6], [7, 8]]])
TR_assert_equal([2, 2, 1], TR_sum(x, TR_dim: -1, TR_keepdim: TR_true).TR_shape)
TR_assert_equal([2, 1, 2], TR_mean(x, TR_dim: 1, TR_keepdim: TR_true).TR_shape)
TR_assert_equal([[[4.0, 6.0]], [[12.0, 14.0]]], TR_sum(x, TR_dim: 1, TR_keepdim: TR_true).TR_tolist)
TR_assert_equal([[[2.0], [4.0]], [[6.0], [8.0]]], TR_max(x, TR_dim: -1, TR_keepdim: TR_true).TR_tolist)
TR_assert_equal([[[1.0, 2.0], [3.0, 4.0]]], TR_min(x, TR_dim: 0, TR_keepdim: TR_true).TR_tolist)
TR_assert_equal([2, 2], TR_sum(x, TR_dim: -1, TR_keepdim: TR_false).TR_shape)
TR_assert_equal([1, 1, 1], TR_sum(x, TR_keepdim: TR_true).TR_shape)
TR_assert_equal([1, 1, 1], TR_max(x, TR_keepdim: TR_true).TR_shape)
# Normalleştirme: yayınlamayla satır toplamına bölme, türev izler.
TR_var w = TR_Tensor([[1.0, 3.0]], TR_requires_grad: TR_true)
TR_var n = w / TR_sum(w, TR_dim: -1, TR_keepdim: TR_true)
TR_assert_equal([[0.25, 0.75]], n.TR_tolist)
TR_backward(TR_sum(n * TR_Tensor([1, 0])))
TR_assert(TR_abs(w.TR_grad.TR_tolist[0][0] - 0.1875) < 0.000001)
`)
	for _, h := range []struct {
		code string
		want messages.Code
	}{
		{"TR_sum([1, 2], TR_keepdim: TR_true)", messages.H6218},
		{"TR_sum(TR_Tensor([1, 2]), TR_keepdim: 1)", messages.H7905},
	} {
		if _, err := run(t, h.code); err == nil || !messages.Has(err, h.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", h.code, err, h.want)
		}
	}
}

// dikkat(...) bileşik sürümle (softmax(q·kᵀ/√dh + maske)·v) aynı sonucu ve gradyanı verir.
// Başvuru bakış önde ([B, H, T, dh]) kurulur; dikkat [B, T, H, dh] düzenini alır (bd çevirir).
func TestAttentionBuiltin(t *testing.T) {
	mustRun(t, `
TR_func bd(x: TR_Tensor) -> TR_Tensor { TR_return x.TR_permute([0, 2, 1, 3]) }
TR_var q = TR_Tensor(TR_randn([2, 4, 6, 8], TR_seed: 1), TR_requires_grad: TR_true)
TR_var k = TR_Tensor(TR_randn([2, 4, 6, 8], TR_seed: 2), TR_requires_grad: TR_true)
TR_var v = TR_Tensor(TR_randn([2, 4, 6, 8], TR_seed: 3), TR_requires_grad: TR_true)
TR_var satırlar = []
TR_for i TR_in 0..6 {
    TR_var s = []
    TR_for j TR_in 0..6 { TR_if j <= i { s.TR_append(0.0) } TR_else { s.TR_append(-1000000000.0) } }
    satırlar.TR_append(s)
}
TR_var ek = TR_Tensor(satırlar)
TR_func fark(a: TR_Tensor, b: TR_Tensor) -> TR_Float {
    TR_var d = a - b
    TR_return TR_max(d * d).TR_tolist
}
TR_var beklenen = TR_softmax((q @ k.TR_transpose()) * (1.0 / TR_sqrt(8)) + ek, TR_dim: -1) @ v
TR_backward(TR_sum(beklenen * beklenen))
TR_var gq = TR_Tensor(q.TR_grad)
TR_var gk = TR_Tensor(k.TR_grad)
TR_var gv = TR_Tensor(v.TR_grad)
TR_var o = bd(TR_attention(bd(TR_Tensor(q, TR_requires_grad: TR_true)), bd(TR_Tensor(k, TR_requires_grad: TR_true)), bd(TR_Tensor(v, TR_requires_grad: TR_true))))
TR_assert(fark(o, beklenen) < 0.0000000001, "ileri")
TR_var q2 = TR_Tensor(q, TR_requires_grad: TR_true)
TR_var k2 = TR_Tensor(k, TR_requires_grad: TR_true)
TR_var v2 = TR_Tensor(v, TR_requires_grad: TR_true)
TR_var o2 = bd(TR_attention(bd(q2), bd(k2), bd(v2)))
TR_backward(TR_sum(o2 * o2))
TR_assert(fark(q2.TR_grad, gq) < 0.0000000001, "dq")
TR_assert(fark(k2.TR_grad, gk) < 0.0000000001, "dk")
TR_assert(fark(v2.TR_grad, gv) < 0.0000000001, "dv")
`)
}

func TestAttentionOptions(t *testing.T) {
	out := mustRun(t, `
TR_func fark(a: TR_Tensor, b: TR_Tensor) -> TR_Float {
    TR_var d = a - b
    TR_return TR_max(d * d).TR_tolist
}
TR_var q = TR_randn([1, 5, 2, 4], TR_seed: 1)
TR_var k = TR_randn([1, 7, 1, 4], TR_seed: 2)
TR_var v = TR_randn([1, 7, 1, 3], TR_seed: 3)
TR_print(TR_attention(q, k, v).TR_shape)
TR_print(TR_attention(q, k, v, TR_causal: TR_false, TR_window: 2).TR_shape)
# Son sorgu (Tq < Tk: sondan hizalı) bütün anahtarları görür: nedensel ile nedensel olmayan aynı.
TR_var a = TR_attention(q, k, v)
TR_var b = TR_attention(q, k, v, TR_causal: TR_false)
TR_print(a[0][4][0].TR_tolist == b[0][4][0].TR_tolist, a[0][0][0].TR_tolist == b[0][0][0].TR_tolist)
# Tek anahtara bakan pencere: çıktı o anahtarın değeridir.
TR_var p = TR_attention(q, k, v, TR_window: 1)
TR_print(fark(p[0][4][1], v[0][6][0]) < 0.0000000001)
# Maske 0 olan anahtarlar görülmez.
TR_var m = TR_ones([5, 7])
m[0] = 0
m[0][2] = 1
TR_print(fark(TR_attention(q, k, v, TR_causal: TR_false, TR_mask: m)[0][0][0], v[0][2][0]) < 0.0000000001)
TR_print(TR_attention(q, k, v, TR_mask: TR_none).TR_shape)
`)
	want := "[1, 5, 2, 3]\n[1, 5, 2, 3]\ntrue false\ntrue\ntrue\n[1, 5, 2, 3]\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
	for _, h := range []struct {
		code string
		want messages.Code
	}{
		{"TR_attention(TR_Tensor([1.0]), TR_Tensor([1.0]), TR_Tensor([1.0]))", messages.H6327},
		{"TR_attention(TR_zeros([4, 3, 2]), TR_zeros([4, 2, 2]), TR_zeros([4, 2, 2]))", messages.H6333},
		{"TR_attention(TR_zeros([2, 4, 2]), TR_zeros([2, 4, 3]), TR_zeros([2, 4, 3]))", messages.H6330},
		{"TR_attention(TR_zeros([2, 4, 2]), TR_zeros([2, 4, 2]), TR_zeros([3, 4, 2]))", messages.H6331},
		{"TR_attention(TR_zeros([1, 2, 4, 2]), TR_zeros([2, 2, 4, 2]), TR_zeros([2, 2, 4, 2]))", messages.H6329},
		{"TR_attention(TR_zeros([4, 2]), TR_zeros([4, 2]), TR_zeros([4, 2]), TR_mask: TR_ones([3, 4]))", messages.H6335},
		{"TR_attention(TR_zeros([4, 2]), TR_zeros([4, 2]), TR_zeros([4, 2]), TR_window: -1)", messages.H6326},
		{"TR_attention(TR_zeros([4, 2]), TR_zeros([4, 2]), TR_zeros([4, 2]), TR_scale: 2)", messages.H7901},
		{"TR_attention(TR_zeros([4, 2]), TR_zeros([4, 2]))", messages.H6321},
		{"TR_attention([[1.0]], TR_zeros([1, 1]), TR_zeros([1, 1]))", messages.H6324},
	} {
		if _, err := run(t, h.code); err == nil || !messages.Has(err, h.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", h.code, err, h.want)
		}
	}
}

// Karışık hassasiyet bloğunda dikkat girdileri bf16'ya çevirir ve bf16 sonuç verir: blok
// dışında bf16 girdilerle yapılan dikkatle aynıdır.
func TestAttentionMixedPrecision(t *testing.T) {
	out := mustRun(t, `
TR_func fark(a: TR_Tensor, b: TR_Tensor) -> TR_Float {
    TR_var d = a - b
    TR_return TR_max(d * d).TR_tolist
}
TR_var q = TR_randn([2, 6, 8], TR_seed: 1)
TR_var a = TR_mixed_precision("bf16") { TR_attention(q, q, q) }
TR_var b = TR_attention(q.TR_to_dtype("bf16"), q.TR_to_dtype("bf16"), q.TR_to_dtype("bf16"))
TR_print(a.TR_dtype, fark(a.TR_to_dtype("float32"), b.TR_to_dtype("float32")) < 0.0000000001)
TR_print(TR_attention(q.TR_to_dtype("bf16"), q.TR_to_dtype("bf16"), q.TR_to_dtype("bf16")).TR_dtype)
`)
	if out != "bf16 true\nbf16\n" {
		t.Fatalf("çıktı: %q", out)
	}
}

func TestSample(t *testing.T) {
	out := mustRun(t, `
TR_var l = TR_Tensor([[0.0, 3.0, 1.0], [2.0, 0.0, 0.5]])
TR_var e = TR_sample(l, TR_temperature: 0)
TR_print(e, e.TR_tolist, e.TR_shape, TR_Type(e.TR_tolist[0]))
TR_print(TR_sample(TR_Tensor([1.0, 5.0, 2.0]), TR_temperature: 0).TR_tolist)
TR_print(TR_sample(l, TR_seed: 7).TR_tolist == TR_sample(l, TR_seed: 7).TR_tolist)
# en_iyi_k: 1 her zaman en olasıyı verir.
TR_print(TR_sample(l, TR_top_k: 1).TR_tolist)
# Tohumsuz çağrılar farklı sonuçlar verir (64 bağımsız örnek).
TR_var düz = TR_ones([64, 50])
TR_print(TR_sample(düz).TR_tolist != TR_sample(düz).TR_tolist)
# en_iyi_k: 2 dışında kalan sınıf hiç seçilmez.
TR_var sayılar = [0, 0, 0]
TR_for x TR_in TR_sample(TR_Tensor([[1.0, 1.1, 1.2]]) + TR_ones([2000, 3]), TR_top_k: 2, TR_seed: 1).TR_tolist { sayılar[x] += 1 }
TR_print(sayılar[0], sayılar[1] > 800, sayılar[2] > 800)
`)
	lines := strings.Split(out, "\n")
	want := []string{"Tensor[2] [1, 0] [1, 0] [2] Int", "1", "true", "[1, 0]", "true", "0 true true"}
	for i, w := range want {
		if i >= len(lines) || lines[i] != w {
			t.Fatalf("satır %d: %q, beklenen %q\n%s", i+1, lines[min(i, len(lines)-1)], w, out)
		}
	}
	// Dağılım: sıcaklık 1'de olasılıklar softmax'a, yüksek sıcaklıkta düzgüne yakın.
	v := value(t, `
TR_var l = TR_Tensor([0.0, 1.0, 2.0])
TR_var satır = TR_ones([30000, 3]) * l
TR_var ö = TR_sample(satır, TR_seed: 5).TR_tolist
TR_var s = [0, 0, 0]
TR_for x TR_in ö { s[x] += 1 }
TR_var cikti = s`)
	s := v.(*interp.ListVal).Elements
	top := math.Exp(0) + math.Exp(1) + math.Exp(2)
	for i, e := range s {
		got := float64(e.(*interp.IntVal).Val) / 30000
		want := math.Exp(float64(i)) / top
		if math.Abs(got-want) > 0.015 {
			t.Fatalf("sınıf %d: oran %.4f, beklenen %.4f", i, got, want)
		}
	}
	expectCodes(t, []codeCase{
		{"TR_sample(TR_Tensor([1.0]), TR_temperature: -1)", messages.H6340},
		{"TR_sample(TR_Tensor([1.0]), TR_top_k: -2)", messages.H6341},
		{"TR_sample(TR_Tensor(1.0))", messages.H6338},
		{"TR_sample([1.0, 2.0])", messages.H6337},
		{"TR_sample(TR_Tensor([1.0]), TR_seed: 1.5)", messages.H6342},
		{"TR_sample(TR_Tensor([1.0]), temperature: 1)", messages.H7901},
	})
}

func TestParameters(t *testing.T) {
	fpath := filepath.ToSlash(filepath.Join(t.TempDir(), "model.trt"))
	out := mustRun(t, `
TR_func p(x: TR_Float) -> TR_Tensor { TR_return TR_Tensor([x], TR_requires_grad: TR_true) }
TR_struct Dikkat { Wq: TR_Tensor, Wk: TR_Tensor, ölçek: TR_Float }
TR_struct Blok { TR_attention: Dikkat, mlp: TR_List[TR_Tensor], yan: TR_Dict[TR_String, TR_Tensor] }
TR_struct Model { TR_embedding: TR_Tensor, TR_blocks: TR_List[Blok], TR_last: TR_Tensor, ad: TR_String, ek: TR_Tensor? = TR_none }
TR_var ortak = p(9)
TR_var m = Model(p(1), [Blok(Dikkat(p(2), p(3), 0.5), [p(4), ortak], {}), Blok(Dikkat(p(5), ortak, 1), [], {"W1": p(6), "sabit": TR_Tensor([0.0])})], ortak, "küçük")
m.ek = p(7)
TR_var liste = TR_parameters(m)
TR_print(liste.TR_length, [x.TR_tolist[0] TR_for x TR_in liste])
TR_var adlar = TR_parameters(m, TR_named: TR_true)
TR_print(adlar.TR_keys())
# Aynı tensör nesnesidir: iyileştirici ile güncellenir.
TR_assert(liste[0] == m.TR_embedding)
TR_save_tensors("`+fpath+`", adlar)
TR_var y = TR_load_tensors("`+fpath+`")
TR_print(y.TR_keys() == adlar.TR_keys(), y["TR_mem_blocks.1.TR_mem_attention.Wq"].TR_tolist)
# Ara sonuçlar ve türev izlemeyenler parametre değildir.
TR_print(TR_parameters([p(1) * 2, TR_Tensor([1.0])]).TR_length, TR_parameters(p(1)).TR_length, TR_parameters(p(1), TR_named: TR_true).TR_keys())
# Döngülü yapılar sonsuz döngüye girmez.
TR_var l = [p(1)]
l.TR_append(l)
TR_print(TR_parameters(l).TR_length)
`)
	lines := strings.Split(out, "\n")
	expected := []string{
		"8 [1.0, 2.0, 3.0, 4.0, 9.0, 5.0, 6.0, 7.0]",
		// Ağırlık adı kimlik yoludur: kitaplığın üye adlarıyla aynı alanlar (gömme, bloklar, dikkat) kodludur.
		`["TR_mem_embedding", "TR_mem_blocks.0.TR_mem_attention.Wq", "TR_mem_blocks.0.TR_mem_attention.Wk", "TR_mem_blocks.0.mlp.0", "TR_mem_blocks.0.mlp.1", "TR_mem_blocks.1.TR_mem_attention.Wq", "TR_mem_blocks.1.yan.W1", "ek"]`,
		"true [5.0]",
		`0 1 ["0"]`,
		"1",
	}
	for i, w := range expected {
		if i >= len(lines) || lines[i] != w {
			t.Fatalf("satır %d:\n  %q\nbeklenen:\n  %q\ntüm çıktı:\n%s", i+1, lines[min(i, len(lines)-1)], w, out)
		}
	}
	if _, err := run(t, "TR_parameters([], adlariyla: TR_true, TR_x: 1)"); err == nil {
		t.Fatal("bilinmeyen adlı argüman hata vermeliydi")
	}
	if _, err := run(t, `TR_parameters({"a.b": TR_Tensor([1.0], TR_requires_grad: TR_true), "a": {"b": TR_Tensor([1.0], TR_requires_grad: TR_true)}}, TR_named: TR_true)`); err == nil || !messages.Has(err, messages.H6505) {
		t.Fatalf("çakışan ad hata vermeliydi: %v", err)
	}
}

// AlanlariGez tanım sırasını, ardından sonradan eklenen alanları ad sırasıyla verir.
func TestFieldWalkOrder(t *testing.T) {
	v := value(t, `
TR_struct A { z: TR_Int, a: TR_Int, b: TR_Int? = TR_none, y: TR_Int? = TR_none }
TR_var x = A(1, 2)
x.y = 3
x.b = 4
TR_var cikti = x`)
	var names []string
	v.(*interp.StructInstanceVal).EachField(func(ad string, _ interp.Value) { names = append(names, ad) })
	if strings.Join(names, ",") != "z,a,b,y" {
		t.Fatalf("sıra: %v", names)
	}
}

// örnekle tanımsız dağılımda (NaN logit, hepsi -∞, +∞) sessizce son adayı seçmez, hata verir.
func TestSampleUndefinedDistribution(t *testing.T) {
	for _, logits := range []string{"TR_Tensor([nan, 1.0, 2.0])", "TR_Tensor([-1e40, -1e40])", "TR_Tensor([1e40, 1.0])"} {
		_, err := run(t, "TR_var nan = TR_log(TR_Tensor([-1.0])).TR_tolist[0]\nTR_sample("+logits+", TR_seed: 1)")
		if err == nil || !messages.Has(err, messages.H6343) {
			t.Fatalf("%s: hata beklenirdi: %v", logits, err)
		}
	}
	mustRun(t, "TR_sample(TR_Tensor([-1e40, 0.0]), TR_seed: 1)") // bir -∞ aday sorun değil
}

package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Kural (15.6d): yerinde yazmanın (atama, bileşik atama, çoklu atama, TR_write_slice) hedef
// zincirinde kopya veren adım denetim hatasıdır. Tensörle indeks H6740'tır, öneri ad vermeyi ve
// satırları geri yazmayı söyler; öbür kopya adımı H6741'dir, öneri ad vermeyi söyler. Maske de
// tensörle indekstir (denetim cinsi bilmez, K6). Ad, alan, kap öğesi, tam sayı ve aralık indeksi ve
// görünüm hata değildir; en dıştaki indeks yazmanın kendisidir.
func TestCopyWriteTargets(t *testing.T) {
	const pre = "TR_var t = TR_Tensor([1.0, 2.0, 3.0, 4.0])\nTR_var idx = TR_Tensor([1, 2])\n"
	const al = "TR_func al(x: TR_Tensor) -> TR_Tensor { TR_return x }\n"
	const back = "TR_for k, i TR_in idx.TR_reshape([-1]) { t[i] = c.TR_reshape([-1] + t[0].TR_shape)[k] }"
	for _, c := range []struct {
		source string
		code   messages.Code
		line   int
		want   string
	}{
		{pre + "t[idx][0] = 9.0", messages.H6740, 3, "`TR_var c = t[idx]; c[0] = 9.0; " + back + "`"},
		{pre + "t[idx][0] += 5.0", messages.H6740, 3, "`TR_var c = t[idx]; c[0] += 5.0; " + back + "`"},
		{pre + "TR_var b = 0\nt[idx][0], b = 9.0, 1", messages.H6740, 4, "`TR_var c = t[idx]; c[0], b = 9.0, 1`"},
		{pre + "TR_var m = t > 2.0\nt[m][0] = 9.0", messages.H6740, 4, "`TR_var c = t[m]; c[0] = 9.0`"},
		{"TR_var m = TR_zeros([3, 2])\nTR_var idx = TR_Tensor([1, 2])\nTR_write_slice(m[idx], TR_ones([2, 1]), TR_dim: 1, TR_start: 0)",
			messages.H6740, 3, "`TR_var c = m[idx]; TR_write_slice(c, TR_ones([2, 1]), TR_dim: 1, TR_start: 0)`"},
		{pre + "(t * 2.0)[0] = 9.0", messages.H6741, 3, "the result of an operation"},
		{pre + "(t * 2.0)[0] = 9.0", messages.H6741, 3, "`TR_var c = t * 2.0; c[0] = 9.0`"},
		{pre + "TR_Tensor(t)[0] = 9.0", messages.H6741, 3, "`TR_var c = TR_Tensor(t); c[0] = 9.0`"},
		{pre + "t.TR_to_dtype(\"float64\")[0] = 9.0", messages.H6741, 3, "a method that is not a view"},
		{pre + "t.TR_reshape([2, 2])[0][0] = 9.0", messages.H6741, 3, "`TR_var c = t.TR_reshape([2, 2]); c[0][0] = 9.0`"},
		{pre + "t.TR_to_device(\"cpu\")[0] = 9.0", messages.H6741, 3, "a method that is not a view"},
		{pre + al + "al(t)[0] = 9.0", messages.H6741, 4, "`TR_var c = al(t); c[0] = 9.0`"},
		{pre + al + "al(t)[idx][0] = 9.0", messages.H6741, 4, "`TR_var c = al(t)[idx]; c[0] = 9.0`"},
		{pre + "TR_var c = 1\nt[idx][0] = 9.0", messages.H6740, 4, "`TR_var c2 = t[idx]; c2[0] = 9.0`"},
	} {
		fileErrorWith(t, c.source, c.code, c.line, c.want)
	}
	for _, source := range []string{
		pre + "t[1] = 9.0",
		pre + "t[0..2][1] = 8.0",
		pre + "t.TR_slice(0, 0, 2)[1] = 6.0",
		pre + "t.TR_permute([0])[1] = 3.0",
		pre + "TR_var c = t[idx]\nc[0] = 9.0",
		pre + "TR_var l = [t]\nl[0][1] = 2.0",
		pre + "TR_var d = {\"w\": t}\nd[\"w\"][1] = 5.0",
		pre + "t.TR_slice(0, 0, 2)[idx] = 9.0", // en dıştaki indeks yazmanın kendisidir (H7042 çalışırken)
		"TR_var m = TR_zeros([2, 2])\nm.TR_transpose()[0][1] = 1.0",
		"TR_var m = TR_zeros([3, 2])\nTR_write_slice(m[1], TR_ones([1]), TR_dim: 0, TR_start: 0)",
	} {
		p := parser.New(lexer.NewBody(source, true))
		prog := p.ParseProgram()
		ch := New()
		ch.SetFile(ProgramFile, source)
		ch.CheckProgram(prog)
		if errs := ch.Errs(); hasCode(errs, messages.H6740) || hasCode(errs, messages.H6741) {
			t.Errorf("kopya adımı yok, gelen %v\nkaynak:\n%s", errs, source)
		}
	}
}

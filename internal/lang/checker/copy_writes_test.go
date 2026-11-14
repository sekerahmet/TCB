package checker

import (
	"strings"
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

// containerPre kap kuralının testlerinin ortak başıdır (9 satır).
const containerPre = "TR_struct Nokta {\n    x: TR_Int = 0\n    TR_func liste() -> TR_List[TR_Int] { TR_return [7, 8] }\n}\n" +
	"TR_var l = [1, 2, 3, 4]\nTR_var d = {\"a\": [1]}\nTR_var ll = [[1], [2]]\nTR_var p = Nokta()\n" +
	"TR_func f() -> TR_List[TR_Int] { TR_return l }\n"

// noContainerError kaynak kap kuralının hatasını vermez.
func noContainerError(t *testing.T, source string) {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	ch := New()
	ch.SetFile(ProgramFile, source)
	ch.CheckProgram(prog)
	if errs := ch.Errs(); hasCode(errs, messages.H2740) || hasCode(errs, messages.H2741) || hasCode(errs, messages.H2743) || len(errs) > 0 {
		t.Errorf("hata beklenmiyordu, gelen %v\nkaynak:\n%s", errs, source)
	}
}

// Kural (5.4, KA2): liste, sözlük ya da yapı hedefinin (atama, bileşik ve çoklu atama; indeks ve alan)
// zincirinde Copies() adımı ya da liste dilimi denetim hatasıdır. Kabın öğesini veren üye H2741
// (kabın indeksi), tam sayı tipli indeksin altındaki liste dilimi H2743 (kaynağın indeksi ve ad),
// negatif sabit indeksli dilim ve öbür adımlar H2740'tır (ad). Ad, alan ve kap öğesinden geçen yazma
// hatasızdır; tensör hedefi 15.6d'de kalır (TestCopyWriteTargets).
func TestContainerWriteChainIsChecked(t *testing.T) {
	for _, c := range []struct {
		body string
		code messages.Code
		want string
	}{
		{"f()[0] = 5", messages.H2740, "`f()` is the result of a call"},
		{"f()[0] = 5", messages.H2740, "`TR_var c = f(); c[0] = 5`"},
		{"[1, 2][0] = 5", messages.H2740, "`[1, 2]` is a newly built value"},
		{"(l + [5])[0] = 1", messages.H2740, "`TR_var c = l + [5]; c[0] = 1`"},
		{"p.liste()[0] = 5", messages.H2740, "the result of a method"},
		{"[x TR_for x TR_in l][0] = 9", messages.H2740, "a newly built value"},
		{"TR_var j = TR_json_parse(\"[1]\")\nTR_List(j)[0] = j[0]", messages.H2740, "`TR_var c = TR_List(j); c[0] = j[0]`"},
		{"l[0..2][-1] = 9", messages.H2740, "`l[0..2]` is a slice:"},
		{"TR_var r = 0..2\nl[r][0] = 9", messages.H2740, "`l[r]` is a slice:"},
		{"l[0..2][0] = 5", messages.H2743, "`l[0] = 5` (only while `0` is not negative"},
		{"TR_var i = 1\nl[0..2][i] = 9", messages.H2743, "`l[0 + i] = 9` (only while `i` is not negative"},
		{"TR_var i = 1\nl[0..2][i] = 9", messages.H2743, "`TR_var c = l[0..2]; c[i] = 9`"},
		{"TR_var a = 1\nl[a..3][0] += 5", messages.H2743, "`l[a + 0] += 5`"},
		{"TR_var b = 0\nl[1..3][1], b = 7, 1", messages.H2743, "`l[2], b = 7, 1`"},
		{"d.TR_get(\"a\")[0] = 5", messages.H2741, "`d[\"a\"][0] = 5`"},
		{"ll.TR_first[0] = 9", messages.H2741, "`ll[0][0] = 9`"},
		{"ll.TR_last[0] = 9", messages.H2741, "`ll[-1][0] = 9`"},
		{"TR_func np() -> Nokta { TR_return p }\nnp().x = 9", messages.H2740, "`TR_var c = np(); c.x = 9`"},
		{"Nokta(1).x = 3", messages.H2740, "`TR_var c = Nokta(1); c.x = 3`"},
		{"TR_var c = 1\nf()[0] = 5", messages.H2740, "`TR_var c2 = f(); c2[0] = 5`"},
	} {
		source := containerPre + c.body
		fileErrorWith(t, source, c.code, strings.Count(source, "\n")+1, c.want)
	}
	for _, body := range []string{
		"l[0] = 5", "d[\"a\"][0] = 5", "ll[0][0] = 9", "p.x = 4", "TR_var c = f()\nc[0] = 5",
		"TR_var ps = [Nokta()]\nps[0].x = 5", "TR_var dn = {\"p\": Nokta()}\ndn[\"p\"].x = 2", "l[-1] += 1",
		"TR_var t = TR_Tensor([1.0, 2.0])\nt[0..2][1] = 8.0",
	} {
		noContainerError(t, containerPre+body)
	}
}

// Kural (5.4, KA3): "in_place" işaretli metodun (.TR_append) alıcısı liste ya da sözlükse alıcının
// zinciri atamanın hedefi gibi denetlenir. Başka alıcıda (TR_Logger) metot yazma değildir; değer
// veren yerinde değiştirici (.TR_pop()) ve adlı alıcı serbesttir.
func TestInPlaceMethodChainIsChecked(t *testing.T) {
	for _, c := range []struct {
		body string
		code messages.Code
		want string
	}{
		{"f().TR_append(3)", messages.H2740, "`TR_var c = f(); c.TR_append(3)`"},
		{"d.TR_get(\"a\").TR_append(3)", messages.H2741, "`d[\"a\"].TR_append(3)`"},
		{"l[0..2].TR_append(3)", messages.H2740, "`l[0..2]` is a slice:"},
		{"p.liste().TR_append(3)", messages.H2740, "the result of a method"},
	} {
		source := containerPre + c.body
		fileErrorWith(t, source, c.code, strings.Count(source, "\n")+1, c.want)
	}
	for _, body := range []string{
		"l.TR_append(3)", "ll[0].TR_append(1)", "TR_print(f().TR_pop())", "TR_Logger(\"k.jsonl\").TR_append(\"s\")",
	} {
		noContainerError(t, containerPre+body)
	}
}

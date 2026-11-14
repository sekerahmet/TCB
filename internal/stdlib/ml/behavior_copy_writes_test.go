package ml

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// runWithSource kodu denetimsiz çalıştırır; VM kaynağı bilir (iletinin önerisi dosyanın yazımıyla).
func runWithSource(t *testing.T, code string) (string, error) {
	t.Helper()
	m := interp.New()
	var out bytes.Buffer
	m.Out, m.Err = &out, &out
	m.SourceCode = code
	_, err := m.Run(trrun.Parse(t, code))
	m.WaitAsync()
	return out.String(), err
}

const copyPre = "TR_var t = TR_Tensor([1.0, 2.0, 3.0, 4.0])\nTR_var idx = TR_Tensor([1, 2])\n" +
	"TR_func yan() -> TR_Float { TR_print(\"sağ yan\"); TR_return 9.0 }\n"

// Kural (15.6d): tip denetiminin görmediği programda da hedef zincirindeki kopya adımı çalışma
// hatasıdır, denetimle aynı kodla (H6740, H6741). Hedefin yeri sağ yandan önce hesaplanır (5.4): hata
// sağ yan değerlendirilmeden çıkar, t değişmez. Görünüm, kap öğesi ve adlı kopya geçerlidir.
func TestCopyWriteTargetsAtRunTime(t *testing.T) {
	const al = "TR_func al(x: TR_Tensor) -> TR_Tensor { TR_return x }\n"
	for _, c := range []struct {
		code string
		want messages.Code
	}{
		{"t[idx][0] = yan()", messages.H6740},
		{"t[idx][0] += yan()", messages.H6740},
		{"TR_var b = 0\nt[idx][0], b = yan(), 1", messages.H6740},
		{"TR_var x = [t]\nx[0][idx][0] = yan()", messages.H6740},
		{"(t * 2.0)[0] = yan()", messages.H6741},
		{"TR_Tensor(t)[0] = yan()", messages.H6741},
		{"t.TR_to_dtype(\"float64\")[0] = yan()", messages.H6741},
		{"t.TR_reshape([2, 2])[0][0] = yan()", messages.H6741},
		{"t.TR_to_device(\"cpu\")[0] = yan()", messages.H6741},
		{al + "al(t)[0] = yan()", messages.H6741},
		{al + "al(t)[idx][0] = yan()", messages.H6741},
		{"TR_var m = TR_zeros([3, 2])\nTR_write_slice(m[idx], TR_ones([2, 1]), TR_dim: 1, TR_start: 0)\nTR_print(m.TR_tolist)", messages.H6740},
	} {
		code := copyPre + c.code + "\nTR_print(t.TR_tolist)"
		out, err := runWithSource(t, code)
		if !messages.Has(err, c.want) {
			t.Errorf("%s:\n  hata %v, %s bekleniyordu", c.code, err, c.want)
			continue
		}
		if out != "" {
			t.Errorf("%s: hata sağ yandan ve yazmadan önce çıkmalı, çıktı %q", c.code, out)
		}
		if text := messages.ErrorIn(err, "en"); !strings.Contains(text, "`TR_var c = ") {
			t.Errorf("%s: öneri dosyanın yazımıyla kurulmalı: %s", c.code, text)
		}
	}
	for _, c := range []struct{ code, want string }{
		{"t[1..3][0] = 8.0", "[1.0, 8.0, 3.0, 4.0]"},
		{"t.TR_slice(0, 0, 2)[1] += 4.0", "[1.0, 6.0, 3.0, 4.0]"},
		{"TR_var c = t[idx]\nc[0] = 9.0", "[1.0, 2.0, 3.0, 4.0]"},
		{"TR_var x = [t]\nx[0][3] = 0.0", "[1.0, 2.0, 3.0, 0.0]"},
		{"TR_var i = 2\nTR_var x = [t]\nx[0][i] = 0.0", "[1.0, 2.0, 0.0, 4.0]"},
		{"TR_var d = {\"w\": t}\nd[\"w\"][0] = 5.0", "[5.0, 2.0, 3.0, 4.0]"},
	} {
		out, err := runWithSource(t, copyPre+c.code+"\nTR_print(t.TR_tolist)")
		if err != nil || out != c.want+"\n" {
			t.Errorf("%s: %q %v, beklenen %s", c.code, out, err, c.want)
		}
	}
}

// Kural (15.6d, K4, K5): iletinin iki önerisi kopyalanınca çalışır. Ad verme yazımın anlamını korur
// (kopya kopyada kalır, görünüm asıl tensöre yazar); H6740'ın geri yazması, yinelenmeyen idx'te
// tensörle indeksin satırlarını asıl tensöre yazar.
func TestCopyWriteSuggestionsRun(t *testing.T) {
	const pre = "TR_var t = TR_Tensor([1.0, 2.0, 3.0, 4.0])\nTR_var idx = TR_Tensor([1, 2])\n" +
		"TR_var m = TR_zeros([3, 2])\nTR_var u = TR_Tensor([10, 20, 30])\nTR_var ui = TR_Tensor([2, 0])\n"
	for _, c := range []struct {
		wrong, show, named, back string // yanlış yazım, yazdırılan, ad vermenin ve geri yazmanın sonucu
	}{
		{"t[idx][0] = 9.0", "t", "[1.0, 2.0, 3.0, 4.0]", "[1.0, 9.0, 3.0, 4.0]"},
		{"t[idx][0] += 5.0", "t", "[1.0, 2.0, 3.0, 4.0]", "[1.0, 7.0, 3.0, 4.0]"},
		{"m[idx][1][0] = 7.0", "m", "[[0.0, 0.0], [0.0, 0.0], [0.0, 0.0]]", "[[0.0, 0.0], [0.0, 0.0], [7.0, 0.0]]"},
		{"TR_write_slice(m[idx], TR_ones([2, 1]), TR_dim: 1, TR_start: 0)", "m", "[[0.0, 0.0], [0.0, 0.0], [0.0, 0.0]]", "[[0.0, 0.0], [1.0, 0.0], [1.0, 0.0]]"},
		{"u[ui][1] += 5", "u", "[10, 20, 30]", "[15, 20, 30]"},
		{"(t * 2.0)[0] = 9.0", "t", "[1.0, 2.0, 3.0, 4.0]", ""},
		{"TR_Tensor(t)[0] = 9.0", "t", "[1.0, 2.0, 3.0, 4.0]", ""},
		{"t.TR_reshape([2, 2])[0][0] = 9.0", "t", "[9.0, 2.0, 3.0, 4.0]", ""}, // görünüm: asıl tensöre yazar, eski yazım gibi
	} {
		src := pre + c.wrong
		p := parser.New(lexer.NewBody(src, true))
		prog := p.ParseProgram()
		ch := checker.New()
		ch.SetFile(checker.ProgramFile, src)
		ch.CheckProgram(prog)
		errs := ch.Errs()
		if len(errs) != 1 {
			t.Errorf("%s: tek hata bekleniyordu, gelen %v", c.wrong, errs)
			continue
		}
		var spellings []string
		for i, part := range strings.Split(messages.ErrorIn(errs[0], "en"), "`") {
			if i%2 == 1 && strings.HasPrefix(part, "TR_var ") {
				spellings = append(spellings, part)
			}
		}
		wants := []string{c.named}
		if c.back != "" {
			wants = append(wants, c.back)
		}
		if len(spellings) != len(wants) {
			t.Errorf("%s: %d öneri, beklenen %d: %v", c.wrong, len(spellings), len(wants), spellings)
			continue
		}
		for i, s := range spellings {
			got := mustRun(t, pre+s+"\nTR_print("+c.show+".TR_tolist)")
			if got != wants[i]+"\n" {
				t.Errorf("%s: öneri %q %q verdi, beklenen %s", c.wrong, s, got, wants[i])
			}
		}
	}
}

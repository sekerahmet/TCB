package interp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// runSourced kodu denetimsiz çalıştırır; VM kaynağı bilir (iletinin önerisi dosyanın yazımıyla).
func runSourced(t *testing.T, code string) (string, error) {
	t.Helper()
	m := New()
	var out bytes.Buffer
	m.Out, m.Err = &out, &out
	m.SourceCode = code
	_, err := m.Run(mustParseProgram(t, code))
	m.WaitAsync()
	return out.String(), err
}

const containerPre = "TR_struct Nokta {\n    x: TR_Int = 0\n    TR_func liste() -> TR_List[TR_Int] { TR_return [7, 8] }\n}\n" +
	"TR_var l = [1, 2, 3, 4]\nTR_var m = [5]\nTR_var d = {\"a\": [1]}\nTR_var ll = [[1], [2]]\nTR_var p = Nokta()\nTR_var i = 1\n" +
	"TR_func f() -> TR_List[TR_Int] { TR_return l }\nTR_func np() -> Nokta { TR_return p }\n" +
	"TR_func yan() -> TR_Int { TR_print(\"sağ yan\"); TR_return 9 }\n"

// Kural (5.4, KA4): denetimin görmediği programda da liste, sözlük ya da yapı hedefinin zincirindeki
// kopya adımı çalışma hatasıdır, denetimle aynı kodla; indeks, alan ve yerinde değiştirici metot
// hedefinde. Hedefin yeri sağ yandan ve argümanlardan önce hesaplanır: hata sağ yan değerlendirilmeden
// çıkar. Ad, alan ve kap öğesinden geçen yazma hatasızdır. TR_List(j) ve TR_Logger'ın .TR_append'i
// kitaplık yerleşiğidir; onlar TR testindedir (tests/yazma_zinciri_test.tr, "5.4 hedefin zinciri").
func TestContainerWriteChainAtRunTime(t *testing.T) {
	for _, c := range []struct {
		code string
		want messages.Code
	}{
		{"f()[0] = yan()", messages.H2740},
		{"[1, 2][0] = yan()", messages.H2740},
		{"(l + m)[0] = yan()", messages.H2740},
		{"p.liste()[0] = yan()", messages.H2740},
		{"[x TR_for x TR_in l][0] = yan()", messages.H2740},
		{"l[0..2][-1] = yan()", messages.H2740},
		{"TR_var r = 0..2\nl[r][0] = yan()", messages.H2740},
		{"l[0..2][i] = yan()", messages.H2743},
		{"l[1..3][0] += yan()", messages.H2743},
		{"TR_var b = 0\nl[0..2][0], b = yan(), 1", messages.H2743},
		{"d.TR_get(\"a\")[0] = yan()", messages.H2741},
		{"ll.TR_first[0] = yan()", messages.H2741},
		{"np().x = yan()", messages.H2740},
		{"Nokta(1).x = yan()", messages.H2740},
		{"f().TR_append(yan())", messages.H2740},
		{"l[0..2].TR_append(yan())", messages.H2740},
		{"d.TR_get(\"a\").TR_append(yan())", messages.H2741},
	} {
		out, err := runSourced(t, containerPre+c.code+"\nTR_print(l)")
		if !messages.Has(err, c.want) {
			t.Errorf("%s:\n  hata %v, %s bekleniyordu", c.code, err, c.want)
			continue
		}
		if out != "" {
			t.Errorf("%s: hata sağ yandan ve yazmadan önce çıkmalı, çıktı %q", c.code, out)
		}
		if text := messages.ErrorIn(err, "en"); strings.Contains(text, "...") {
			t.Errorf("%s: öneri dosyanın yazımıyla kurulmalı: %s", c.code, text)
		}
	}
	for _, c := range []struct{ code, want string }{
		{"l[0] = 5\nTR_print(l)", "[5, 2, 3, 4]"},
		{"d[\"a\"][0] = 5\nTR_print(d)", "{\"a\": [5]}"},
		{"ll[0][0] = 9\nTR_print(ll)", "[[9], [2]]"},
		{"p.x = 4\nTR_print(p.x)", "4"},
		{"l.TR_append(3)\nTR_print(l)", "[1, 2, 3, 4, 3]"},
		{"TR_var c = f()\nc[0] = 5\nTR_print(l)", "[5, 2, 3, 4]"},
		{"TR_print(f().TR_pop(), l)", "4 [1, 2, 3]"},
		{"TR_var ps = [Nokta()]\nps[0].x = 5\nTR_print(ps[0].x)", "5"},
	} {
		out, err := runSourced(t, containerPre+c.code)
		if err != nil || out != c.want+"\n" {
			t.Errorf("%s: %q %v, beklenen %s", c.code, out, err, c.want)
		}
	}
}

// Kural (5.4, KA5): iletinin önerileri kopyalanınca çalışır ve beklenen değeri verir; denetim ve
// çalışma anı aynı metni verir. Ad verme yazımın anlamını korur (paylaşılan değerde kaynak değişir,
// kopyada değişmez); H2741 kabın indeksiyle, H2743 kaynağın indeksiyle asıl kaba yazar.
func TestContainerWriteSuggestionsRun(t *testing.T) {
	for _, c := range []struct {
		wrong, show, named, other string // yanlış yazım, yazdırılan, ad vermenin ve öbür önerinin sonucu
	}{
		{"f()[0] = 5", "l", "[5, 2, 3, 4]", ""},
		{"[1, 2][0] = 5", "l", "[1, 2, 3, 4]", ""},
		{"(l + m)[0] = 9", "l", "[1, 2, 3, 4]", ""},
		{"p.liste()[0] = 5", "l", "[1, 2, 3, 4]", ""},
		{"l[0..2][-1] = 9", "l", "[1, 2, 3, 4]", ""},
		{"l[0..2][i] = 9", "l", "[1, 2, 3, 4]", "[1, 9, 3, 4]"},
		{"l[1..3][0] += 5", "l", "[1, 2, 3, 4]", "[1, 7, 3, 4]"},
		{"l[1..3][i] = 0", "l", "[1, 2, 3, 4]", "[1, 2, 0, 4]"},
		{"d.TR_get(\"a\")[0] = 5", "d", "", "{\"a\": [5]}"},
		{"ll.TR_first[0] = 9", "ll", "", "[[9], [2]]"},
		{"ll.TR_last[0] = 9", "ll", "", "[[1], [9]]"},
		{"np().x = 9", "p.x", "9", ""},
		{"Nokta(1).x = 3", "p.x", "0", ""},
		{"f().TR_append(3)", "l", "[1, 2, 3, 4, 3]", ""},
		{"d.TR_get(\"a\").TR_append(3)", "d", "", "{\"a\": [1, 3]}"},
	} {
		src := containerPre + c.wrong
		ch := checker.New()
		ch.SetFile(checker.ProgramFile, src)
		ch.CheckProgram(mustParseProgram(t, src))
		errs := ch.Errs()
		if len(errs) != 1 {
			t.Errorf("%s: tek hata bekleniyordu, gelen %v", c.wrong, errs)
			continue
		}
		text := messages.ErrorIn(errs[0], "en")
		if _, err := runSourced(t, src); messages.ErrorIn(err, "en") != text {
			t.Errorf("%s: çalışma anının metni denetiminkiyle aynı olmalı:\n  %v\n  %s", c.wrong, err, text)
		}
		var named, other []string
		for k, part := range strings.Split(text, "`") {
			switch {
			case k%2 == 0 || k == 1:
			case strings.HasPrefix(part, "TR_var "):
				named = append(named, part)
			case strings.Contains(part, "=") || strings.Contains(part, ".TR_append("):
				other = append(other, part)
			}
		}
		for _, s := range []struct {
			got  []string
			want string
		}{{named, c.named}, {other, c.other}} {
			if (len(s.got) == 1) != (s.want != "") || len(s.got) > 1 {
				t.Errorf("%s: öneriler %v, beklenen sonuç %q: %s", c.wrong, s.got, s.want, text)
				continue
			}
			if s.want == "" {
				continue
			}
			code := containerPre + s.got[0] + "\nTR_print(" + c.show + ")" // öneri olduğu gibi kopyalanır
			if out, err := runSourced(t, code); err != nil || out != s.want+"\n" {
				t.Errorf("%s: öneri %q: %q %v, beklenen %s", c.wrong, s.got[0], out, err, s.want)
			}
		}
	}
}

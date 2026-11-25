package lexer

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (13.2): dört biçim UAX #15'in verdiğidir. Başvuru Python'un unicodedata'sıdır
// (testdata/generate_normalize.py): x/text'in kırpılan anahtarının çiftleri, 31 ve daha çok birleşen
// işaret, Hangıl, sınıfa göre sıralanan işaretler ve rastgele diziler; satırların kaçında x/text'in tek
// başına yanıldığı da sayılır (yanılmıyorsa fikstür tam yolu sınamıyordur).
func TestNormalizeMatchesUnicodeData(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path("internal/lang/lexer/testdata/normalize.txt"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) < 1600 || !strings.HasPrefix(lines[0], "# Unicode 15.0.0,") {
		t.Fatalf("fikstür beklenen değil: %d satır, %q", len(lines), lines[0])
	}
	alone := 0
	for i, line := range lines[1:] {
		columns := strings.Split(line, ";")
		if len(columns) != 5 {
			t.Fatalf("satır %d: %q", i+2, line)
		}
		src := fromHex(t, columns[0])
		for k, f := range Forms {
			want := fromHex(t, columns[k+1])
			if got := Normalize(f, src); got != want {
				t.Errorf("satır %d, %s(%s) = %s, beklenen %s", i+2, f, columns[0], toHex(got), columns[k+1])
			}
			if formTables[f].form.String(src) != want {
				alone++
			}
		}
	}
	if alone < 1000 {
		t.Errorf("x/text tek başına yalnız %d biçimde yanılıyor; fikstür tam yolu sınamıyor", alone)
	}
}

// Kural (13.2): biçimler ikilinin Unicode sürümünündür (17.0.0). Fikstürün sürümünden sonra gelen
// birleşikler UCD'nin satırlarından: 105C9 = 105D2 0307 ve 105E4 = 105DA 0307 (Todhri), 16121 = 1611E
// 1611E (Gurung Khema), 16D69 = 16D63 16D67 ve 16D6A = 16D69 16D67 (Kirat Rai). İbranice gimel ve
// sondaki kaf, üst noktayla Todhri harfine birleşmez (16 bitlik anahtarları aynıdır).
func TestNormalizeComposesNewerScripts(t *testing.T) {
	for _, c := range []struct{ src, composed string }{
		{"\U000105D2̇", "\U000105C9"}, {"\U000105DȦ", "\U000105E4"},
		{"ג̇", "ג̇"}, {"ך̇", "ך̇"}, {"ג̣̇", "ג̣̇"},
		{"가\U0001611E\U0001611E", "가\U00016121"},
		{"\U00016D63\U00016D67\U00016D67", "\U00016D6A"},
	} {
		for _, f := range Forms {
			want := c.src
			if f == FormNFC || f == FormNFKC {
				want = c.composed
			}
			if got := Normalize(f, c.src); got != want {
				t.Errorf("%s(%s) = %s, beklenen %s", f, toHex(c.src), toHex(got), toHex(want))
			}
		}
	}
}

// Kural (13.2): birleşen işaret dizisi ne kadar uzun olursa olsun sonuca karakter eklenmez (x/text tek
// başına 31. işaretin önüne U+034F koyar); biçimi bozuk baytta da metin yitmez.
func TestNormalizeAddsNoCharacter(t *testing.T) {
	for _, n := range []int{30, 31, 90, 5000} {
		src := "q" + strings.Repeat("́", n)
		for _, f := range Forms {
			if got := Normalize(f, src); got != src {
				t.Errorf("%s, %d işaret: %d bayt, beklenen %d", f, n, len(got), len(src))
			}
		}
		if n > 30 && norm.NFC.String(src) == src {
			t.Errorf("x/text %d işarete karakter eklemiyor; tam yolun bu koşulu artık gerekmiyor olabilir", n)
		}
	}
	src := "\U00020041̀\xff" + strings.Repeat("́", 40)
	for _, f := range Forms {
		if got := Normalize(f, src); !strings.Contains(got, "\xff") {
			t.Errorf("%s bozuk baytı yitirdi: %q", f, got)
		}
	}
}

// Kural (13.2): biçim kararlıdır: sonucun aynı biçimi kendisidir.
func TestNormalizeIsStable(t *testing.T) {
	for _, src := range []string{"", "a", "\U00020041̀", "ג̇", "e" + strings.Repeat("́", 40), "ﷺ̈́क़"} {
		for _, f := range Forms {
			once := Normalize(f, src)
			if twice := Normalize(f, once); twice != once {
				t.Errorf("%s(%s): %s, sonra %s", f, toHex(src), toHex(once), toHex(twice))
			}
		}
	}
}

// Kural (2.3): adlar NFC'lerine göre karşılaştırılır, NFC'si ayrı iki ad ayrıdır. x/text tek başına
// U+20041 U+0300'ü À'ya, İbranice gimel ve sondaki kaf ile üst noktayı Todhri harfine birleştiriyor, 31
// işaretli ada U+034F ekliyordu: iki ad sessizce bir oluyordu.
func TestNamesAboveFFFFCompareInNFC(t *testing.T) {
	marks := strings.Repeat("́", 30)
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{"é", "é", true},
		{"\U000105D2̇", "\U000105C9", true},
		{"\U00020041̀", "À", false},
		{"ג̇", "\U000105C9", false},
		{"ך̇", "\U000105E4", false},
		{"q" + marks + "́", "q" + marks + "͏́", false},
	} {
		l := NewBody(c.a+" "+c.b, true)
		toks := allOf(l)
		if len(l.Errors()) > 0 || len(toks) != 2 || toks[0].Type != TokIdent || toks[1].Type != TokIdent {
			t.Errorf("%s ve %s: %v %v", toHex(c.a), toHex(c.b), toks, l.Errors())
			continue
		}
		if same := toks[0].Literal == toks[1].Literal; same != c.same {
			t.Errorf("%s ve %s aynı ad: %v, beklenen %v (%s, %s)", toHex(c.a), toHex(c.b), same, c.same,
				toHex(toks[0].Literal), toHex(toks[1].Literal))
		}
	}
}

func fromHex(t *testing.T, field string) string {
	t.Helper()
	var b strings.Builder
	for _, word := range strings.Fields(field) {
		v, err := strconv.ParseUint(word, 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		b.WriteRune(rune(v))
	}
	return b.String()
}

func toHex(s string) string {
	var words []string
	for _, r := range s {
		words = append(words, fmt.Sprintf("%04x", r))
	}
	return strings.Join(words, " ")
}

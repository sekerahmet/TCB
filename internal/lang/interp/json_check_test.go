package interp

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// jsonPlace hatanın kodu, satırı ve sütunudur (yer, iletinin son iki değeridir).
func jsonPlace(t *testing.T, err error) (messages.Code, int, int) {
	t.Helper()
	m, ok := messages.As(err)
	if !ok || len(m.Args) < 2 {
		t.Fatalf("yerli kodlu hata bekleniyordu: %v", err)
	}
	return m.Code, m.Args[len(m.Args)-2].(int), m.Args[len(m.Args)-1].(int)
}

type placeCase struct {
	name         string
	in           string
	code         messages.Code
	line, column int
}

func checkPlaces(t *testing.T, cases []placeCase) {
	t.Helper()
	for _, c := range cases {
		err := CheckJSON([]byte(c.in))
		if err == nil {
			t.Errorf("%s (%q): hata bekleniyordu", c.name, c.in)
			continue
		}
		code, line, column := jsonPlace(t, err)
		if code != c.code || line != c.line || column != c.column {
			t.Errorf("%s (%q): %v satır %d sütun %d; beklenen %v satır %d sütun %d (%v)",
				c.name, c.in, code, line, column, c.code, c.line, c.column, err)
		}
	}
}

// Kural (3.7a): metin değerinde ve nesne anahtarında eşi olmayan UTF-16 vekil kaçışı çalışma
// hatasıdır (H2570); yer kaçışın ters bölüsüdür. Geçerli çift, noncharacter ve \u0000 geçer.
func TestJSONLoneSurrogateIsAnError(t *testing.T) {
	checkPlaces(t, []placeCase{
		{"yüksek", `"\ud800"`, messages.H2570, 1, 2},
		{"düşük ve harf", `"\udc00x"`, messages.H2570, 1, 2},
		{"yüksek ve harf", `"\ud83dx"`, messages.H2570, 1, 2},
		{"yüksek ve başka kaçış", `"\ud83d\u0041"`, messages.H2570, 1, 2},
		{"dizgi sonunda yüksek", `"a\ud83d"`, messages.H2570, 1, 3},
		{"büyük harfli yüksek", `"\uD800"`, messages.H2570, 1, 2},
		{"büyük harfli düşük", `"\uDC00"`, messages.H2570, 1, 2},
		{"yüksek ve ham 4 baytlık karakter", "\"\\ud800\xf0\x90\x80\x80\"", messages.H2570, 1, 2},
		{"anahtar", `{"\ud800": 1}`, messages.H2570, 1, 3},
		{"iki anahtar", `{"\ud800": 1, "\udbff": 2}`, messages.H2570, 1, 3},
		{"çok satır, Türkçe harften sonra", "{\n  \"ş\": \"\\udc00\"\n}", messages.H2570, 2, 9},
	})
	for _, in := range []string{`"\ud83d\ude00"`, `"\uD83D\uDE00"`, `"\uffff"`, `"\ufdd0"`, `"\u0000"`, `"\\ud800"`, "\"\xef\xbf\xbd\""} {
		if err := CheckJSON([]byte(in)); err != nil {
			t.Errorf("%q geçmeliydi: %v", in, err)
		}
	}
	// İki farklı vekilli anahtar yinelenen anahtar (H7011) sayılmaz: önce vekil söylenir.
	if _, err := JSONToTR([]byte(`{"\ud800": 1, "\udbff": 2}`)); !messages.Has(err, messages.H2570) || messages.Has(err, messages.H7011) {
		t.Errorf("JSONToTR: H2570 bekleniyordu: %v", err)
	}
	if v, err := JSONToTR([]byte(`"\ud83d\ude00"`)); err != nil || v.(*StringVal).Val != "\U0001F600" {
		t.Errorf("geçerli çift: %v, %v", v, err)
	}
}

// Kural (3.7a): geçersiz UTF-8 bayt, dizginin içinde de dışında da, H2571'dir; metnin sonunda
// yarım kalan çok baytlı dizi de (H2574 değil).
func TestJSONInvalidUTF8IsAnError(t *testing.T) {
	checkPlaces(t, []placeCase{
		{"dizgide", "\"a\xffb\"", messages.H2571, 1, 3},
		{"anahtarda", "{\"\xff\": 1}", messages.H2571, 1, 3},
		{"dizgi dışında", "\xff", messages.H2571, 1, 1},
		{"dizide", "[1, \xff]", messages.H2571, 1, 5},
		{"UTF-8'le yazılmış vekil", "\"\xed\xa0\x80\"", messages.H2571, 1, 2},
		{"fazla uzun yazım", "\"\xc0\xaf\"", messages.H2571, 1, 2},
		{"U+10FFFF'ten büyük", "\"\xf4\x90\x80\x80\"", messages.H2571, 1, 2},
		{"yarım dizi ve tırnak", "\"\xc3\"", messages.H2571, 1, 2},
		{"metnin sonunda yarım dizi", "\"\xc3", messages.H2571, 1, 2},
	})
	m, _ := messages.As(CheckJSON([]byte("\"\xed\xa0\x80\"")))
	if m.Args[0] != "ED" {
		t.Errorf("bayt ED olarak yazılmalıydı: %v", m.Args)
	}
}

// Kural (3.7a, 13.8): sözdizimi hatası TR iletisiyle, yeri ve nedeniyle gelir; Go'nun metni
// kullanıcıya gitmez. Boş metnin yeri metnin sonudur (H2579).
func TestJSONSyntaxErrorsSayPlaceAndReason(t *testing.T) {
	cases := []placeCase{
		{"boş", "", messages.H2579, 1, 1},
		{"yalnız boşluk", "   ", messages.H2579, 1, 4},
		{"çok satırlı boşluk (iki boşluk, LF, bir boşluk)", "  \n ", messages.H2579, 2, 2},
		{"değer yok", `{"a": }`, messages.H2573, 1, 7},
		{"iki nokta yok", `{"a" 1}`, messages.H2573, 1, 6},
		{"anahtar metin değil", `{1: 2}`, messages.H2572, 1, 2},
		{"nesnede sondaki virgül", `{"a": 1,}`, messages.H2573, 1, 9},
		{"dizide sondaki virgül", `[1,]`, messages.H2573, 1, 4},
		{"dizide virgül yok", `[1 2]`, messages.H2573, 1, 4},
		{"nesnede virgül yok", `{"a": 1 "b": 2}`, messages.H2572, 1, 9},
		{"kapanmamış metin", `"abc`, messages.H2574, 1, 1},
		{"kapanmamış dizi", `[1, 2`, messages.H2574, 1, 1},
		{"kapanmamış nesne", `{"a": 1`, messages.H2574, 1, 1},
		{"yarım true", `tru`, messages.H2578, 1, 1},
		{"yarım null", `nul`, messages.H2578, 1, 1},
		{"bilinmeyen ad", `x`, messages.H2578, 1, 1},
		{"Python'un True'su", `True`, messages.H2578, 1, 1},
		{"tek tırnaklı değer", `['a']`, messages.H2578, 1, 2},
		{"tek tırnaklı anahtar", `{'a': 1}`, messages.H2572, 1, 2},
		{"geçersiz kaçış", `"a\qb"`, messages.H2576, 1, 3},
		{"yarım \\u", `"\u12"`, messages.H2576, 1, 2},
		{"onaltılık olmayan \\u", `"\u12zz"`, messages.H2576, 1, 2},
		{"denetim karakteri", "\"a\tb\"", messages.H2575, 1, 3},
		{"0'la başlayan sayı", `01`, messages.H2577, 1, 1},
		{"noktayla biten sayı", `1.`, messages.H2577, 1, 1},
		{"yalnız eksi", `-`, messages.H2577, 1, 1},
		{"üssü eksik", `1e`, messages.H2577, 1, 1},
		{"artı işaretli", `+1`, messages.H2577, 1, 1},
		{"noktayla başlayan", `.5`, messages.H2577, 1, 1},
		{"fazla veri", `1 2`, messages.H2573, 1, 3},
		{"fazla kapanış", `[1]]`, messages.H2573, 1, 4},
		{"çok satır", "{\n  \"a\": 1,\n  \"b\": [1, 2,, 3]\n}", messages.H2573, 3, 14},
		{"Türkçe harften sonra", `{"ş": "ğ", x}`, messages.H2573, 1, 12},
		{"CR LF ile biten satırdan sonra", "[1,\r\n]", messages.H2573, 2, 1},
		{"BOM", "\xef\xbb\xbf1", messages.H2573, 1, 1},
	}
	checkPlaces(t, cases)
	for _, c := range cases {
		text := CheckJSON([]byte(c.in)).Error()
		for _, goWord := range []string{"invalid", "unexpected", "EOF", "looking for"} {
			if strings.Contains(text, goWord) {
				t.Errorf("%s: iletide Go'nun sözcüğü %q var: %s", c.name, goWord, text)
			}
		}
	}
	// Beklenen dallar: H2573'ün "expected"ı, H2572'nin "after"ı, H2574'ün "open"ı.
	for in, want := range map[string]string{`{"a": }`: "value", `{"a" 1}`: "colon", `{"a": 1,}`: "key", `[1 2]`: "array_next",
		`[,]`: "array_open", `1 2`: "end", `{1: 2}`: "open", `{"a": 1 "b": 2}`: "next", `[1, 2`: "array", `{"a": 1`: "object", `"abc`: "string"} {
		m, _ := messages.As(CheckJSON([]byte(in)))
		if m == nil || len(m.Args) < 3 || m.Args[len(m.Args)-3] != want {
			t.Errorf("%q: dal %q bekleniyordu: %v", in, want, m)
		}
	}
	// 1e999'un sözdizimi doğrudur; sayının sığmaması ayrıştırıcının H2503'üdür.
	if err := CheckJSON([]byte("1e999")); err != nil {
		t.Errorf("1e999: %v", err)
	}
}

// loneSurrogate taramadan bağımsız kıyas ölçütüdür (safetensors başlığının eski denetimi,
// DENETİM 587 Ş1): b'de eşi olmayan bir UTF-16 vekil kaçışı var mı?
func loneSurrogate(b []byte) bool {
	hex4 := func(i int) (rune, bool) {
		if i+6 > len(b) || b[i] != '\\' || b[i+1] != 'u' {
			return 0, false
		}
		n, err := strconv.ParseUint(string(b[i+2:i+6]), 16, 32)
		return rune(n), err == nil
	}
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' {
			continue
		}
		r, ok := hex4(i)
		if !ok {
			i++ // başka bir kaçış (\\, \", \n ...): iki bayt atlanır
			continue
		}
		switch {
		case r >= 0xDC00 && r <= 0xDFFF:
			return true
		case r >= 0xD800 && r <= 0xDBFF:
			low, ok := hex4(i + 6)
			if !ok || low < 0xDC00 || low > 0xDFFF {
				return true
			}
			i += 11
		default:
			i += 5
		}
	}
	return false
}

// jsonAgreement: H2500 dışında CheckJSON(b) == nil ⇔ json.Valid(b) && utf8.Valid(b) &&
// !loneSurrogate(b). Go 1.27.4'te json.Valid jsontext'in çözücüsüdür; derinliği 10000'e kadar
// kabul eder, TR 1000'de durur (H2500, ayrı satırlarda sınanır).
func jsonAgreement(t *testing.T, b []byte) {
	err := CheckJSON(b)
	if messages.Has(err, messages.H2500) {
		return
	}
	want := json.Valid(b) && utf8.Valid(b) && !loneSurrogate(b)
	if (err == nil) != want {
		t.Fatalf("%q: CheckJSON %v, Go'nun ölçütü geçerli=%v", b, err, want)
	}
	if m, ok := messages.As(err); err != nil && (!ok || m.Code < messages.H2570 || m.Code > messages.H2579) {
		t.Fatalf("%q: H2570–H2579 bekleniyordu: %v", b, err)
	}
}

var jsonAgreementSeeds = []string{
	``, ` `, `{}`, `[]`, `1`, `-0`, `1E+2`, `0.5e-3`, `true`, `[true,false,null]`, ` 1 `, `"ş"`,
	`{"a":[1,2,{"b":null}],"c":"ş"}`, `"\/\b\f\n\r\t\"\\"`, `"\u00e7"`, `"\ud83d\ude00"`, `"\ud800"`, `"\udc00"`,
	`"\\ud800"`, `{"a": }`, `{"a" 1}`, `{1: 2}`, `{"a": 1,}`, `[1,]`, `[1 2]`, `"abc`, `[1, 2`, `tru`, `True`,
	`"a\qb"`, `"\u12"`, "\"a\tb\"", `01`, `1.`, `-`, `+1`, `.5`, `1 2`, `[1]]`, `{'a': 1}`, "\"a\xffb\"",
	"\xff", "\"\xed\xa0\x80\"", "\"\xc3", "\xef\xbb\xbf1", "\"\x7f\"", `[1true]`, `truefalse`, `{"a":1}x`,
}

func TestCheckJSONAgreesWithGo(t *testing.T) {
	for _, s := range jsonAgreementSeeds {
		jsonAgreement(t, []byte(s))
	}
	// Derinlik: 1000 iç içe kap geçer, 1001 H2500'dür (dizi ve nesne).
	for _, open := range []string{"[", `{"a":`} {
		close := map[string]string{"[": "]", `{"a":`: "}"}[open]
		nest := func(n int) []byte { return []byte(strings.Repeat(open, n) + "1" + strings.Repeat(close, n)) }
		if err := CheckJSON(nest(maxJSONDepth)); err != nil {
			t.Errorf("%s ×1000: %v", open, err)
		}
		if err := CheckJSON(nest(maxJSONDepth + 1)); !messages.Has(err, messages.H2500) {
			t.Errorf("%s ×1001: H2500 bekleniyordu: %v", open, err)
		}
	}
}

func FuzzCheckJSON(f *testing.F) {
	for _, s := range jsonAgreementSeeds {
		f.Add([]byte(s))
	}
	f.Fuzz(jsonAgreement)
}

package interp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// runChecked kodu denetleyerek hazırlar (denetim hatası testi düşürür) ve vm'de çalıştırır; vm nil
// ise yeni bir VM açar. Çıktıyı ve çalışma hatasını döndürür.
func runChecked(t *testing.T, vm *VM, code string) (string, error) {
	t.Helper()
	r := source.Prepare(source.Header+"\n"+code, source.Options{TypeCheck: true})
	if r.Stage != "" {
		t.Fatalf("hazırlık (%s): %v\n%s", r.Stage, r.Errors, code)
	}
	var out bytes.Buffer
	if vm == nil {
		vm = New()
	}
	vm.Out, vm.Err = &out, &out
	_, err := vm.Run(r.Program)
	return out.String(), err
}

// runCheckedIn kodlu gövdeyi kişinin lang dilinde yazılan satır olarak denetler
// (checker.SetFileLanguage; denetim hatası testi düşürür) ve çalıştırır; çalışma hatasını döndürür.
func runCheckedIn(t *testing.T, lang, code string) error {
	t.Helper()
	p := parser.New(bodyLexer(code))
	prog := p.ParseProgram()
	c := checker.New()
	c.SetFileLanguage(lang)
	c.SetFile(checker.ProgramFile, code)
	c.CheckProgram(prog)
	if errs := append(p.Errs(), c.Errs()...); len(errs) > 0 {
		t.Fatalf("hazırlık: %v\n%s", errs, code)
	}
	vm := New()
	vm.Out, vm.Err = &bytes.Buffer{}, &bytes.Buffer{}
	_, err := vm.Run(prog)
	return err
}

// Kural (1.2): denetleyicinin tipini bilmediği değer tipli bir yere girerken çalışırken denetlenir:
// tanım, atama, bileşik atama, programın fonksiyonuna argüman, dönüş, alan yazma, kurucu, kap öğesi,
// öğe tipi bilinmeyen kap; modülde ve kabuğun ayrı satırlarında da. Uymazsa yerin denetimdeki
// koduyla çalışma hatasıdır ve değer yere girmez; uyan değer olduğu gibi girer. Tipsiz bir adla
// paylaşılan kaba sonradan yazılan öğe denetlenmez (K6'nın sınırı).
func TestUntypedValuesAreCheckedAtTypedPlaces(t *testing.T) {
	m := "TR_var m = [1, \"a\", 1.5]\n"
	k := "TR_struct K { x: TR_Int }\n"
	for _, c := range []struct {
		name, code string
		want       messages.Code
	}{
		{"tanım", m + "TR_var y: TR_Int = m[1]\n", messages.H1001},
		{"atama", m + "TR_var y: TR_Int = 0\ny = m[1]\n", messages.H1005},
		{"bileşik atama", m + "TR_var y: TR_Int = 1\ny += m[2]\n", messages.H1005},
		{"argüman", m + "TR_func f(x: TR_Int) -> TR_Int { TR_return x }\nTR_print(f(m[1]))\n", messages.H1004},
		{"dönüş", m + "TR_func f() -> TR_Int { TR_return m[1] }\nTR_print(f())\n", messages.H1003},
		{"alan", m + k + "TR_var o = K(x: 1)\no.x = m[1]\n", messages.H1124},
		{"kurucu", m + k + "TR_var o = K(x: m[1])\n", messages.H1125},
		{"kap öğesi", m + "TR_var l: TR_List[TR_Int] = [1]\nl[0] = m[1]\n", messages.H1136},
		{"kap öğesine bileşik", m + "TR_var l: TR_List[TR_Int] = [1]\nl[0] += m[2]\n", messages.H1136},
		{"öğe tipsiz kap", m + "TR_var y: TR_List[TR_Int] = m\n", messages.H1001},
		{"listeye bileşik ekleme", m + "TR_var y: TR_List[TR_Int] = [1]\ny += m\n", messages.H1005},
		{"ondalık yere tam sayı değeri", m + "TR_var y: TR_Float = m[0]\n", messages.H1001},
	} {
		out, err := runChecked(t, nil, c.code+"TR_print(\"girdi\")\n")
		if !messages.Has(err, c.want) || strings.Contains(out, "girdi") {
			t.Errorf("%s: %v hatası bekleniyordu ve program durmalıydı: %v\n%s", c.name, c.want, err, out)
		}
	}
	for _, c := range []struct{ name, code, out string }{
		{"uyan değer", m + "TR_var y: TR_Int = m[0]\nTR_print(y)\n", "1"},
		{"boş kap", "TR_var b = [[], \"a\"]\nTR_var e: TR_List[TR_String] = b[0]\nTR_print(e)\n", "[]"},
		{"uyan bileşik", m + "TR_var y: TR_Int = 1\ny += m[0]\nTR_print(y)\n", "2"},
		{"tipsiz yer", m + "TR_var y = m[1]\nTR_print(y)\n", "a"},
		{"paylaşılan kap (sınır)", "TR_var k = [[\"a\"], 1]\nTR_var l = k[0]\nTR_var g: TR_List[TR_String] = l\nl.TR_append(1)\nTR_print(g)\n", "[\"a\", 1]"},
	} {
		out, err := runChecked(t, nil, c.code)
		if err != nil || strings.TrimSpace(out) != c.out {
			t.Errorf("%s: %q bekleniyordu, gelen %q (%v)", c.name, c.out, out, err)
		}
	}
	// Kabukta her satır ayrı denetlenir; işaretler aynı VM'de birikir.
	vm := New()
	if _, err := runChecked(t, vm, m); err != nil {
		t.Fatal(err)
	}
	if _, err := runChecked(t, vm, "TR_var z = [1, \"b\"]\nTR_var y: TR_Int = z[1]\n"); !messages.Has(err, messages.H1001) {
		t.Errorf("kabuğun sonraki satırı da denetlenmeli: %v", err)
	}
	// Modülün kendi işaretleri modülde çalışır.
	root := t.TempDir()
	runInDir(t, root)
	_, err := runWithModules(t, root, map[string]string{
		"ana.tr": "TR_import \"tip\"\nTR_print(ilk())\n",
		"tip.tr": "TR_func ilk() -> TR_Int {\n    TR_var l = [\"a\", 1]\n    TR_return l[0]\n}\n",
	})
	if !messages.Has(err, messages.H1003) {
		t.Errorf("modüldeki dönüş de denetlenmeli: %v", err)
	}
}

// Kural (1.2, 3.13): çalışırken uygunluk denetleyicinin kuralıdır. Tipi bilinen bir değişkenin değeri
// tipli yere denetimde ne karar alıyorsa, aynı değer tipsiz yoldan girdiğinde çalışırken aynı kararı
// alır: kap iç içe, demet öğe öğe, iki bilinen yapı ayrı; tam sayı değeri ondalık yere ikisinde de
// girmez. Tensörün cinsi ve fonksiyonun imzası ikisinde de karar değildir.
func TestRuntimeFitsMatchesTheChecker(t *testing.T) {
	defs := "TR_struct A { x: TR_Int }\nTR_struct B { x: TR_Int }\nTR_func id(x: TR_Int) -> TR_Int { TR_return x }\n"
	for _, c := range []struct {
		expected, value string
		fits            bool
	}{
		{"TR_Int", "1", true},
		{"TR_Float", "1", false},
		{"TR_Float", "1.5", true},
		{"TR_String?", "\"a\"", true},
		{"TR_Int?", "TR_none", true},
		{"TR_List[TR_Int]", "[1, 2]", true},
		{"TR_List[TR_Int]", "[\"a\"]", false},
		{"TR_List[TR_Float]", "[1]", false},
		{"TR_List[TR_List[TR_Int]]", "[[1], [2]]", true},
		{"TR_List[TR_List[TR_String]]", "[[1], [2]]", false},
		{"TR_Dict[TR_String, TR_Int]", "{\"a\": 1}", true},
		{"TR_Dict[TR_String, TR_Int]", "{\"a\": \"b\"}", false},
		{"TR_Dict[TR_String, TR_Int]", "{1: 1}", false},
		{"TR_List[TR_JSON]", "[TR_none]", true},
		{"(TR_Int, TR_String)", "(1, \"a\")", true},
		{"(TR_Int, TR_String)", "(1, 2)", false},
		{"A", "A(x: 1)", true},
		{"A", "B(x: 1)", false},
		{"TR_Function", "id", true},
		{"TR_Int", "id", false},
	} {
		typed := defs + "TR_var s = " + c.value + "\nTR_var y: " + c.expected + " = s\n"
		r := source.Prepare(source.Header+"\n"+typed, source.Options{TypeCheck: true})
		if checked := r.Stage == ""; checked != c.fits {
			t.Errorf("%s ← %s: denetleyici %v diyor, tablo %v (%v)", c.expected, c.value, checked, c.fits, r.Errors)
		}
		_, err := runChecked(t, nil, defs+"TR_var u = ["+c.value+", \"z\", 0.5]\nTR_var y: "+c.expected+" = u[0]\n")
		if (err == nil) != c.fits {
			t.Errorf("%s ← %s: çalışma anı %v diyor, denetleyici %v (%v)", c.expected, c.value, err == nil, c.fits, err)
		}
	}
}

// Kural (1.2, 2.6): çalışma hatasının iletisi değerin çalışırken bilinen tipini yazar (öğeleri tek
// tipte kap öğe tipiyle, karışık kap asıl adıyla) ve kapta uymayan ilk öğeyi yoluyla ve tipiyle
// (H1170; anahtarda H1171). Yoldaki metin anahtar TR'nin metin yazımıyla, ters tırnak \u{60}
// ile yazılır; yazımı olmayan yol (TR_Bool anahtar), birden çok satırlık ya da ters tırnaklı değer
// ipucu vermez. Kişinin dilinde yazılan satırda (lang) tip adları iletinin dilindedir.
func TestUntypedValueErrorsSayTheValue(t *testing.T) {
	for _, c := range []struct {
		name, lang, code string
		parts, absent    []string
	}{
		{"karışık kap", "", "TR_var m = [1, \"a\"]\nTR_var y: TR_List[TR_Int] = m\n",
			[]string{"\"TR_List\" tipinde", "uymayan ilk öğe `m[1]`, tipi \"TR_String\""}, nil},
		{"tek tipli kap", "", "TR_var m = [[\"a\"], 1]\nTR_var y: TR_List[TR_Int] = m[0]\n",
			[]string{"\"TR_List[TR_String]\" tipinde", "uymayan ilk öğe `m[0][0]`"}, nil},
		{"iç içe sözlük", "", "TR_var m = [{\"a\\tb\": [\"x\"]}, 1]\nTR_var y: TR_Dict[TR_String, TR_List[TR_Int]] = m[0]\n",
			[]string{"`m[0][\"a\\tb\"][0]`, tipi \"TR_String\""}, nil},
		{"ters tırnaklı anahtar", "", "TR_var m = [{\"`\": \"x\"}, 1]\nTR_var y: TR_Dict[TR_String, TR_Int] = m[0]\n",
			[]string{"`m[0][\"\\u{60}\"]`"}, nil},
		{"anahtar", "", "TR_var m = [{1: 2}, \"a\"]\nTR_var y: TR_Dict[TR_String, TR_Int] = m[0]\n",
			[]string{"uymayan ilk anahtar `m[0]` sözlüğündeki \"1\", tipi \"TR_Int\""}, nil},
		{"yazımı olmayan anahtar", "", "TR_var m = [{TR_true: [\"x\"]}, 1]\nTR_var y: TR_Dict[TR_Bool, TR_List[TR_Int]] = m[0]\n",
			[]string{"\"TR_Dict[TR_Bool, TR_List[TR_String]]\" tipinde"}, []string{"uymayan"}},
		{"kap değil", "", "TR_var m = [1, \"a\"]\nTR_var y: TR_Int = m[1]\n",
			[]string{"\"TR_String\" tipinde"}, []string{"uymayan"}},
		{"birden çok satır", "", "TR_var m = [1, \"a\"]\nTR_var y: TR_List[TR_Int] = [\n    m[1],\n]\n",
			nil, []string{"uymayan"}},
		{"ters tırnaklı değer yazımı", "", "TR_var m = {\"a`b\": [\"x\"], \"c\": 1}\nTR_var y: TR_List[TR_Int] = m[\"a`b\"]\n",
			[]string{"\"TR_List[TR_String]\" tipinde"}, []string{"uymayan"}},
		{"iletinin dilinde tip adı", "en", "TR_var m = [1, \"a\"]\nTR_var y: TR_List[TR_Int] = m\n",
			[]string{"\"List\"", "of type \"String\""}, []string{"TR_List", "TR_String"}},
	} {
		var err error
		if c.lang == "" {
			_, err = runChecked(t, nil, c.code)
		} else {
			err = runCheckedIn(t, c.lang, c.code)
		}
		if err == nil {
			t.Errorf("%s: hata bekleniyordu", c.name)
			continue
		}
		text := err.Error()
		if c.lang != "" {
			text = messages.ErrorIn(err, c.lang)
		}
		for _, p := range c.parts {
			if !strings.Contains(text, p) {
				t.Errorf("%s: ileti %q parçasını içermeli: %s", c.name, p, text)
			}
		}
		for _, p := range c.absent {
			if strings.Contains(text, p) {
				t.Errorf("%s: ileti %q içermemeli: %s", c.name, p, text)
			}
		}
	}
}

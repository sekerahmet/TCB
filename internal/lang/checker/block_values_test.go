package checker

import (
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// checkBodyIn lang'de (nil: kodlu gövde) yazılmış kodu ayrıştırıp denetler; denetimin hatalarını verir.
func checkBodyIn(t *testing.T, code string, lang *codes.Language, interactive bool) []error {
	t.Helper()
	l := lexer.NewBody(code, true)
	if lang != nil {
		l = lexer.NewBodyIn(code, false, lang)
	}
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errs()) > 0 {
		t.Fatalf("%q: %v", code, p.Errs())
	}
	c := New()
	if interactive {
		c.SetInteractive()
	}
	c.CheckProgram(prog)
	return c.Errs()
}

// Çağrı içermeyen, değeri kullanılmayan ifade denetim hatasıdır (2.8): satır başındaki '-' önceki
// satırı sürdürmez, bu yüzden 'a = 1' ardından gelen '- b' bölünmüş bir formülün kopan parçasıdır.
// İkili biçimi de olan işleçle başlayan satırın iletisi işleci önceki satırın sonuna taşımayı söyler
// (H1017); öbürleri değeri bir değişkene atamayı ya da satırı silmeyi (H1023). Çağrı (x |> f de)
// deyimdir. Kabukta (etkileşimli) ifadenin değeri gösterildiği için hata yoktur.
func TestUnusedValueIsAnError(t *testing.T) {
	errs := checkBodyIn(t, "TR_var b = 2\nTR_var a = 1\n    - b", nil, false)
	if len(errs) != 1 || !messages.Has(errs[0], messages.H1017) {
		t.Fatalf("H1017 bekleniyordu: %v", errs)
	}
	if line, _, _ := messages.Pos(errs[0]); line != 3 {
		t.Errorf("hata 3. satırda olmalı: %v", errs[0])
	}
	errs = checkBodyIn(t, "TR_var k = {\"ad\": 1}\nk.ad", nil, false)
	if len(errs) != 1 || !messages.Has(errs[0], messages.H1023) {
		t.Fatalf("H1023 bekleniyordu: %v", errs)
	}
	if line, _, _ := messages.Pos(errs[0]); line != 2 {
		t.Errorf("hata 2. satırda olmalı: %v", errs[0])
	}
	for _, code := range []string{
		"TR_var b = 2\nTR_var a = 1 -\n    b\nTR_var c = -b",
		"TR_var l = [1]\nl.TR_append(2)",
		"TR_var x = 1\nTR_String(x + 2)",
		"TR_func f() -> TR_Int { TR_return 1 }\n-f()",
	} {
		if errs := checkBodyIn(t, code, nil, false); len(errs) != 0 {
			t.Errorf("kullanılan değerde ya da çağrıda hata olmamalı: %q: %v", code, errs)
		}
	}
	if errs := checkBodyIn(t, "TR_var b = 2\n-b", nil, true); len(errs) != 0 {
		t.Errorf("kabukta hata olmamalı: %v", errs)
	}
}

// Değeri kullanılan bir bloğun son ifadesi değerdir: 'TR_var g = TR_if a { 0.01 } TR_else { -0.01 }'
// bloğun değeridir (iç içe eğer ve eşle de); değeri kullanılmayan eğer deyiminin bloğundaki '-x'
// hata olmaya devam eder. Bir dildeki yazım (kabuk satırı) önce kodlanır, denetleyici kodlu gövdeyi okur.
func TestBlockValueIsNotAnUnusedValue(t *testing.T) {
	for _, c := range []struct {
		lang   *codes.Language
		used   []string
		unused string
	}{
		{nil, []string{
			"TR_var a = TR_true\nTR_var g = TR_if a { 0.01 } TR_else { -0.01 }\nTR_print(g)",
			"TR_var a = TR_true\nTR_print(TR_if a { 1 } TR_else TR_if TR_not a { -1 } TR_else { TR_if a { -2 } TR_else { -3 } })",
			"TR_var a = 1\nTR_print(TR_match a { 1 => { -1 }; _ => { -2 } })",
		}, "TR_var a = TR_true\nTR_var b = 1\nTR_if a {\n    -b\n}"},
	} {
		name := "kodlu"
		if c.lang != nil {
			name = c.lang.Code
		}
		for _, code := range c.used {
			if errs := checkBodyIn(t, code, c.lang, false); len(errs) != 0 {
				t.Errorf("%s: değeri kullanılan bloğun son ifadesi hata vermemeli: %q: %v", name, code, errs)
			}
		}
		if errs := checkBodyIn(t, c.unused, c.lang, false); len(errs) != 1 || !messages.Has(errs[0], messages.H1017) {
			t.Errorf("%s: değeri kullanılmayan eğer deyimindeki '-b' hata vermeli: %v", name, errs)
		}
	}
}

// Kural (5.5, 5.6, 1.2): değer veren eğer ve eşlenin tipi dallarının ortak tipidir (değişmezdeki gibi
// tam sayı yazımı ondalığın yanında ondalıktır); TR_return ya da TR_throw ile biten kol değer
// vermez. Dallar farklıysa ya da biri bilinmiyorsa TR_Any.
func TestIfAndMatchValueTypes(t *testing.T) {
	expectTypes(t, map[string]string{
		"TR_var a = 1\nTR_var v = TR_if a > 0 { \"+\" } TR_else { \"-\" }":                     typeString,
		"TR_var a = 1\nTR_var v = TR_if a > 0 { 1 } TR_else TR_if a < 0 { 2.5 } TR_else { 0 }": typeFloat,
		"TR_var a = 1\nTR_var v = TR_if a > 0 { 1 } TR_else { \"x\" }":                         typeAny,
		"TR_var a = 1\nTR_var v = TR_if a > 0 { " + unknownValue + " } TR_else { \"x\" }":      typeAny,
		"TR_var a = 1\nTR_var v = TR_match a { 1 => \"bir\"\n    _ => \"çok\" }":               typeString,
		"TR_var a = 1\nTR_var v = TR_match a { 1 => { TR_var b = 2\n        b }\n    _ => 0 }": typeInt,
		"TR_var a = 1\nTR_var v = TR_match a { 1 => [1]\n    _ => { TR_throw \"yok\" } }":      listType(typeInt),
		"TR_var a = 1\nTR_var v = TR_match a { 1 => 1\n    _ => \"x\" }":                       typeAny,
		"TR_var a = 1\nTR_var v = TR_if a > 0 { 1 } TR_else { TR_none }":                       optionalType(typeInt),
		"TR_var a = 1\nTR_var v = TR_match a { 1 => TR_none\n    _ => \"x\" }":                 optionalType(typeString),
	})
	singleError(t, "TR_var a = 1\nTR_var v = TR_if a > 0 { \"+\" } TR_else { \"-\" }\nv = 3", messages.H1005, 3, "TR_String")
}

// Kural (5.5, 5.6, 6.6): değer veren dalların hepsi TR_none veriyorsa tip beklenen tipten gelir;
// beklenen tip yoksa ya da her tipi alıyorsa (TR_print'in argümanı, TR_Any) H1161. Beklenen tip:
// tipli değişken ve atama, dönüş tipi, kullanıcı fonksiyonunun parametresi, tipli fonksiyon
// ifadesinin gövdesi, tipli listenin öğesi; tipi bilinen alana ve kabın öğesine atama, öğe tipi
// bilinen listenin .TR_append'i (DENETİM 400 Ş3). Tipi bilinmeyen kabın öğesi beklenen tip vermez.
// Başka bir dalın değeri olan iç ifadeye dıştaki karar verir. Beklenen tip isteğe bağlı değilse
// TestAllNoneBranchesAreANone.
func TestAllNoneBranchesNeedAnExpectedType(t *testing.T) {
	head := "TR_var c = TR_true\nTR_func al(x: TR_Int?) -> TR_Int {\n    TR_return x ?? 0\n}\n" +
		"TR_struct Kap {\n    x: TR_Int? = 1\n}\nTR_var k = Kap()\nTR_var l: TR_List[TR_Int?] = [1]\n" +
		"TR_var d: TR_Dict[TR_String, TR_Int?] = {\"a\": 1}\n"
	none := "TR_if c { TR_none } TR_else { TR_none }"
	match := "TR_match 1 {\n    1 => TR_none\n    _ => TR_none\n}"
	for _, src := range []string{
		"TR_print(" + none + ")",
		"TR_var a = " + none,
		"TR_var a = " + match,
		"TR_print(" + match + ")",
		"TR_var g = (b: TR_Bool) => TR_if b { TR_none } TR_else { TR_none }",
		"TR_var a = TR_if c { " + none + " } TR_else { TR_none }",
		"TR_var m = [" + none + "]",
		"TR_var u = [1, \"a\"]\nu[0] = " + none,
	} {
		_, errs := checkErrs(t, head+src)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H1161) {
			t.Errorf("%q: tek H1161 bekleniyordu, gelen %v", src, errs)
		}
	}
	for _, src := range []string{
		"TR_var y: TR_Int? = " + none,
		"TR_var y: TR_String? = " + match,
		"TR_var y: TR_Int? = 1\ny = " + none,
		"TR_func f() -> TR_Int? {\n    TR_return " + none + "\n}",
		"TR_print(al(" + none + "))",
		"TR_var f: (TR_Bool) -> TR_Int? = (b) => TR_if b { TR_none } TR_else { TR_none }",
		"TR_var m: TR_List[TR_Int?] = [" + none + "]",
		"k.x = " + none,
		"l[0] = " + none,
		"l.TR_append(" + none + ")",
		"d[\"b\"] = " + match,
		"TR_var w = TR_if c { " + none + " } TR_else { 1 }",
		"TR_var y: TR_Int? = TR_if c { " + none + " } TR_else { TR_none }",
	} {
		if _, errs := checkErrs(t, head+src); len(errs) > 0 {
			t.Errorf("%q hatasız olmalı: %v", src, errs)
		}
	}
}

// Kural (5.5, 5.6, 3.5): değer veren dalların hepsi TR_none veriyorsa ifade düz TR_none gibidir: tipi
// TR_NoneType'tır, isteğe bağlı beklenen tipe (T?) girer, isteğe bağlı olmayana girmez. Her yazımda
// düz TR_none tek bir hata verir; eğer ve eşle biçimi de aynı kodla tek hata verir (H1161 değil),
// isteğe bağlı karşılığı hatasızdır. Öğe ve sözlük değeri ataması düz TR_none'da da denetlenir
// (DENETİM 400 kapanış K-Ş1, K-N1). Çekirdek metodun tek tipli parametresinin isteğe bağlı karşılığı yoktur.
func TestAllNoneBranchesAreANone(t *testing.T) {
	head := "TR_var c = TR_true\nTR_struct HepYokKabı {\n    x: TR_Int = 1\n    y: TR_Int? = 1\n}\n" +
		"TR_var k = HepYokKabı()\nTR_var l: TR_List[TR_Int] = [1]\nTR_var lo: TR_List[TR_Int?] = [1]\n" +
		"TR_var d: TR_Dict[TR_String, TR_Int] = {\"a\": 1}\nTR_var do: TR_Dict[TR_String, TR_Int?] = {\"a\": 1}\n" +
		"TR_func tam(x: TR_Int) -> TR_Int {\n    TR_return x\n}\nTR_func belki(x: TR_Int?) -> TR_Int {\n    TR_return x ?? 0\n}\n" +
		"TR_var y2: TR_Int = 1\nTR_var y3: TR_Int? = 1\n"
	forms := []string{
		"TR_if c { TR_none } TR_else { TR_none }",
		"TR_match 1 {\n    1 => TR_none\n    _ => TR_none\n}",
		"TR_if c { TR_if c { TR_none } TR_else { TR_none } } TR_else { TR_none }",
	}
	for _, tc := range []struct{ required, optional string }{
		{"l.TR_append(@)", "lo.TR_append(@)"},
		{"k.x = @", "k.y = @"},
		{"l[0] = @", "lo[0] = @"},
		{"d[\"b\"] = @", "do[\"b\"] = @"},
		{"TR_var s = \" a \".TR_strip(@)", ""},
		{"TR_var y: TR_Int = @", "TR_var y: TR_Int? = @"},
		{"y2 = @", "y3 = @"},
		{"TR_func f() -> TR_Int {\n    TR_return @\n}", "TR_func f() -> TR_Int? {\n    TR_return @\n}"},
		{"TR_print(tam(@))", "TR_print(belki(@))"},
	} {
		_, plain := checkErrs(t, head+strings.ReplaceAll(tc.required, "@", "TR_none"))
		var want *messages.Error
		if len(plain) != 1 || !errors.As(plain[0], &want) {
			t.Errorf("%q: düz TR_none tek hata vermeli, gelen %v", tc.required, plain)
			continue
		}
		for _, form := range forms {
			src := strings.ReplaceAll(tc.required, "@", form)
			if _, errs := checkErrs(t, head+src); len(errs) != 1 || !messages.Has(errs[0], want.Code) {
				t.Errorf("%q: düz TR_none gibi tek %s bekleniyordu, gelen %v", src, want.Code, errs)
			}
			if tc.optional == "" {
				continue
			}
			src = strings.ReplaceAll(tc.optional, "@", form)
			if _, errs := checkErrs(t, head+src); len(errs) > 0 {
				t.Errorf("%q hatasız olmalı: %v", src, errs)
			}
		}
	}
}

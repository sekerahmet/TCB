package messages

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Cümle parçalardan kurulmaz: bir iletinin anlamı tek şablondadır, çünkü başka dillerde
// sözcük sırası, cinsiyet ve çekim farklıdır. Bir ileti koduna başka bir ileti kodunun metni
// (yüklem, nesne, iyelik parçası: "okunamadı", "bir değişkene") argüman olarak geçirilirse
// çevrilemez; her anlam kendi tam cümlesi olur, değişen yer şablonda `select` ile seçilir.
// Test, depodaki test dışı Go kodunda üç biçimi arar:
//
//	(a) bir ileti çağrısına (`messages.New/Text`, `syntaxError`, `turkish.NewReason`) argüman
//	    olarak doğrudan verilen `messages.Text(...)`;
//	(b) aynı işlevde bir değişkene alınıp (`x := messages.Text(...)`, `x = ...`) sonra bir ileti
//	    çağrısına değer olarak geçirilen metin;
//	(c) `+` ile Go'da birleştirilen ileti metni (`"..." + messages.Text(...)`).
//
// Hata değeri taşıyan argümanlar (`messages.New(...)`, hata zinciri: "{fn}: {error}") parça
// sayılmaz. `interp.ArgCountError`'a verilen parametre adları (H2300-H2330: "bir değer",
// "dosya yolu") kendi başına anlamlı birer ad öbeğidir (messages README'si buna izin verir).
//
// Açık ölçüt (sunum): `+`nın öbür işleneni yalnızca boşluk, simge (❌ ✗ →), işaret ve satır
// sonundan oluşan bir sabitse (`"  ⚠️ "`, `"\n"`) birleştirme sunumdur, cümle kurmaz. Harf ya da
// noktalama (`", "`, `": "`) içeren sabit cümleyi birleştirir; yasaktır.
//
// Açık ölçüt (ad öbeği): kendi başına anlamlı, her dilde bağımsız çevrilen bir ad öbeği (tür
// adı, "bir tam sayı", konu etiketi) değer olabilir; öbek cümlede yalın durur, kendisi hâl/iyelik
// eki almaz. Böyle bir kod nounPhraseUses'a "dosya KOD" anahtarıyla, gerekçesiyle yazılır.
// Yüklem, ek alan öbek ve sözcük sırasını belirleyen parça ad öbeği değildir.
//
// Açık ölçüt (ipucu yuvası): ana iletinin sonuna eklenen, kendi başına tam cümle olan ipucu
// ("Bunu mu demek istediniz ...", H1104/H2479) tanımda adı `hint` ya da `suggestion` olan
// parametreye ileti olarak verilebilir (isHintSlot). Başka adlı parametreye verilemez.
// Boş olabilen parça ise Go'da metin kurulmadan şablonda `{x, select, empty {} other {..}}`
// ile yazılır.
//
// nounPhraseUses kuralın bugünkü istisnalarıdır (kayıt → kullanım sayısı) ve yalnızca küçülür;
// her kayıt gerekçesini yazar. Bugün boştur: eski etiketler (H2408/H2409, H6636/H6637) ileti
// parçası (messages.New) olarak verilir.
var nounPhraseUses = map[string]int{}

// messageBuilders ileti kuran işlevlerdir; argümanları iletinin parametreleridir.
var messageBuilders = map[string]bool{
	"New": true, "Text": true, "NewReason": true, "syntaxError": true, "Internalf": true, "Internal": true,
}

// fragmentSite bir parça kullanımıdır: yer, parça iletinin kodu ve hangi biçim.
type fragmentSite struct {
	pos  string
	code string
	kind string
}

func TestMessageArgsAreNotMessageFragments(t *testing.T) {
	root := reporoot.Root()
	found := map[string]int{}
	var where []string
	for _, top := range []string{"cmd", "internal", "lib", "tools"} {
		err := reporoot.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			fset := token.NewFileSet()
			f, err := goparser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			for _, site := range fragmentSites(fset, f) {
				key := rel + " " + site.code
				found[key]++
				where = append(where, site.kind+" "+site.pos+" "+site.code)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for key, n := range found {
		switch allowed := nounPhraseUses[key]; {
		case n > allowed:
			t.Errorf("%s: bir iletinin metni başka bir iletiye parça olarak veriliyor ya da Go'da birleştiriliyor (%d kez, izinli %d); anlamı tek şablonda, tam cümle olarak yazın (select ile)", key, n, allowed)
		case n < allowed:
			t.Errorf("%s: izin listesi eskimiş (%d kullanım kaldı, listede %d); kaydı küçültün", key, n, allowed)
		}
	}
	for key := range nounPhraseUses {
		if found[key] == 0 {
			t.Errorf("%s: artık ihlal yok; kaydı silin", key)
		}
	}
	if t.Failed() {
		sort.Strings(where)
		t.Logf("kullanım yerleri:\n%s", strings.Join(where, "\n"))
	}
}

// fragmentSites dosyadaki parça kullanımlarını verir (üç biçim, başlıktaki a, b, c).
func fragmentSites(fset *token.FileSet, f *ast.File) []fragmentSite {
	var sites []fragmentSite
	add := func(e ast.Expr, kind string) {
		sites = append(sites, fragmentSite{fset.Position(e.Pos()).String(), textCode(e), kind})
	}
	// scan bir işlev gövdesini tarar; değişken izleme işlev başınadır.
	scan := func(body ast.Node) {
		vars := map[string][]string{} // metin değişkeni → aldığı iletilerin kodları
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for i, r := range x.Rhs {
					if id, ok := x.Lhs[min(i, len(x.Lhs)-1)].(*ast.Ident); ok && isTextCall(r) {
						vars[id.Name] = append(vars[id.Name], textCode(r))
					}
				}
			case *ast.ValueSpec:
				for i, r := range x.Values {
					if i < len(x.Names) && isTextCall(r) {
						vars[x.Names[i].Name] = append(vars[x.Names[i].Name], textCode(r))
					}
				}
			}
			return true
		})
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if !isMessageBuilder(x) {
					return true
				}
				for i, a := range x.Args {
					if isHintSlot(x, i) {
						continue
					}
					if isTextCall(a) {
						add(a, "doğrudan")
					} else if id, ok := a.(*ast.Ident); ok {
						for _, code := range vars[id.Name] {
							sites = append(sites, fragmentSite{fset.Position(a.Pos()).String(), code, "değişken"})
						}
					}
				}
			case *ast.BinaryExpr:
				if x.Op != token.ADD {
					return true
				}
				for _, pair := range [][2]ast.Expr{{x.X, x.Y}, {x.Y, x.X}} {
					if isTextCall(pair[0]) && !isPresentationOnly(pair[1]) {
						add(pair[0], "birleştirme")
					}
				}
			}
			return true
		})
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
			scan(fn.Body)
		}
	}
	return sites
}

// hintSlots ipucu/öneri yuvasının parametre adlarıdır. Ana iletinin sonuna eklenen, kendi
// başına tam bir cümle olan ipucu (H1104 "Bunu mu demek istediniz ...") bu adlı parametreye
// ileti olarak verilebilir: tek kaynaktan (hint paketi) gelir, dilbilgisini ana iletiden almaz.
var hintSlots = map[string]bool{"hint": true, "suggestion": true}

// isHintSlot `messages.New/Text(messages.Hxxxx, ...)` çağrısının i. argümanı, tanımda adı
// hintSlots'ta olan parametre mi?
func isHintSlot(call *ast.CallExpr, i int) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "New" && sel.Sel.Name != "Text") || len(call.Args) == 0 || i == 0 {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "messages" {
		return false
	}
	name, ok := call.Args[0].(*ast.SelectorExpr)
	if !ok {
		return false
	}
	code, err := Parse(name.Sel.Name)
	if err != nil {
		return false
	}
	load()
	params := definitions.Params[code]
	return i-1 < len(params) && hintSlots[params[i-1]]
}

// textCode `messages.Text(KOD, ...)` çağrısının kod adıdır (H1104); okunamazsa "?".
func textCode(e ast.Expr) string {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return "?"
	}
	switch a := call.Args[0].(type) {
	case *ast.SelectorExpr:
		return a.Sel.Name
	case *ast.Ident:
		return a.Name
	}
	return "?"
}

// isPresentationOnly ifade yalnızca boşluk, simge ve satır sonundan oluşan bir dizi sabiti mi?
func isPresentationOnly(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return false
	}
	for _, r := range s {
		if !unicode.IsSpace(r) && !unicode.IsSymbol(r) && !unicode.IsMark(r) && r != '·' && r != '•' {
			return false
		}
	}
	return true
}

// TestFragmentDetector dedektörün üç biçimi yakaladığını ve sunum önekini bıraktığını gösterir.
func TestFragmentDetector(t *testing.T) {
	src := `package p
func a() {
	x := messages.Text(messages.H1)
	_ = messages.New(messages.H2, x)
	y := messages.Text(messages.H3)
	y = messages.Text(messages.H4, y)
	_ = ", " + messages.Text(messages.H5)
	_ = messages.Text(messages.H6) + ": " + "x"
	_ = "❌ " + messages.Text(messages.H7) + "\n"
	_ = messages.New(messages.H8, messages.Text(messages.H9))
	_ = messages.New(messages.H2410, a, b, messages.Text(messages.H1104, c))
	_ = messages.New(messages.H2410, messages.Text(messages.H1104, c), b, a)
	z := messages.Text(messages.H10)
	_ = z
}
`
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range fragmentSites(fset, f) {
		got = append(got, s.kind+":"+s.code)
	}
	sort.Strings(got)
	want := []string{"birleştirme:H5", "birleştirme:H6", "değişken:H1", "değişken:H3", "değişken:H4", "doğrudan:H1104", "doğrudan:H9"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("bulunan %v, beklenen %v", got, want)
	}
}

// isMessageBuilder çağrı `messages.New(...)` gibi bir ileti kuran işlev ya da yöntem mi?
func isMessageBuilder(call *ast.CallExpr) bool {
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		if pkg, ok := f.X.(*ast.Ident); ok && pkg.Name == "messages" {
			return messageBuilders[f.Sel.Name]
		}
		return f.Sel.Name == "syntaxError" || f.Sel.Name == "NewReason"
	case *ast.Ident:
		return f.Name == "NewReason"
	}
	return false
}

// isTextCall ifade `messages.Text(...)` çağrısı mı?
func isTextCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "messages" && sel.Sel.Name == "Text"
}

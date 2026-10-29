package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// İleti ve içindeki dil adları tek dildedir: ad iletiye kodla verilir (codes.NameRef,
// interp.CallName, bind.Named), katalog onu iletinin dilinde yazar. Yazıldığı anda bir dilde
// kurulmuş metin (codes.Name, codes.DisplayName, bind.Named'in .TR'si, bir
// Ref'in String()'i, hazır ileti metni messages.Text, bunları içeren fmt.Sprint) iletiye ne
// doğrudan, ne bir yerel değişkenle, ne de onu iletiye ileten bir fonksiyonun parametresiyle
// (fn, ad, iletinin değerlerini alan değişken sayılı sarmalayıcılar: syntaxError, NewReason ...)
// ulaşır: yakalanan hatada ileti yakalayan dosyanın dilindedir, böyle bir metin öteki dilde
// kalırdı.
//
// Kapsam dışı: hesap katmanı (internal/compute) dil adlarını bilmez, ona verilen işlem etiketi
// veridir; katalogda example adlı parametre örnek koddur (dile çevrilmesi ayrı tasarım, yol
// haritası 10). Gerekçeli tek tek istisnalar probes'tadır.
func TestNoFixedNamesInMessages(t *testing.T) {
	for _, v := range fixedNameFlows(t, reporoot.Root()) {
		t.Errorf("%s: iletiye %s ile yazılmış ad ulaşıyor; ileti ve içindeki adlar tek dildedir, kodu verin (codes.NameRef, interp.CallName, bind.Named)", v.pos, v.code)
	}
}

// probes kurulmuş adı iletiye gitmeyen bir yoklamada kullanan çağrılardır (çağıran → çağrılan):
// sonuç hatası atılır, kullanıcıya ulaşmaz.
var probes = map[[2]string]string{
	{"interp.memberSuggestionError", ".GetProperty"}: "önerilen üyeyi kanonik adıyla yoklar; hata atılır",
}

// exampleParams katalogda örnek kod ya da kullanım metni taşıyan parametre adlarıdır (dile
// çevrilmeleri ayrı tasarım, yol haritası 10).
var exampleParams = map[string]bool{"example": true, "usage": true, "signature": true, "customGradExample": true}

// displayLayers iletileri yalnız kişinin dilinde (TR_LANG) gösteren katmanlardır: burada hiçbir
// hata yakalanmaz, iç içe hazır metin (messages.Text) dıştaki iletiyle aynı dilde kalır; bu
// kaynak orada sayılmaz.
var displayLayers = []string{"cmd/", "internal/devtools/"}

// goFunc bir fonksiyon bildirimidir; key çağrıdaki adıyla eşleşir (paket.Ad; metotta .Ad).
type goFunc struct {
	key      string
	params   []string
	variadic bool // son parametre ...T
	decl     *ast.FuncDecl
	file     *goFile
}

type goFile struct {
	pkg     string
	imports map[string]bool // dosyanın içe aktardığı paket adları
	display bool            // görüntü katmanında (displayLayers)
}

// fixedNameFlows kurulmuş adın iletiye ulaştığı yerleri bulur. Bir parametre "iletiye gider"
// sayılır: iletinin (messages.New/Text/TextIn) değeri ise ya da iletiye giden bir parametreye
// verilirse (sabit noktaya kadar; paketler arası, metotlar adıyla). Kaynak, kurulmuş adın kendisi
// ya da ondan atanan yerel değişkendir; + ile birleştirme de taşır.
func fixedNameFlows(t *testing.T, root string) []call {
	t.Helper()
	fset := token.NewFileSet()
	var funcs []*goFunc
	err := reporoot.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := e.Name()
		if e.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || name == "scratch" || name == "testdata" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		rel = filepath.ToSlash(rel)
		if rel == "internal/lang/messages" || rel == "internal/lang/codes" || strings.HasPrefix(rel, "internal/compute/") {
			return nil // adların ve iletilerin kendi paketleri; hesap katmanı
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		gf := &goFile{pkg: f.Name.Name, imports: map[string]bool{}}
		for _, l := range displayLayers {
			gf.display = gf.display || strings.HasPrefix(rel+"/", l)
		}
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			n := p[strings.LastIndex(p, "/")+1:]
			if im.Name != nil {
				n = im.Name.Name
			}
			gf.imports[n] = true
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			g := &goFunc{decl: fd, file: gf, key: gf.pkg + "." + fd.Name.Name}
			if fd.Recv != nil {
				g.key = "." + fd.Name.Name
			}
			for _, p := range fd.Type.Params.List {
				if len(p.Names) == 0 {
					g.params = append(g.params, "_")
				}
				for _, n := range p.Names {
					g.params = append(g.params, n.Name)
				}
				_, g.variadic = p.Type.(*ast.Ellipsis)
			}
			funcs = append(funcs, g)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defs := definitions(t, root)
	// variadicAt[key]: fonksiyonun değişken sayılı son parametresinin sırası; o parametre
	// iletiye gidiyorsa çağrıdaki ondan sonraki her değer de gider.
	variadicAt := map[string]int{}
	for _, g := range funcs {
		if g.variadic {
			variadicAt[g.key] = len(g.params) - 1
		}
	}
	// bound[key][i]: fonksiyonun i. parametresi iletiye gider.
	bound := map[string]map[int]bool{}
	for changed := true; changed; {
		changed = false
		for _, g := range funcs {
			index := map[string]int{}
			for i, p := range g.params {
				index[p] = i
			}
			eachSink(g, bound, variadicAt, defs, func(_ string, arg ast.Expr) {
				for _, id := range carried(arg) {
					if i, ok := index[id]; ok && !bound[g.key][i] {
						if bound[g.key] == nil {
							bound[g.key] = map[int]bool{}
						}
						bound[g.key][i], changed = true, true
					}
				}
			})
		}
	}
	var out []call
	seen := map[string]bool{}
	for _, g := range funcs {
		tainted := map[string]string{} // yerel değişken → onu kuran çağrı
		refs := map[string]bool{}      // codes.Ref taşıyan yerel değişkenler
		ast.Inspect(g.decl.Body, func(n ast.Node) bool {
			a, ok := n.(*ast.AssignStmt)
			if !ok || len(a.Lhs) != len(a.Rhs) {
				return true
			}
			for i, l := range a.Lhs {
				if id, ok := l.(*ast.Ident); ok {
					if name := fixedName(a.Rhs[i], refs); name != "" {
						tainted[id.Name] = name
					}
					if isRef(a.Rhs[i], refs) {
						refs[id.Name] = true
					}
				}
			}
			return true
		})
		eachSink(g, bound, variadicAt, defs, func(to string, arg ast.Expr) {
			if probes[[2]string{g.key, to}] != "" {
				return
			}
			name := fixedName(arg, refs)
			if name == "" {
				for _, id := range carried(arg) {
					if name = tainted[id]; name != "" {
						break
					}
				}
			}
			if strings.HasPrefix(name, "messages.Text") && g.file.display {
				return
			}
			if pos := fset.Position(arg.Pos()).String(); name != "" && !seen[pos] {
				seen[pos] = true
				out = append(out, call{code: name, pos: pos})
			}
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].pos < out[j].pos })
	return out
}

// eachSink g'nin gövdesinde iletiye giden her değeri, gittiği çağrının anahtarıyla verir:
// messages.New/Text/TextIn'in değerleri (örnek kod parametreleri dışında) ve iletiye giden bir
// parametreye verilen argümanlar.
func eachSink(g *goFunc, bound map[string]map[int]bool, variadicAt map[string]int, defs map[string][]string, sink func(to string, arg ast.Expr)) {
	ast.Inspect(g.decl.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		pkg, fn := callee(c)
		if pkg == "messages" && (fn == "New" || fn == "Text" || fn == "TextIn") {
			first := 1
			if fn == "TextIn" {
				first = 2
			}
			var params []string
			if len(c.Args) >= first {
				params = defs[codeArg(c.Args[first-1])]
			}
			for i, a := range c.Args {
				if j := i - first; j >= 0 && j < len(params) && exampleParams[params[j]] {
					continue
				}
				sink("messages."+fn, a)
			}
			return true
		}
		key := ""
		switch {
		case pkg == "" && fn != "":
			key = g.file.pkg + "." + fn
		case g.file.imports[pkg]:
			key = pkg + "." + fn
		default:
			if s, ok := c.Fun.(*ast.SelectorExpr); ok {
				key = "." + s.Sel.Name
			}
		}
		for i := range bound[key] {
			if i >= len(c.Args) {
				continue
			}
			last := i
			if v, ok := variadicAt[key]; ok && v == i {
				last = len(c.Args) - 1 // sarmalayıcının değerleri
			}
			// Kodu argüman alan sarmalayıcıda (syntaxError(tok, H0143, ...)) örnek kod
			// parametreleri katalogdaki adlarıyla atlanır.
			var params []string
			first := 0
			for j, a := range c.Args[:i] {
				if code := codeArg(a); code != "" {
					params, first = defs[code], j+1
				}
			}
			for j := i; j <= last; j++ {
				if k := j - first; params != nil && k >= 0 && k < len(params) && exampleParams[params[k]] {
					continue
				}
				sink(key, c.Args[j])
			}
		}
		return true
	})
}

// definitions ileti kodlarının parametre adlarıdır (definitions.json).
func definitions(t *testing.T, root string) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "internal", "lang", "messages", "definitions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var defs map[string]struct {
		Param []string `json:"param"`
	}
	if err := json.Unmarshal(data, &defs); err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for k, d := range defs {
		out[k] = d.Param
	}
	return out
}

// carried değerin taşıdığı yalın adlardır: ad, parantez içi ya da + ile birleşen parçalar.
func carried(e ast.Expr) []string {
	switch x := e.(type) {
	case *ast.Ident:
		return []string{x.Name}
	case *ast.ParenExpr:
		return carried(x.X)
	case *ast.BinaryExpr:
		return append(carried(x.X), carried(x.Y)...)
	}
	return nil
}

// fixedName değer (ya da onun + ile birleşen parçası) yazıldığı anda bir dilde kurulmuş bir
// metinse onu üreten çağrıdır: codes.Name, codes.DisplayName, bind.Named'in
// .TR'si, bir Ref'in String()'i (refs: Ref taşıyan yerel değişkenler), hazır ileti metni
// (messages.Text) ya da bunlardan birini içeren fmt.Sprint/Sprintf.
func fixedName(e ast.Expr, refs map[string]bool) string {
	switch x := e.(type) {
	case *ast.BinaryExpr:
		if n := fixedName(x.X, refs); n != "" {
			return n
		}
		return fixedName(x.Y, refs)
	case *ast.ParenExpr:
		return fixedName(x.X, refs)
	case *ast.SelectorExpr:
		if x.Sel.Name == "TR" {
			return ".TR"
		}
	case *ast.CallExpr:
		pkg, fn := callee(x)
		switch {
		case pkg == "codes" && (fn == "Name" || fn == "DisplayName"):
			return pkg + "." + fn
		case pkg == "messages" && fn == "Text":
			return "messages.Text"
		case pkg == "fmt" && strings.HasPrefix(fn, "Sprint"):
			for _, a := range x.Args {
				if n := fixedName(a, refs); n != "" {
					return "fmt." + fn + "(" + n + ")"
				}
			}
		}
		if s, ok := x.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "String" && len(x.Args) == 0 && isRef(s.X, refs) {
			return "Ref.String"
		}
	}
	return ""
}

// isRef değer bir codes.Ref mi: Ref kuran bir çağrı (codes.NameRef ..., interp.CallName,
// interp.CallPrefix), onun Called()'ı ya da Ref taşıyan bir yerel değişken.
func isRef(e ast.Expr, refs map[string]bool) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return refs[x.Name]
	case *ast.CallExpr:
		pkg, fn := callee(x)
		switch {
		case pkg == "codes" && strings.HasSuffix(fn, "Ref"):
			return true
		case pkg == "interp" && (fn == "CallName" || fn == "CallPrefix"):
			return true
		}
		if s, ok := x.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Called" {
			return isRef(s.X, refs)
		}
	}
	return false
}

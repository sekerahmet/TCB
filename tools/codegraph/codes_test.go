package main

import (
	"go/ast"
	"go/types"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// commonNameFiles bir kodun adı yalnız definitions.json'daki ortak addır (madde 15 adım 2): dil dosyalarında satırı var
// (açıklama, örnek) ama adı yok; ortam değişkeninin ise dil dosyalarında satırı da yok.
var commonNameFiles = map[string]string{
	"p/p.go": "package p\n",
	"internal/lang/codes/definitions.json": `{
  "TR_fn_sin": { "kind": "function", "common_name": "sin" },
  "TR_fn_print": { "kind": "function" },
  "TR_env_CPU_AVX2": { "kind": "command", "common_name": "TR_CPU_AVX2" }
}
`,
	"internal/lang/codes/tr.json": `{
  "TR_fn_sin": { "family": "matematik" },
  "TR_fn_print": { "name": "yazdır", "family": "çıktı" }
}
`,
	"internal/lang/codes/en.json": `{
  "TR_fn_sin": { "family": "math" },
  "TR_fn_print": { "name": "print", "family": "output" }
}
`,
}

// Kural: dil dosyasında adı yazılmayan kodun adı definitions.json'daki ortak addır (tek kaynak): ad kodu düğümünün adı
// ve açıklaması ortak ad, `query <ortak ad>` kimliği bulur, ortak adlı ortam değişkeni koduna bağlanır.
func TestACommonNameComesFromTheDefinitions(t *testing.T) {
	root := auditModule(t, commonNameFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	sin := ix.byID["name:TR_sin"]
	if sin == nil || !slices.Equal(sin.Aliases, []string{"sin"}) || !strings.HasPrefix(sin.Detail, "sin (") {
		t.Fatalf("TR_sin: %+v", sin)
	}
	if found := ix.lookup("sin"); len(found) != 1 || found[0] != sin {
		t.Errorf("query sin: %v", found)
	}
	if p := ix.byID["name:TR_print"]; p == nil || !slices.Equal(p.Aliases, []string{"yazdır", "print"}) {
		t.Errorf("a name in the language files stays: %+v", p)
	}
	if got := envCodes(root)["TR_CPU_AVX2"]; got != "TR_CPU_AVX2" {
		t.Errorf("the common name of an environment variable: %q", got)
	}
}

// Kural: codegraph'ın her ad kodu düğümünün adı codes paketinin adıdır (Türkçe ve İngilizce), ortam değişkeni kodlarının
// her dildeki adı koduna bağlanır. Örnekle değil, bütün kodlarla: adı başka bir kaynaktan (ortak ad) gelen bir kod
// okuyucu tarafından kaçırılırsa düşer.
func TestEveryNameCodeNodeHasTheNameOfTheCodesPackage(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	env := envCodes(root)
	for _, k := range codes.Codes() {
		id := k.String()
		n := ix.byID["name:"+id]
		if n == nil {
			t.Errorf("%s has no node", id)
			continue
		}
		want := []string{codes.Source().Name(k)}
		if e, ok := en.Entry(k); ok && e.Name != want[0] {
			want = append(want, e.Name)
		}
		if !slices.Equal(n.Aliases, want) {
			t.Errorf("%s: node names %v, the codes package %v", id, n.Aliases, want)
		}
		if k.Namespace() == codes.NamespaceEnvironment {
			for _, name := range want {
				if env[name] != id {
					t.Errorf("environment variable %s is not linked to %s (%q)", name, id, env[name])
				}
			}
		}
	}
}

// Kural (parti K2, C): bir ad kodu codes paketinin bir işlevine argüman olarak verilmişse (`codes.Spellings(codes.FnPrint)`)
// o satır kodun adını okur, yerleşiği uygulamaz: kenar `reads_name`dir, "uygulayıcının testi" kuralına girmez. Önceden
// kabuğun startsWithPrint'i yazdır'ın yazımlarını okuduğu için yazdır kullanan altı TR testi cmd/tr/main.go'nun testi
// görünüyordu. Okumalar codegraph'tan bağımsız, modülün tipleriyle bulunur; aynı satırda kodun başka bir kullanımı
// yoksa o satırda uses_name kenarı olmamalı, reads_name olmalı.
func TestANameCodeGivenToTheCodesPackageIsReadNotImplemented(t *testing.T) {
	root, m, g := repoGraph(t)
	keys := codeConstKeys(root)
	type site struct {
		file string
		line int
		code string
	}
	reads, other := map[site]bool{}, map[site]bool{}
	codesPkg := m.Path + "/" + codesDir
	for _, u := range m.Units {
		for _, f := range u.Files {
			inCall := map[*ast.Ident]bool{}
			ast.Inspect(f, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					if fn, ok := u.Info.Uses[nameIdent(c.Fun)].(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == codesPkg {
						for _, a := range c.Args {
							if sel, ok := a.(*ast.SelectorExpr); ok {
								inCall[sel.Sel] = true
							}
						}
					}
				}
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				k, isConst := u.Info.Uses[id].(*types.Const)
				if !isConst || k.Pkg() == nil || k.Pkg().Path() != codesPkg || keys[k.Name()] == "" || u.PkgPath == codesPkg {
					return true
				}
				pos := m.Fset.Position(id.Pos())
				s := site{m.Rel(pos.Filename), pos.Line, "name:" + keys[k.Name()]}
				if inCall[id] {
					reads[s] = true
				} else {
					other[s] = true
				}
				return true
			})
		}
	}
	kinds := map[site]map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == edgeUsesName || e.Kind == edgeReadsName {
			s := site{e.File, e.Line, e.To}
			if kinds[s] == nil {
				kinds[s] = map[string]bool{}
			}
			kinds[s][e.Kind] = true
		}
	}
	checked := 0
	for s := range reads {
		if other[s] || kinds[s] == nil {
			continue // aynı satırda başka kullanım ya da grafiğin görmediği yapılandırma
		}
		checked++
		if kinds[s][edgeUsesName] || !kinds[s][edgeReadsName] {
			t.Errorf("%s:%d gives %s to the codes package: want reads_name, the graph has %v", s.file, s.line, s.code, kinds[s])
		}
	}
	if checked < 20 {
		t.Errorf("only %d name reads checked", checked)
	}
}
